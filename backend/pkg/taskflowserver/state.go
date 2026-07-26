package taskflowserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
	"github.com/redis/go-redis/v9"
)

const taskflowStatePrefix = "devloom:taskflow:"

type persistentState struct {
	VMs    map[string]*taskflow.VirtualMachine             `json:"vms"`
	TaskVM map[string]string                               `json:"task_vm"`
	Ports  map[string]map[string]*taskflow.PortForwardInfo `json:"ports,omitempty"`
}

func (s *Server) loadFileState() error {
	if s.cfg.StateFile == "" {
		return nil
	}
	payload, err := os.ReadFile(s.cfg.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state persistentState
	if err := json.Unmarshal(payload, &state); err != nil {
		return err
	}
	s.mu.Lock()
	for id, vm := range state.VMs {
		if id != "" && vm != nil && vm.ID == id {
			s.vms[id] = vm
		}
	}
	for taskID, vmID := range state.TaskVM {
		if taskID != "" && vmID != "" {
			s.taskVM[taskID] = vmID
		}
	}
	for vmID, ports := range state.Ports {
		if vmID != "" && ports != nil {
			s.ports[vmID] = ports
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *Server) persistFileState() {
	if s.cfg.StateFile == "" {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.mu.RLock()
	state := persistentState{VMs: s.vms, TaskVM: s.taskVM, Ports: s.ports}
	payload, err := json.Marshal(&state)
	s.mu.RUnlock()
	if err != nil {
		s.logger.Warn("encode taskflow state", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.StateFile), 0o750); err != nil {
		s.logger.Warn("create taskflow state directory", "error", err)
		return
	}
	temporary := s.cfg.StateFile + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		s.logger.Warn("write taskflow state", "error", err)
		return
	}
	if err := os.Rename(temporary, s.cfg.StateFile); err != nil {
		s.logger.Warn("replace taskflow state", "error", err)
	}
}

func (s *Server) rememberVM(vm *taskflow.VirtualMachine) {
	if vm == nil || vm.ID == "" {
		return
	}
	clone := *vm
	s.mu.Lock()
	s.vms[vm.ID] = &clone
	s.mu.Unlock()
	s.persistFileState()
	if s.redis == nil {
		return
	}
	payload, err := json.Marshal(&clone)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Set(ctx, taskflowStatePrefix+"vm:"+vm.ID, payload, 0).Err(); err != nil {
		s.logger.Warn("persist VM route", "vm_id", vm.ID, "error", err)
	}
}

func (s *Server) knownVM(vmID string) *taskflow.VirtualMachine {
	s.mu.RLock()
	vm := s.vms[vmID]
	s.mu.RUnlock()
	if vm != nil {
		clone := *vm
		return &clone
	}
	if s.redis == nil || vmID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	payload, err := s.redis.Get(ctx, taskflowStatePrefix+"vm:"+vmID).Bytes()
	if err != nil {
		if err != redis.Nil {
			s.logger.Warn("restore VM route", "vm_id", vmID, "error", err)
		}
		return nil
	}
	var restored taskflow.VirtualMachine
	if err := json.Unmarshal(payload, &restored); err != nil || restored.ID != vmID {
		s.logger.Warn("decode persisted VM route", "vm_id", vmID, "error", err)
		return nil
	}
	s.mu.Lock()
	s.vms[vmID] = &restored
	s.mu.Unlock()
	s.persistFileState()
	clone := restored
	return &clone
}

func (s *Server) forgetVM(vmID string) {
	s.mu.Lock()
	delete(s.vms, vmID)
	delete(s.ports, vmID)
	s.mu.Unlock()
	s.persistFileState()
	if s.redis == nil || vmID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Del(ctx, taskflowStatePrefix+"vm:"+vmID).Err(); err != nil {
		s.logger.Warn("delete VM route", "vm_id", vmID, "error", err)
	}
}

func (s *Server) rememberTaskVM(taskID, vmID string) {
	if taskID == "" || vmID == "" {
		return
	}
	s.mu.Lock()
	s.taskVM[taskID] = vmID
	s.mu.Unlock()
	s.persistFileState()
	if s.redis == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Set(ctx, taskflowStatePrefix+"task:"+taskID, vmID, 0).Err(); err != nil {
		s.logger.Warn("persist task route", "task_id", taskID, "vm_id", vmID, "error", err)
	}
}

func (s *Server) taskVMID(taskID string) string {
	s.mu.RLock()
	vmID := s.taskVM[taskID]
	s.mu.RUnlock()
	if vmID != "" || s.redis == nil || taskID == "" {
		return vmID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	vmID, err := s.redis.Get(ctx, taskflowStatePrefix+"task:"+taskID).Result()
	if err != nil {
		if err != redis.Nil {
			s.logger.Warn("restore task route", "task_id", taskID, "error", err)
		}
		return ""
	}
	s.mu.Lock()
	s.taskVM[taskID] = vmID
	s.mu.Unlock()
	s.persistFileState()
	return vmID
}
