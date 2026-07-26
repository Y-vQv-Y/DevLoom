package taskflowserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

type chatMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var eventSequence atomic.Uint64

func (s *Server) appendEvent(taskID, event, kind string, data any) *taskflow.TaskChunk {
	var payload []byte
	switch value := data.(type) {
	case nil:
		payload = nil
	case []byte:
		payload = value
	case string:
		payload = []byte(value)
	default:
		payload, _ = json.Marshal(value)
	}
	chunk := &taskflow.TaskChunk{Event: event, Kind: kind, Data: payload, Seq: eventSequence.Add(1), Timestamp: time.Now().UnixNano()}
	s.mu.Lock()
	s.events[taskID] = append(s.events[taskID], chunk)
	for watcher := range s.watches[taskID] {
		select {
		case watcher <- chunk:
		default:
		}
	}
	s.mu.Unlock()
	return chunk
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req taskflow.CreateTaskReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.ID == uuid.Nil || req.VMID == "" {
		fail(w, http.StatusBadRequest, fmt.Errorf("task ID and VM ID are required"))
		return
	}
	taskID := req.ID.String()
	if registration, remote := s.remoteVM(req.VMID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		s.mu.Lock()
		s.tasks[taskID] = req
		s.mu.Unlock()
		s.rememberTaskVM(taskID, req.VMID)
		respond(w, http.StatusOK, nil)
		return
	}
	s.mu.Lock()
	if old := s.cancels[taskID]; old != nil {
		old()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancels[taskID] = cancel
	s.tasks[taskID] = req
	s.mu.Unlock()
	s.rememberTaskVM(taskID, req.VMID)
	s.appendEvent(taskID, "user-input", "user_input", map[string]any{"encoding": "plaintext", "content": req.Text, "attachments": req.Attachments})
	s.appendEvent(taskID, "task-started", "acp_event", nil)
	go s.runTask(ctx, req)
	respond(w, http.StatusOK, nil)
}

func (s *Server) runTask(ctx context.Context, req taskflow.CreateTaskReq) {
	taskID := req.ID.String()
	defer func() { s.mu.Lock(); delete(s.cancels, taskID); s.mu.Unlock() }()
	if err := s.applyTaskConfigs(req); err != nil {
		s.finishTask(taskID, err)
		return
	}
	if err := s.runAgent(ctx, req); err != nil {
		if ctx.Err() != nil {
			s.appendEvent(taskID, "task-ended", "acp_event", map[string]any{"status": "cancelled"})
			s.scheduleTaskStatus(taskflow.TaskStatusCallbackReq{ID: req.ID, Status: "cancelled"})
			return
		}
		s.finishTask(taskID, err)
		return
	}
	s.appendEvent(taskID, "task-ended", "acp_event", map[string]any{"status": "completed"})
	s.scheduleTaskStatus(taskflow.TaskStatusCallbackReq{ID: req.ID, Status: "completed"})
}

func (s *Server) finishTask(taskID string, err error) {
	s.appendEvent(taskID, "task-error", "acp_event", map[string]any{"message": err.Error(), "details": err.Error()})
	s.appendEvent(taskID, "task-ended", "acp_event", map[string]any{"status": "failed"})
	id, parseErr := uuid.Parse(taskID)
	if parseErr == nil {
		s.scheduleTaskStatus(taskflow.TaskStatusCallbackReq{ID: id, Status: "failed", Error: err.Error()})
	}
}

func (s *Server) applyTaskConfigs(req taskflow.CreateTaskReq) error {
	for _, config := range req.Configs {
		path := strings.TrimSpace(config.Path)
		if strings.HasPrefix(path, "/workspace/") || !filepath.IsAbs(path) && !strings.HasPrefix(path, "~/") {
			target, err := s.workspacePath(req.VMID, path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			mode := os.FileMode(0o640)
			if config.Mode != nil {
				mode = os.FileMode(*config.Mode)
			}
			if err := os.WriteFile(target, []byte(config.Content), mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) runAgent(ctx context.Context, req taskflow.CreateTaskReq) error {
	if strings.TrimSpace(req.LLM.BaseURL) == "" || strings.TrimSpace(req.LLM.Model) == "" {
		return fmt.Errorf("task model is not configured")
	}
	system := strings.TrimSpace(req.SystemPrompt)
	if system != "" {
		system += "\n\n"
	}
	system += "You are running inside a Linux development container. Complete the user's request by using the provided tools. Work only in /workspace. Run tests. For web applications, bind the server to 0.0.0.0 and call publish_port after it is running. Do not merely describe code: create and verify it."
	messages := []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: req.Text}}
	tools := agentTools()
	for step := 0; step < s.cfg.MaxAgentSteps; step++ {
		response, err := s.chat(ctx, req.LLM, messages, tools)
		if err != nil {
			return err
		}
		if len(response.ToolCalls) == 0 {
			text, _ := response.Content.(string)
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("model returned neither text nor tools")
			}
			s.agentMessage(req.ID.String(), text)
			return nil
		}
		messages = append(messages, response)
		for _, call := range response.ToolCalls {
			s.toolEvent(req.ID.String(), call, "in_progress", nil)
			result, toolErr := s.executeTool(ctx, req.VMID, call)
			if toolErr != nil {
				result = "ERROR: " + toolErr.Error()
			}
			s.toolEvent(req.ID.String(), call, map[bool]string{true: "failed", false: "completed"}[toolErr != nil], result)
			messages = append(messages, chatMessage{Role: "tool", ToolCallID: call.ID, Content: result})
		}
	}
	return fmt.Errorf("agent exceeded %d tool steps", s.cfg.MaxAgentSteps)
}

func (s *Server) chat(ctx context.Context, llm taskflow.LLM, messages []chatMessage, tools []map[string]any) (chatMessage, error) {
	endpoint := strings.TrimRight(llm.BaseURL, "/") + "/chat/completions"
	body := map[string]any{"model": llm.Model, "messages": messages, "tools": tools, "tool_choice": "auto", "stream": false}
	if llm.Temperature != nil {
		body["temperature"] = *llm.Temperature
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return chatMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if llm.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+llm.ApiKey)
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return chatMessage{}, fmt.Errorf("model request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return chatMessage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return chatMessage{}, fmt.Errorf("model HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded chatResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return chatMessage{}, fmt.Errorf("decode model response: %w", err)
	}
	if decoded.Error != nil {
		return chatMessage{}, fmt.Errorf("model: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("model returned no choices")
	}
	return decoded.Choices[0].Message, nil
}

func agentTools() []map[string]any {
	tool := func(name, description string, properties map[string]any, required ...string) map[string]any {
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": description,
				"parameters": map[string]any{
					"type":                 "object",
					"properties":           properties,
					"required":             required,
					"additionalProperties": false,
				},
			},
		}
	}
	stringField := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	return []map[string]any{
		tool("write_file", "Create or replace a UTF-8 text file in /workspace.", map[string]any{"path": stringField("Workspace-relative path"), "content": stringField("Complete file content")}, "path", "content"),
		tool("read_file", "Read a UTF-8 file from /workspace.", map[string]any{"path": stringField("Workspace-relative path")}, "path"),
		tool("list_files", "List files below a workspace directory.", map[string]any{"path": stringField("Workspace-relative directory, or .")}, "path"),
		tool("run_command", "Run a shell command in /workspace and return combined output.", map[string]any{"command": stringField("Shell command"), "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 600}}, "command"),
		tool("publish_port", "Publish a running HTTP service through the preview relay.", map[string]any{"port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}}, "port"),
	}
}

func (s *Server) executeTool(ctx context.Context, vmID string, call chatToolCall) (string, error) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}
	getString := func(name string) string { var value string; _ = json.Unmarshal(args[name], &value); return value }
	switch call.Function.Name {
	case "write_file":
		path, err := s.workspacePath(vmID, getString("path"))
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return "", err
		}
		content := getString("content")
		if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(content), getString("path")), nil
	case "read_file":
		path, err := s.workspacePath(vmID, getString("path"))
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if len(content) > 1<<20 {
			content = content[:1<<20]
		}
		return string(content), nil
	case "list_files":
		path := getString("path")
		command := "find " + shellQuote(path) + " -maxdepth 3 -mindepth 1 -printf '%y %p\\n' | sort | head -500"
		output, err := s.docker.Exec(ctx, vmID, command)
		return string(output), err
	case "run_command":
		timeout := 120
		_ = json.Unmarshal(args["timeout_seconds"], &timeout)
		if timeout < 1 {
			timeout = 120
		}
		if timeout > 600 {
			timeout = 600
		}
		commandCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		output, err := s.docker.Exec(commandCtx, vmID, getString("command"))
		if len(output) > 2<<20 {
			output = output[len(output)-(2<<20):]
		}
		return string(output), err
	case "publish_port":
		var port int
		if err := json.Unmarshal(args["port"], &port); err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("invalid port")
		}
		forwardID := uuid.NewString()
		accessURL := fmt.Sprintf("%s/p/%s/%d/", s.cfg.PreviewBaseURL, vmID, port)
		info := &taskflow.PortForwardInfo{Port: int32(port), Status: "online", ForwardID: &forwardID, AccessURL: &accessURL, CreatedAt: time.Now().Unix(), Success: true}
		s.mu.Lock()
		if s.ports[vmID] == nil {
			s.ports[vmID] = make(map[string]*taskflow.PortForwardInfo)
		}
		s.ports[vmID][forwardID] = info
		s.mu.Unlock()
		return "preview URL: " + accessURL, nil
	default:
		return "", fmt.Errorf("unknown tool %q", call.Function.Name)
	}
}

func (s *Server) agentMessage(taskID, text string) {
	s.appendEvent(taskID, "task-running", "acp_event", map[string]any{"update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": text}}})
}

func (s *Server) toolEvent(taskID string, call chatToolCall, status string, result any) {
	update := map[string]any{"sessionUpdate": "tool_call", "toolCallId": call.ID, "kind": toolKind(call.Function.Name), "status": status, "title": call.Function.Name, "rawInput": json.RawMessage(call.Function.Arguments)}
	if status != "in_progress" {
		update["sessionUpdate"] = "tool_call_update"
		update["rawOutput"] = map[string]any{"stdout": result}
	}
	s.appendEvent(taskID, "task-running", "acp_event", map[string]any{"update": update})
}

func toolKind(name string) string {
	switch name {
	case "write_file":
		return "edit"
	case "read_file":
		return "read"
	case "list_files":
		return "search"
	case "run_command":
		return "execute"
	default:
		return "other"
	}
}

func (s *Server) stopTask(w http.ResponseWriter, r *http.Request) {
	var req taskflow.TaskReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Task == nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("task is required"))
		return
	}
	taskID := req.Task.ID.String()
	if registration, remote := s.remoteTask(taskID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, r.URL.Path, &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, nil)
		return
	}
	s.mu.RLock()
	cancel := s.cancels[taskID]
	s.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	s.appendEvent(taskID, "user-cancel", "control", nil)
	s.appendEvent(taskID, "task-ended", "acp_event", map[string]any{"status": "cancelled"})
	s.scheduleTaskStatus(taskflow.TaskStatusCallbackReq{ID: req.Task.ID, Status: "cancelled"})
	respond(w, http.StatusOK, nil)
}

func (s *Server) continueTask(w http.ResponseWriter, r *http.Request) {
	var request taskflow.TaskReq
	if err := decode(r, &request); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if request.Task == nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("task is required"))
		return
	}
	taskID := request.Task.ID.String()
	if registration, remote := s.remoteTask(taskID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/continue", &request, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, nil)
		return
	}
	s.mu.RLock()
	previous, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("task not found"))
		return
	}
	previous.Text = request.Task.Text
	synthetic := r.Clone(r.Context())
	payload, _ := json.Marshal(previous)
	synthetic.Body = io.NopCloser(bytes.NewReader(payload))
	s.createTask(w, synthetic)
}

func (s *Server) restartTask(w http.ResponseWriter, r *http.Request) {
	var req taskflow.RestartTaskReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	taskID := req.ID.String()
	if registration, remote := s.remoteTask(taskID); remote {
		var result taskflow.RestartTaskResp
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/restart", &req, &result); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, &result)
		return
	}
	s.mu.RLock()
	previous, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("task not found"))
		return
	}
	if req.ExecutionConfig != nil {
		previous.Env = req.ExecutionConfig.Envs
		previous.Configs = req.ExecutionConfig.ConfigFiles
		previous.McpConfigs = req.ExecutionConfig.McpServers
		previous.AgentResources = req.ExecutionConfig.AgentResources
	}
	s.mu.Lock()
	if cancel := s.cancels[taskID]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancels[taskID] = cancel
	s.tasks[taskID] = previous
	s.mu.Unlock()
	s.appendEvent(taskID, "task-started", "acp_event", nil)
	go s.runTask(ctx, previous)
	respond(w, http.StatusOK, &taskflow.RestartTaskResp{ID: req.ID, RequestId: req.RequestId, Success: true, Message: "restarted", SessionID: uuid.NewString()})
}

func (s *Server) taskLive(w http.ResponseWriter, r *http.Request) {
	if registration, remote := s.remoteTask(r.URL.Query().Get("id")); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "stream ended")
	taskID := r.URL.Query().Get("id")
	watcher := make(chan *taskflow.TaskChunk, 128)
	s.mu.Lock()
	history := append([]*taskflow.TaskChunk(nil), s.events[taskID]...)
	if s.watches[taskID] == nil {
		s.watches[taskID] = make(map[chan *taskflow.TaskChunk]struct{})
	}
	s.watches[taskID][watcher] = struct{}{}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.watches[taskID], watcher); close(watcher); s.mu.Unlock() }()
	for _, chunk := range history {
		if err := writeChunk(r.Context(), conn, chunk); err != nil {
			return
		}
		if chunk.Event == "task-ended" {
			return
		}
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case chunk := <-watcher:
			if err := writeChunk(r.Context(), conn, chunk); err != nil {
				return
			}
			if chunk.Event == "task-ended" {
				return
			}
		}
	}
}

func (s *Server) autoApproveTask(w http.ResponseWriter, r *http.Request) {
	var req taskflow.TaskApproveReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteTask(req.ID.String()); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/auto-approve", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
	}
	respond(w, http.StatusOK, nil)
}

func (s *Server) answerTaskQuestion(w http.ResponseWriter, r *http.Request) {
	var req taskflow.AskUserQuestionResponse
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteTask(req.TaskId); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/task/ask-user-question", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
	}
	respond(w, http.StatusOK, nil)
}

func writeChunk(ctx context.Context, conn *websocket.Conn, chunk *taskflow.TaskChunk) error {
	payload, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, payload)
}
