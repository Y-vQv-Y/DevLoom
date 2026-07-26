package taskflowserver

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

type DockerRuntime struct {
	cfg    Config
	logger *slog.Logger
}

func NewDockerRuntime(cfg Config, logger *slog.Logger) *DockerRuntime {
	return &DockerRuntime{cfg: cfg, logger: logger}
}

func (d *DockerRuntime) containerName(id string) (string, error) {
	if !safeIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid VM ID %q", id)
	}
	return d.cfg.ContainerPrefix + "-" + strings.ToLower(id), nil
}

func (d *DockerRuntime) command(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, d.cfg.DockerBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("docker %s: %s", args[0], message)
	}
	return stdout.Bytes(), nil
}

func (d *DockerRuntime) Create(ctx context.Context, req *taskflow.CreateVirtualMachineReq, workspace string) error {
	name, err := d.containerName(req.ID)
	if err != nil {
		return err
	}
	image := strings.TrimSpace(req.ImageURL)
	if image == "" {
		image = d.cfg.DefaultImage
	}
	workspaceHost := strings.TrimRight(d.cfg.WorkspaceHostRoot, "/\\") + "/" + req.ID
	args := []string{
		"run", "--detach", "--name", name,
		"--label", "devloom.managed=true",
		"--label", "devloom.vm.id=" + req.ID,
		"--label", "devloom.host.id=" + d.cfg.HostID,
		"--restart", "unless-stopped",
		"--workdir", "/workspace",
		"--mount", "type=bind,src=" + workspaceHost + ",dst=/workspace",
	}
	if req.Cores != "" {
		if cpus, parseErr := strconv.ParseFloat(req.Cores, 64); parseErr == nil && cpus > 0 {
			args = append(args, "--cpus", req.Cores)
		}
	}
	if req.Memory > 0 {
		args = append(args, "--memory", strconv.FormatUint(req.Memory, 10))
	}
	for _, item := range req.Envs {
		if strings.Contains(item, "=") {
			args = append(args, "--env", item)
		}
	}
	args = append(args, image, "sh", "-lc", "trap : TERM INT; while :; do sleep 3600; done")
	if _, err := d.command(ctx, args...); err != nil {
		return err
	}
	d.logger.Info("VM container created", "vm_id", req.ID, "container", name, "workspace", workspace)
	return nil
}

func (d *DockerRuntime) Delete(ctx context.Context, id string) error {
	name, err := d.containerName(id)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, "rm", "--force", name)
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}

func (d *DockerRuntime) Stop(ctx context.Context, id string) error {
	name, err := d.containerName(id)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, "stop", "--time", "20", name)
	return err
}

func (d *DockerRuntime) Start(ctx context.Context, id string) error {
	name, err := d.containerName(id)
	if err != nil {
		return err
	}
	_, err = d.command(ctx, "start", name)
	return err
}

func (d *DockerRuntime) IsRunning(ctx context.Context, id string) bool {
	name, err := d.containerName(id)
	if err != nil {
		return false
	}
	out, err := d.command(ctx, "inspect", "--format", "{{.State.Running}}", name)
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func (d *DockerRuntime) Exec(ctx context.Context, id, command string) ([]byte, error) {
	name, err := d.containerName(id)
	if err != nil {
		return nil, err
	}
	return d.command(ctx, "exec", name, "sh", "-lc", command)
}

func (d *DockerRuntime) Process(ctx context.Context, id, command string) (*exec.Cmd, error) {
	name, err := d.containerName(id)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, d.cfg.DockerBinary, "exec", "-i", name, "sh", "-lc", command)
	return cmd, nil
}

func commandContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}
