package taskflowserver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	Listen                  string
	HostID                  string
	HostName                string
	DockerBinary            string
	ContainerPrefix         string
	WorkspaceRoot           string
	WorkspaceHostRoot       string
	PreviewBaseURL          string
	BackendInternalURL      string
	TaskStatusCallbackURL   string
	TaskStatusCallbackToken string
	PublicIP                string
	DefaultImage            string
	MaxAgentSteps           int
	RunnerSecret            string
	RunnerMode              bool
	RedisAddr               string
	RedisPassword           string
	StateFile               string
}

func ConfigFromEnv() (Config, error) {
	hostname, _ := os.Hostname()
	cfg := Config{
		Listen:                  env("DEVLOOM_TASKFLOW_LISTEN", ":8888"),
		HostID:                  env("DEVLOOM_TASKFLOW_HOST_ID", "local-docker"),
		HostName:                env("DEVLOOM_TASKFLOW_HOST_NAME", hostname),
		DockerBinary:            env("DEVLOOM_DOCKER_BINARY", "docker"),
		ContainerPrefix:         env("DEVLOOM_VM_CONTAINER_PREFIX", "devloom-vm"),
		WorkspaceRoot:           env("DEVLOOM_WORKSPACE_ROOT", "/workspaces"),
		WorkspaceHostRoot:       env("DEVLOOM_WORKSPACE_HOST_ROOT", "/var/lib/devloom/workspaces"),
		PreviewBaseURL:          strings.TrimRight(env("DEVLOOM_PREVIEW_BASE_URL", "http://127.0.0.1:9080"), "/"),
		BackendInternalURL:      strings.TrimRight(strings.TrimSpace(os.Getenv("DEVLOOM_BACKEND_INTERNAL_URL")), "/"),
		TaskStatusCallbackURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("DEVLOOM_TASK_STATUS_CALLBACK_URL")), "/"),
		TaskStatusCallbackToken: strings.TrimSpace(os.Getenv("DEVLOOM_TASK_STATUS_CALLBACK_TOKEN")),
		PublicIP:                env("DEVLOOM_PUBLIC_IP", "127.0.0.1"),
		DefaultImage:            env("DEVLOOM_DEFAULT_DEVBOX_IMAGE", "devloom.local/devloom-devbox:local"),
		MaxAgentSteps:           envInt("DEVLOOM_AGENT_MAX_STEPS", 30),
		RunnerSecret:            runnerSecretFromEnv(),
		RunnerMode:              strings.EqualFold(strings.TrimSpace(os.Getenv("DEVLOOM_RUNNER_MODE")), "true"),
		RedisAddr:               redisAddrFromEnv(),
		RedisPassword:           strings.TrimSpace(os.Getenv("DEVLOOM_REDIS_PASS")),
		StateFile:               strings.TrimSpace(os.Getenv("DEVLOOM_TASKFLOW_STATE_FILE")),
	}
	if cfg.TaskStatusCallbackURL == "" && cfg.BackendInternalURL != "" {
		cfg.TaskStatusCallbackURL = cfg.BackendInternalURL + "/internal/task-status"
	}
	if cfg.HostID == "" || cfg.WorkspaceRoot == "" || cfg.WorkspaceHostRoot == "" {
		return Config{}, fmt.Errorf("host ID and workspace paths must not be empty")
	}
	abs, err := filepath.Abs(cfg.WorkspaceRoot)
	if err != nil {
		return Config{}, fmt.Errorf("workspace root: %w", err)
	}
	cfg.WorkspaceRoot = abs
	if cfg.StateFile == "" && cfg.RunnerMode {
		cfg.StateFile = filepath.Join(cfg.WorkspaceRoot, ".devloom-runner-state.json")
	}
	return cfg, nil
}

func redisAddrFromEnv() string {
	host := strings.TrimSpace(os.Getenv("DEVLOOM_REDIS_HOST"))
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":6379"
}

func runnerSecretFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("DEVLOOM_RUNNER_SIGNING_SECRET")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("DEVLOOM_RUNNER_SHARED_SECRET"))
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (c Config) hostArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	}
	return runtime.GOARCH
}
