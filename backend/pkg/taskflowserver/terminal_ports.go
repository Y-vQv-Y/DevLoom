package taskflowserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func (s *Server) terminal(w http.ResponseWriter, r *http.Request) {
	if registration, remote := s.remoteVM(r.URL.Query().Get("id")); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "terminal closed")
	vmID := r.URL.Query().Get("id")
	command := r.URL.Query().Get("exec")
	if command == "" {
		command = "exec sh"
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd, err := s.docker.Process(ctx, vmID, command)
	if err != nil {
		_ = writeTerminalJSON(ctx, conn, taskflow.TerminalData{Error: stringPtr(err.Error())})
		return
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = writeTerminalJSON(ctx, conn, taskflow.TerminalData{Error: stringPtr(err.Error())})
		return
	}
	outputReader, outputWriter := io.Pipe()
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	if err := cmd.Start(); err != nil {
		_ = writeTerminalJSON(ctx, conn, taskflow.TerminalData{Error: stringPtr(err.Error())})
		return
	}
	_ = writeTerminalJSON(ctx, conn, taskflow.TerminalData{Connected: true})

	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, 32<<10)
		for {
			count, readErr := outputReader.Read(buffer)
			if count > 0 {
				if err := conn.Write(ctx, websocket.MessageBinary, buffer[:count]); err != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer stdin.Close()
		for {
			kind, data, readErr := conn.Read(ctx)
			if readErr != nil {
				return
			}
			if kind == websocket.MessageBinary {
				if _, err := stdin.Write(data); err != nil {
					return
				}
			}
		}
	}()

	waitErr := cmd.Wait()
	_ = outputWriter.Close()
	<-done
	if waitErr != nil && ctx.Err() == nil {
		_ = writeTerminalJSON(context.Background(), conn, taskflow.TerminalData{Error: stringPtr(waitErr.Error())})
	}
}

func writeTerminalJSON(ctx context.Context, conn *websocket.Conn, data taskflow.TerminalData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, payload)
}

func stringPtr(value string) *string { return &value }

func (s *Server) listTerminals(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, []*taskflow.Terminal{})
}

func (s *Server) closeTerminal(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, nil)
}

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	if registration, remote := s.remoteVM(r.URL.Query().Get("id")); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "reports closed")
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case now := <-ticker.C:
			entry := taskflow.ReportEntry{ID: uuid.NewString(), Source: r.URL.Query().Get("id"), Ts: now.UnixMilli()}
			payload, _ := json.Marshal(entry)
			if err := conn.Write(r.Context(), websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func (s *Server) listPorts(w http.ResponseWriter, r *http.Request) {
	vmID := r.URL.Query().Get("id")
	if registration, remote := s.remoteVM(vmID); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	s.mu.RLock()
	records := s.ports[vmID]
	result := make([]*taskflow.PortForwardInfo, 0, len(records))
	for _, info := range records {
		clone := *info
		result = append(result, &clone)
	}
	s.mu.RUnlock()
	respond(w, http.StatusOK, &taskflow.ListPortforwadResp{Ports: result})
}

func (s *Server) createPort(w http.ResponseWriter, r *http.Request) {
	var req taskflow.CreatePortForward
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		var result taskflow.PortForwardInfo
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/port-forward", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	if req.LocalPort < 1 || req.LocalPort > 65535 {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid port"))
		return
	}
	if !s.docker.IsRunning(r.Context(), req.ID) {
		fail(w, http.StatusConflict, fmt.Errorf("VM is not online"))
		return
	}
	forwardID := uuid.NewString()
	accessURL := fmt.Sprintf("%s/p/%s/%d/", s.cfg.PreviewBaseURL, req.ID, req.LocalPort)
	info := &taskflow.PortForwardInfo{Port: req.LocalPort, Status: "online", ForwardID: &forwardID, AccessURL: &accessURL, CreatedAt: time.Now().Unix(), Success: true, WhitelistIPs: req.WhitelistIPs}
	s.mu.Lock()
	if s.ports[req.ID] == nil {
		s.ports[req.ID] = make(map[string]*taskflow.PortForwardInfo)
	}
	s.ports[req.ID][forwardID] = info
	s.mu.Unlock()
	s.persistFileState()
	respond(w, http.StatusOK, info)
}

func (s *Server) updatePort(w http.ResponseWriter, r *http.Request) {
	var req taskflow.UpdatePortForward
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		var result taskflow.PortForwardInfo
		if err := s.forwardJSON(r, registration, http.MethodPut, "/internal/port-forward", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	s.mu.Lock()
	info := s.ports[req.ID][req.ForwardID]
	if info != nil {
		info.WhitelistIPs = append([]string(nil), req.WhitelistIPs...)
	}
	s.mu.Unlock()
	s.persistFileState()
	if info == nil {
		fail(w, http.StatusNotFound, fmt.Errorf("port forward not found"))
		return
	}
	respond(w, http.StatusOK, info)
}

func (s *Server) closePort(w http.ResponseWriter, r *http.Request) {
	var req taskflow.ClosePortForward
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/port-forward/close", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, nil)
		return
	}
	s.mu.Lock()
	if s.ports[req.ID] != nil {
		delete(s.ports[req.ID], req.ForwardID)
	}
	s.mu.Unlock()
	s.persistFileState()
	respond(w, http.StatusOK, nil)
}

func (s *Server) findPort(vmID string, port int) *taskflow.PortForwardInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, info := range s.ports[vmID] {
		if int(info.Port) == port {
			clone := *info
			return &clone
		}
	}
	return nil
}
