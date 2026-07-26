package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflowserver"
)

type registration struct {
	Host      taskflow.Host `json:"host"`
	Endpoint  string        `json:"endpoint"`
	MachineID string        `json:"machine_id"`
}

type registrationResponse struct {
	Registered bool   `json:"registered"`
	HostID     string `json:"host_id"`
	Credential string `json:"credential"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	center := strings.TrimRight(required("DEVLOOM_CENTER_RUNNER_URL"), "/")
	installToken := required("DEVLOOM_RUNNER_REGISTRATION_TOKEN")
	machineID := required("DEVLOOM_RUNNER_MACHINE_ID")
	listen := value("DEVLOOM_RUNNER_LISTEN", ":8890")
	advertise := strings.TrimRight(required("DEVLOOM_RUNNER_ADVERTISE_URL"), "/")
	hostname, _ := os.Hostname()
	registered, err := registerRunner(context.Background(), center, installToken, advertise, machineID, machineID, value("DEVLOOM_RUNNER_HOST_NAME", hostname), value("DEVLOOM_PUBLIC_IP", ""))
	if err != nil {
		logger.Error("register runner", "error", err)
		os.Exit(1)
	}
	cfg := taskflowserver.Config{
		Listen: listen, HostID: registered.HostID, HostName: value("DEVLOOM_RUNNER_HOST_NAME", hostname),
		DockerBinary: "docker", ContainerPrefix: value("DEVLOOM_VM_CONTAINER_PREFIX", "devloom-vm"),
		WorkspaceRoot:     value("DEVLOOM_WORKSPACE_ROOT", "/workspaces"),
		WorkspaceHostRoot: required("DEVLOOM_WORKSPACE_HOST_ROOT"),
		StateFile:         value("DEVLOOM_TASKFLOW_STATE_FILE", "/workspaces/.devloom-runner-state.json"),
		PreviewBaseURL:    strings.TrimRight(value("DEVLOOM_PREVIEW_BASE_URL", advertise), "/"),
		PublicIP:          value("DEVLOOM_PUBLIC_IP", strings.TrimPrefix(strings.TrimPrefix(advertise, "http://"), "https://")),
		DefaultImage:      required("DEVLOOM_DEFAULT_DEVBOX_IMAGE"), MaxAgentSteps: 30,
		RunnerSecret: registered.Credential, RunnerMode: true,
		TaskStatusCallbackURL: center + "/task-status", TaskStatusCallbackToken: registered.Credential,
	}
	server, err := taskflowserver.New(cfg, logger)
	if err != nil {
		logger.Error("initialize runner", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{Addr: listen, Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second}
	go registerLoop(context.Background(), center, installToken, advertise, registered.HostID, machineID, cfg.HostName, cfg.PublicIP, logger)
	logger.Info("orchestrator listening", "address", listen, "advertise", advertise, "host_id", registered.HostID)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("orchestrator stopped", "error", err)
		os.Exit(1)
	}
}

func registerLoop(ctx context.Context, center, installToken, endpoint, hostID, machineID, hostname, publicIP string, logger *slog.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			registered, err := registerRunner(ctx, center, installToken, endpoint, hostID, machineID, hostname, publicIP)
			if err != nil {
				logger.Warn("register runner", "error", err)
				continue
			}
			if registered.HostID != hostID {
				logger.Error("runner registration changed host ID", "expected", hostID, "actual", registered.HostID)
			}
		}
	}
}

func registerRunner(ctx context.Context, center, installToken, endpoint, hostID, machineID, hostname, publicIP string) (*registrationResponse, error) {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	payload := registration{
		Host:     taskflow.Host{ID: hostID, MachineID: machineID, Hostname: hostname, Name: "DevLoom remote Docker", Arch: arch, OS: "linux", Cores: int32(runtime.NumCPU()), PublicIP: publicIP, InternalIP: publicIP, TTL: taskflow.TTL{Kind: taskflow.TTLForever}, CreatedAt: time.Now().Unix(), Version: "source"},
		Endpoint: endpoint, MachineID: machineID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, center+"/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+installToken)
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("registration returned HTTP %d", response.StatusCode)
	}
	var envelope taskflow.Resp[registrationResponse]
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode registration: %w", err)
	}
	if envelope.Code != 0 || !envelope.Data.Registered || envelope.Data.HostID == "" || envelope.Data.Credential == "" {
		return nil, fmt.Errorf("registration rejected: %s", envelope.Message)
	}
	return &envelope.Data, nil
}

func required(name string) string {
	result := strings.TrimSpace(os.Getenv(name))
	if result == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", name)
		os.Exit(2)
	}
	return result
}
func value(name, fallback string) string {
	if result := strings.TrimSpace(os.Getenv(name)); result != "" {
		return result
	}
	return fallback
}
