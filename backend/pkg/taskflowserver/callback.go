package taskflowserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

const (
	vmReadyRetryInterval    = 250 * time.Millisecond
	vmReadyMaxAttempts      = 40
	taskStatusRetryInterval = 500 * time.Millisecond
	taskStatusMaxAttempts   = 40
)

func (s *Server) scheduleVMReady(vm taskflow.VirtualMachine) {
	if s.cfg.BackendInternalURL == "" {
		return
	}

	go func() {
		for attempt := 1; attempt <= vmReadyMaxAttempts; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := s.notifyVMReady(ctx, vm)
			cancel()
			if err == nil {
				s.logger.Info("VM ready callback delivered", "vm_id", vm.ID, "attempt", attempt)
				return
			}
			if attempt < vmReadyMaxAttempts {
				time.Sleep(vmReadyRetryInterval)
				continue
			}
			s.logger.Error("VM ready callback exhausted retries", "vm_id", vm.ID, "error", err)
		}
	}()
}

func (s *Server) notifyVMReady(ctx context.Context, vm taskflow.VirtualMachine) error {
	payload, err := json.Marshal(vm)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BackendInternalURL+"/internal/vm-ready", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("backend HTTP %d: %s", resp.StatusCode, string(body))
	}
	var result taskflow.Resp[json.RawMessage]
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode backend response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("backend code %d: %s", result.Code, result.Message)
	}
	return nil
}

func (s *Server) scheduleTaskStatus(update taskflow.TaskStatusCallbackReq) {
	if s.cfg.TaskStatusCallbackURL == "" || update.ID == [16]byte{} {
		return
	}
	go s.deliverTaskStatus(s.cfg.TaskStatusCallbackURL, s.cfg.TaskStatusCallbackToken, update)
}

func (s *Server) scheduleBackendTaskStatus(update taskflow.TaskStatusCallbackReq) {
	if s.cfg.BackendInternalURL == "" || update.ID == [16]byte{} {
		return
	}
	go s.deliverTaskStatus(s.cfg.BackendInternalURL+"/internal/task-status", "", update)
}

func (s *Server) deliverTaskStatus(endpoint, token string, update taskflow.TaskStatusCallbackReq) {
	var lastErr error
	for attempt := 1; attempt <= taskStatusMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = s.notifyTaskStatus(ctx, endpoint, token, update)
		cancel()
		if lastErr == nil {
			s.logger.Info("task status callback delivered", "task_id", update.ID, "status", update.Status, "attempt", attempt)
			return
		}
		if attempt < taskStatusMaxAttempts {
			time.Sleep(taskStatusRetryInterval)
		}
	}
	s.logger.Error("task status callback exhausted retries", "task_id", update.ID, "status", update.Status, "error", lastErr)
}

func (s *Server) notifyTaskStatus(ctx context.Context, endpoint, token string, update taskflow.TaskStatusCallbackReq) error {
	payload, err := json.Marshal(update)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if s.cfg.RunnerMode && s.cfg.HostID != "" {
		req.Header.Set("X-DevLoom-Runner-ID", s.cfg.HostID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("callback HTTP %d: %s", resp.StatusCode, string(body))
	}
	var result taskflow.Resp[json.RawMessage]
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode callback response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("callback code %d: %s", result.Code, result.Message)
	}
	return nil
}
