package taskflowserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		Listen: ":0", HostID: "local-test", HostName: "test",
		DockerBinary: "docker", ContainerPrefix: "devloom-test-vm",
		WorkspaceRoot: root, WorkspaceHostRoot: root,
		PreviewBaseURL: "http://preview.test:9080", PublicIP: "127.0.0.1",
		DefaultImage: "devloom-test", MaxAgentSteps: 5,
	}
	server, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return server
}

func TestWorkspacePathRejectsEscape(t *testing.T) {
	server := testServer(t)
	if _, err := server.workspacePath("vm-1", "../../outside"); err == nil {
		t.Fatal("workspacePath accepted traversal")
	}
	path, err := server.workspacePath("vm-1", "/workspace/src/main.go")
	if err != nil {
		t.Fatalf("workspacePath() error = %v", err)
	}
	want := filepath.Join(server.cfg.WorkspaceRoot, "vm-1", "src", "main.go")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestFileSaveListAndRead(t *testing.T) {
	server := testServer(t)
	if err := os.MkdirAll(filepath.Join(server.cfg.WorkspaceRoot, "vm-1"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := `{"operate":"save","id":"vm-1","path":"src/main.txt","content":"hello"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/files", strings.NewReader(body))
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	content, err := os.ReadFile(filepath.Join(server.cfg.WorkspaceRoot, "vm-1", "src", "main.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q", content)
	}

	listBody := `{"operate":"list","id":"vm-1","path":"src"}`
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/internal/files", strings.NewReader(listBody))
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "main.txt") {
		t.Fatalf("list status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAgentToolLoopWritesWorkspaceAndEmitsCompletion(t *testing.T) {
	server := testServer(t)
	vmID := "vm-agent"
	if err := os.MkdirAll(filepath.Join(server.cfg.WorkspaceRoot, vmID), 0o750); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"write_file","arguments":"{\"path\":\"demo.txt\",\"content\":\"generated\"}"}}]}}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"Done"}}]}`)
	}))
	defer model.Close()
	taskID := uuid.New()
	req := taskflow.CreateTaskReq{ID: taskID, VMID: vmID, Text: "create demo", LLM: taskflow.LLM{BaseURL: model.URL + "/v1", Model: "test"}}
	if err := server.runAgent(context.Background(), req); err != nil {
		t.Fatalf("runAgent() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(server.cfg.WorkspaceRoot, vmID, "demo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "generated" {
		t.Fatalf("content = %q", content)
	}
	server.mu.RLock()
	events := append([]*taskflow.TaskChunk(nil), server.events[taskID.String()]...)
	server.mu.RUnlock()
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	if events[len(events)-1].Event != "task-running" || !strings.Contains(string(events[len(events)-1].Data), "Done") {
		t.Fatalf("final event = %#v", events[len(events)-1])
	}
}

func TestPublishPortReturnsRelayURL(t *testing.T) {
	server := testServer(t)
	call := chatToolCall{ID: "call-port", Type: "function"}
	call.Function.Name = "publish_port"
	call.Function.Arguments = `{"port":4173}`
	result, err := server.executeTool(context.Background(), "vm-1", call)
	if err != nil {
		t.Fatalf("executeTool() error = %v", err)
	}
	if result != "preview URL: http://preview.test:9080/p/vm-1/4173/" {
		t.Fatalf("result = %q", result)
	}
}

func TestNotifyVMReadyUsesBackendContract(t *testing.T) {
	var received taskflow.VirtualMachine
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/vm-ready" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		respond(w, http.StatusOK, nil)
	}))
	defer backend.Close()

	server := testServer(t)
	server.cfg.BackendInternalURL = backend.URL
	vm := taskflow.VirtualMachine{ID: "vm-ready", EnvironmentID: "env-ready", HostID: "host-ready", Status: taskflow.VirtualMachineStatusOnline}
	if err := server.notifyVMReady(context.Background(), vm); err != nil {
		t.Fatalf("notifyVMReady() error = %v", err)
	}
	if received.ID != vm.ID || received.EnvironmentID != vm.EnvironmentID || received.HostID != vm.HostID {
		t.Fatalf("received VM = %#v, want %#v", received, vm)
	}
}

func TestNotifyTaskStatusUsesCallbackContract(t *testing.T) {
	taskID := uuid.New()
	var received taskflow.TaskStatusCallbackReq
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer runner-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		respond(w, http.StatusOK, nil)
	}))
	defer callback.Close()

	server := testServer(t)
	update := taskflow.TaskStatusCallbackReq{ID: taskID, Status: "completed"}
	if err := server.notifyTaskStatus(context.Background(), callback.URL, "runner-secret", update); err != nil {
		t.Fatalf("notifyTaskStatus() error = %v", err)
	}
	if received != update {
		t.Fatalf("received callback = %#v, want %#v", received, update)
	}
}

func TestHibernateRequestAcceptsCompleteBackendContract(t *testing.T) {
	payload := `{"host_id":"host-1","user_id":"user-1","id":"vm-1","environment_id":"env-1"}`
	request := httptest.NewRequest(http.MethodPost, "/internal/vm/hibernate", strings.NewReader(payload))
	var req taskflow.HibernateVirtualMachineReq
	if err := decode(request, &req); err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if req.HostID != "host-1" || req.UserID != "user-1" || req.ID != "vm-1" || req.EnvironmentID != "env-1" {
		t.Fatalf("decoded request = %#v", req)
	}
}

func TestRemoteRunnerRegistrationAndVMForward(t *testing.T) {
	server := testServer(t)
	server.cfg.RunnerSecret = "runner-secret"
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req taskflow.CheckTokenReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Token != "install-token" || req.MachineID != "machine-1" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		respond(w, http.StatusOK, taskflow.Token{Kind: taskflow.OrchestratorToken, Token: "remote-1", User: &taskflow.TokenUser{ID: uuid.NewString()}})
	}))
	defer backend.Close()
	server.cfg.BackendInternalURL = backend.URL
	expectedCredential := server.runnerCredential("remote-1", "machine-1")
	var createCalls atomic.Int32
	runner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+expectedCredential {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/internal/vm" {
			http.NotFound(w, r)
			return
		}
		createCalls.Add(1)
		var req taskflow.CreateVirtualMachineReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		respond(w, http.StatusOK, &taskflow.VirtualMachine{ID: req.ID, EnvironmentID: req.ID, HostID: "remote-1", Status: taskflow.VirtualMachineStatusOnline})
	}))
	defer runner.Close()

	registration := map[string]any{"host": map[string]any{"id": "machine-1", "hostname": "worker", "arch": "x86_64", "os": "linux", "name": "worker"}, "endpoint": runner.URL, "machine_id": "machine-1"}
	payload, _ := json.Marshal(registration)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/runner/register", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer install-token")
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"host_id":"remote-1"`) || !strings.Contains(recorder.Body.String(), `"credential":"`+expectedCredential+`"`) {
		t.Fatalf("register body=%s", recorder.Body.String())
	}

	vmRequest := taskflow.CreateVirtualMachineReq{ID: "remote-vm", HostID: "remote-1", ImageURL: "devbox:test"}
	payload, _ = json.Marshal(vmRequest)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/internal/vm", bytes.NewReader(payload))
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if createCalls.Load() != 1 {
		t.Fatalf("runner create calls=%d, want 1", createCalls.Load())
	}
	if !strings.Contains(recorder.Body.String(), `"host_id":"remote-1"`) {
		t.Fatalf("create body=%s", recorder.Body.String())
	}
}

func TestRemoteRoutesSurviveCenterRestart(t *testing.T) {
	store := miniredis.RunT(t)
	newClient := func() *redis.Client { return redis.NewClient(&redis.Options{Addr: store.Addr()}) }

	before := testServer(t)
	before.redis = newClient()
	before.rememberVM(&taskflow.VirtualMachine{ID: "persisted-vm", HostID: "remote-1"})
	before.rememberTaskVM("persisted-task", "persisted-vm")

	after := testServer(t)
	after.redis = newClient()
	after.runners["remote-1"] = &runnerRegistration{
		Host: taskflow.Host{ID: "remote-1"}, Endpoint: "http://runner.test", Credential: "session",
		UpdatedAt: time.Now(),
	}
	registration, remote := after.remoteVM("persisted-vm")
	if !remote || registration == nil || registration.Host.ID != "remote-1" {
		t.Fatalf("remoteVM() = %#v, %v", registration, remote)
	}
	registration, remote = after.remoteTask("persisted-task")
	if !remote || registration == nil || registration.Host.ID != "remote-1" {
		t.Fatalf("remoteTask() = %#v, %v", registration, remote)
	}
}

func TestRunnerRoutesSurviveLocalRestart(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "runner-state.json")
	before := testServer(t)
	before.cfg.StateFile = stateFile
	before.rememberVM(&taskflow.VirtualMachine{ID: "runner-vm", HostID: "runner-host"})
	before.rememberTaskVM("runner-task", "runner-vm")

	after := testServer(t)
	after.cfg.StateFile = stateFile
	if err := after.loadFileState(); err != nil {
		t.Fatalf("loadFileState() error = %v", err)
	}
	vm := after.knownVM("runner-vm")
	if vm == nil || vm.HostID != "runner-host" {
		t.Fatalf("knownVM() = %#v", vm)
	}
	if vmID := after.taskVMID("runner-task"); vmID != "runner-vm" {
		t.Fatalf("taskVMID() = %q", vmID)
	}
}
