package taskflowserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

type Server struct {
	cfg     Config
	logger  *slog.Logger
	docker  *DockerRuntime
	mux     *http.ServeMux
	mu      sync.RWMutex
	stateMu sync.Mutex
	vms     map[string]*taskflow.VirtualMachine
	ports   map[string]map[string]*taskflow.PortForwardInfo
	taskVM  map[string]string
	events  map[string][]*taskflow.TaskChunk
	watches map[string]map[chan *taskflow.TaskChunk]struct{}
	cancels map[string]func()
	tasks   map[string]taskflow.CreateTaskReq
	runners map[string]*runnerRegistration
	redis   *redis.Client
}

func New(cfg Config, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(cfg.WorkspaceRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}
	s := &Server{
		cfg: cfg, logger: logger, mux: http.NewServeMux(),
		vms:     make(map[string]*taskflow.VirtualMachine),
		ports:   make(map[string]map[string]*taskflow.PortForwardInfo),
		taskVM:  make(map[string]string),
		events:  make(map[string][]*taskflow.TaskChunk),
		watches: make(map[string]map[chan *taskflow.TaskChunk]struct{}),
		cancels: make(map[string]func()),
		tasks:   make(map[string]taskflow.CreateTaskReq),
		runners: make(map[string]*runnerRegistration),
	}
	s.docker = NewDockerRuntime(cfg, logger)
	if cfg.RedisAddr != "" && !cfg.RunnerMode {
		s.redis = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	}
	if err := s.loadFileState(); err != nil {
		return nil, fmt.Errorf("load taskflow state: %w", err)
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	handler := http.Handler(s.mux)
	if s.cfg.RunnerMode {
		handler = s.runnerAuth(handler)
	}
	return requestLogger(s.logger, handler)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("POST /internal/runner/register", s.registerRunner)
	s.mux.HandleFunc("POST /internal/runner/task-status", s.runnerTaskStatus)
	s.mux.HandleFunc("GET /internal/stats", s.stats)
	s.mux.HandleFunc("GET /internal/host/list", s.hostList)
	s.mux.HandleFunc("POST /internal/host/is-online", s.hostOnline)
	s.mux.HandleFunc("POST /internal/vm", s.createVM)
	s.mux.HandleFunc("DELETE /internal/vm", s.deleteVM)
	s.mux.HandleFunc("GET /internal/vm/list", s.listVM)
	s.mux.HandleFunc("GET /internal/vm/info", s.infoVM)
	s.mux.HandleFunc("POST /internal/vm/is-online", s.vmOnline)
	s.mux.HandleFunc("POST /internal/vm/hibernate", s.hibernateVM)
	s.mux.HandleFunc("POST /internal/vm/resume", s.resumeVM)
	s.mux.HandleFunc("POST /internal/files", s.files)
	s.mux.HandleFunc("GET /internal/ws/files/download", s.downloadFile)
	s.mux.HandleFunc("GET /internal/ws/files/upload", s.uploadFile)
	s.mux.HandleFunc("GET /internal/ws/terminal", s.terminal)
	s.mux.HandleFunc("GET /internal/terminal", s.listTerminals)
	s.mux.HandleFunc("DELETE /internal/terminal", s.closeTerminal)
	s.mux.HandleFunc("GET /internal/ws/reports", s.reports)
	s.mux.HandleFunc("GET /internal/port-forward", s.listPorts)
	s.mux.HandleFunc("POST /internal/port-forward", s.createPort)
	s.mux.HandleFunc("PUT /internal/port-forward", s.updatePort)
	s.mux.HandleFunc("POST /internal/port-forward/close", s.closePort)
	s.mux.HandleFunc("POST /internal/task", s.createTask)
	s.mux.HandleFunc("POST /internal/task/stop", s.stopTask)
	s.mux.HandleFunc("POST /internal/task/cancel", s.stopTask)
	s.mux.HandleFunc("POST /internal/task/continue", s.continueTask)
	s.mux.HandleFunc("POST /internal/task/restart", s.restartTask)
	s.mux.HandleFunc("POST /internal/task/auto-approve", s.autoApproveTask)
	s.mux.HandleFunc("POST /internal/task/ask-user-question", s.answerTaskQuestion)
	s.mux.HandleFunc("POST /internal/task/repo-list-files", s.repoListFiles)
	s.mux.HandleFunc("POST /internal/task/repo-read-file", s.repoReadFile)
	s.mux.HandleFunc("POST /internal/task/repo-file-diff", s.repoFileDiff)
	s.mux.HandleFunc("POST /internal/task/repo-file-changes", s.repoFileChanges)
	s.mux.HandleFunc("GET /internal/ws/task-live", s.taskLive)
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(taskflow.Resp[any]{Code: 0, Message: "ok", Data: data})
}

func fail(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(taskflow.Resp[any]{Code: status, Message: err.Error()})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ack(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, nil)
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func statusFor(err error) int {
	if errors.Is(err, os.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}
