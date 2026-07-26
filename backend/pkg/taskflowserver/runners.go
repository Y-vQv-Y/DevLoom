package taskflowserver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func (s *Server) runnerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.cfg.RunnerSecret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.RunnerSecret)) != 1 {
			fail(w, http.StatusUnauthorized, fmt.Errorf("invalid runner credential"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type runnerRegistration struct {
	Host       taskflow.Host `json:"host"`
	Endpoint   string        `json:"endpoint"`
	MachineID  string        `json:"machine_id"`
	Credential string        `json:"-"`
	UpdatedAt  time.Time     `json:"-"`
}

type runnerRegistrationResponse struct {
	Registered bool   `json:"registered"`
	HostID     string `json:"host_id"`
	Credential string `json:"credential"`
}

func (s *Server) registerRunner(w http.ResponseWriter, r *http.Request) {
	installToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if installToken == "" || s.cfg.RunnerSecret == "" || s.cfg.BackendInternalURL == "" {
		fail(w, http.StatusUnauthorized, fmt.Errorf("runner registration is not configured"))
		return
	}
	var registration runnerRegistration
	if err := decode(r, &registration); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if !safeIDPattern.MatchString(registration.MachineID) {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid runner machine ID"))
		return
	}
	endpoint, err := url.Parse(registration.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		fail(w, http.StatusBadRequest, fmt.Errorf("runner endpoint must be an absolute HTTP(S) URL"))
		return
	}
	registration.Endpoint = strings.TrimRight(endpoint.String(), "/")
	auth, err := s.authenticateRunnerInstall(r.Context(), installToken, registration.MachineID)
	if err != nil || auth.Kind != taskflow.OrchestratorToken || auth.Token == "" || auth.User == nil {
		fail(w, http.StatusUnauthorized, fmt.Errorf("invalid runner install token"))
		return
	}
	if !safeIDPattern.MatchString(auth.Token) || auth.Token == s.cfg.HostID {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid remote host ID"))
		return
	}
	registration.Host.ID = auth.Token
	registration.Host.MachineID = registration.MachineID
	registration.Host.UserID = auth.User.ID
	registration.Credential = s.runnerCredential(auth.Token, registration.MachineID)
	registration.UpdatedAt = time.Now()
	s.mu.Lock()
	s.runners[registration.Host.ID] = &registration
	s.mu.Unlock()
	respond(w, http.StatusOK, runnerRegistrationResponse{Registered: true, HostID: auth.Token, Credential: registration.Credential})
}

func (s *Server) authenticateRunnerInstall(ctx context.Context, installToken, machineID string) (*taskflow.Token, error) {
	payload, err := json.Marshal(taskflow.CheckTokenReq{Token: installToken, MachineID: machineID})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BackendInternalURL+"/internal/check-token", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("backend token check returned HTTP %d", response.StatusCode)
	}
	var envelope taskflow.Resp[taskflow.Token]
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("backend token check failed: %s", envelope.Message)
	}
	return &envelope.Data, nil
}

func (s *Server) runnerCredential(hostID, machineID string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.RunnerSecret))
	_, _ = mac.Write([]byte("devloom-runner-v1\x00" + hostID + "\x00" + machineID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) runnerTaskStatus(w http.ResponseWriter, r *http.Request) {
	hostID := r.Header.Get("X-DevLoom-Runner-ID")
	registration := s.runner(hostID)
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if registration == nil || subtle.ConstantTimeCompare([]byte(provided), []byte(registration.Credential)) != 1 {
		fail(w, http.StatusUnauthorized, fmt.Errorf("invalid runner credential"))
		return
	}
	var update taskflow.TaskStatusCallbackReq
	if err := decode(r, &update); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if update.ID == [16]byte{} || (update.Status != "completed" && update.Status != "failed" && update.Status != "cancelled") {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid task status callback"))
		return
	}
	s.scheduleBackendTaskStatus(update)
	respond(w, http.StatusOK, nil)
}

func (s *Server) runner(hostID string) *runnerRegistration {
	s.mu.RLock()
	registration := s.runners[hostID]
	s.mu.RUnlock()
	if registration == nil || time.Since(registration.UpdatedAt) > 45*time.Second {
		return nil
	}
	clone := *registration
	return &clone
}

func (s *Server) forwardJSON(r *http.Request, registration *runnerRegistration, method, path string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(r.Context(), method, registration.Endpoint+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if registration.Credential != "" {
		request.Header.Set("Authorization", "Bearer "+registration.Credential)
	}
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Do(request)
	if err != nil {
		return fmt.Errorf("runner %s: %w", registration.Host.ID, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("runner HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if output == nil {
		return nil
	}
	var envelope taskflow.Resp[json.RawMessage]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode runner response: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("runner: %s", envelope.Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, output)
}

func (s *Server) remoteVM(vmID string) (*runnerRegistration, bool) {
	vm := s.knownVM(vmID)
	if vm == nil || vm.HostID == "" || vm.HostID == s.cfg.HostID {
		return nil, false
	}
	return s.runner(vm.HostID), true
}

func (s *Server) remoteTask(taskID string) (*runnerRegistration, bool) {
	vmID := s.taskVMID(taskID)
	return s.remoteVM(vmID)
}

func (s *Server) proxyRunner(w http.ResponseWriter, r *http.Request, registration *runnerRegistration) {
	if registration == nil {
		fail(w, http.StatusServiceUnavailable, fmt.Errorf("remote runner is offline"))
		return
	}
	target, _ := url.Parse(registration.Endpoint)
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(request *http.Request) {
		original(request)
		request.Host = target.Host
		if registration.Credential != "" {
			request.Header.Set("Authorization", "Bearer "+registration.Credential)
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		fail(w, http.StatusBadGateway, fmt.Errorf("remote runner proxy: %w", err))
	}
	proxy.ServeHTTP(w, r)
}
