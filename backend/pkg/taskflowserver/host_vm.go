package taskflowserver

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func (s *Server) host() *taskflow.Host {
	return &taskflow.Host{
		ID: s.cfg.HostID, Hostname: s.cfg.HostName, Name: "DevLoom local Docker",
		Arch: s.cfg.hostArch(), OS: runtime.GOOS, Cores: int32(runtime.NumCPU()),
		PublicIP: s.cfg.PublicIP, InternalIP: s.cfg.PublicIP,
		TTL: taskflow.TTL{Kind: taskflow.TTLForever}, CreatedAt: time.Now().Unix(), Version: "source",
	}
}

func (s *Server) hostList(w http.ResponseWriter, r *http.Request) {
	host := s.host()
	host.UserID = r.URL.Query().Get("user_id")
	result := map[string]*taskflow.Host{host.ID: host}
	s.mu.RLock()
	for id, registration := range s.runners {
		if time.Since(registration.UpdatedAt) <= 45*time.Second {
			clone := registration.Host
			clone.UserID = host.UserID
			result[id] = &clone
		}
	}
	s.mu.RUnlock()
	respond(w, http.StatusOK, result)
}

func (s *Server) hostOnline(w http.ResponseWriter, r *http.Request) {
	var req taskflow.IsOnlineReq[string]
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	result := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		result[id] = id == s.cfg.HostID || s.runner(id) != nil
	}
	respond(w, http.StatusOK, &taskflow.IsOnlineResp{OnlineMap: result})
}

func (s *Server) createVM(w http.ResponseWriter, r *http.Request) {
	var req taskflow.CreateVirtualMachineReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if req.ID == "" {
		req.ID = req.TaskID.String()
	}
	if !safeIDPattern.MatchString(req.ID) {
		fail(w, http.StatusBadRequest, fmt.Errorf("invalid VM ID"))
		return
	}
	if req.HostID != "" && req.HostID != s.cfg.HostID {
		registration := s.runner(req.HostID)
		if registration == nil {
			fail(w, http.StatusNotFound, fmt.Errorf("host %s is not registered", req.HostID))
			return
		}
		var vm taskflow.VirtualMachine
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/vm", &req, &vm); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		s.rememberVM(&vm)
		if taskID := req.TaskID.String(); taskID != "00000000-0000-0000-0000-000000000000" {
			s.rememberTaskVM(taskID, vm.ID)
		}
		respond(w, http.StatusOK, &vm)
		s.scheduleVMReady(vm)
		return
	}
	workspace := filepath.Join(s.cfg.WorkspaceRoot, req.ID)
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.prepareWorkspace(r.Context(), workspace, req.Git, req.ZipUrl, req.Workspace); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	if err := s.docker.Create(ctx, &req, workspace); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	now := time.Now().Unix()
	vm := &taskflow.VirtualMachine{
		ID: req.ID, EnvironmentID: req.ID, HostID: s.cfg.HostID, Hostname: req.HostName,
		Arch: s.cfg.hostArch(), OS: "linux", Name: req.ID, Repository: req.Git.URL,
		Status: taskflow.VirtualMachineStatusOnline, Cores: 2, Memory: req.Memory,
		TTL: taskflow.TTL{Kind: taskflow.TTLForever}, ExternalIP: s.cfg.PublicIP,
		CreatedAt: now, Version: "source",
	}
	s.rememberVM(vm)
	if taskID := req.TaskID.String(); taskID != "00000000-0000-0000-0000-000000000000" {
		s.rememberTaskVM(taskID, vm.ID)
	}
	respond(w, http.StatusOK, vm)
	s.scheduleVMReady(*vm)
}

func (s *Server) prepareWorkspace(ctx context.Context, workspace string, git taskflow.Git, zipURL string, policy *taskflow.WorkspacePolicy) error {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	if strings.TrimSpace(git.URL) != "" {
		args := []string{"clone", "--depth", "1"}
		baseBranch := strings.TrimSpace(git.Branch)
		if policy != nil && policy.Isolated && strings.TrimSpace(policy.BaseBranch) != "" {
			baseBranch = strings.TrimSpace(policy.BaseBranch)
		}
		if baseBranch != "" {
			if err := validateGitBranch(ctx, baseBranch); err != nil {
				return fmt.Errorf("invalid base branch: %w", err)
			}
			args = append(args, "--branch", baseBranch)
		}
		args = append(args, git.URL, workspace)
		cmd, cleanup, err := s.authenticatedGitCommand(ctx, git, args...)
		if err != nil {
			return err
		}
		defer cleanup()
		if output, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return fmt.Errorf("git clone: %s", strings.TrimSpace(string(output)))
		}
		if strings.TrimSpace(git.Username) != "" {
			if output, cmdErr := s.dockerGitCommand(ctx, "-C", workspace, "config", "user.name", git.Username).CombinedOutput(); cmdErr != nil {
				return fmt.Errorf("configure git user: %s", strings.TrimSpace(string(output)))
			}
		}
		if strings.TrimSpace(git.Email) != "" {
			if output, cmdErr := s.dockerGitCommand(ctx, "-C", workspace, "config", "user.email", git.Email).CombinedOutput(); cmdErr != nil {
				return fmt.Errorf("configure git email: %s", strings.TrimSpace(string(output)))
			}
		}
		if policy != nil && policy.Isolated {
			if err := s.prepareIsolatedBranch(ctx, workspace, *policy); err != nil {
				return err
			}
		}
		return nil
	}
	if strings.TrimSpace(zipURL) != "" {
		return downloadAndExtractZip(ctx, zipURL, workspace)
	}
	return nil
}

func (s *Server) dockerGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", args...)
}

func (s *Server) authenticatedGitCommand(ctx context.Context, git taskflow.Git, args ...string) (*exec.Cmd, func(), error) {
	cmd := s.dockerGitCommand(ctx, args...)
	if strings.TrimSpace(git.Token) == "" {
		return cmd, func() {}, nil
	}
	authDir, err := os.MkdirTemp("", "devloom-git-auth-")
	if err != nil {
		return nil, nil, fmt.Errorf("create git authentication helper: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(authDir) }
	path := filepath.Join(authDir, "askpass.sh")
	script := "#!/bin/sh\ncase \"$1\" in\n  *Username*) printf '%s\\n' \"$DEVLOOM_GIT_USERNAME\" ;;\n  *) printf '%s\\n' \"$DEVLOOM_GIT_TOKEN\" ;;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write git authentication helper: %w", err)
	}
	username := strings.TrimSpace(git.Username)
	if username == "" {
		username = "oauth2"
	}
	cmd.Env = append(os.Environ(),
		"GIT_ASKPASS="+path,
		"GIT_TERMINAL_PROMPT=0",
		"DEVLOOM_GIT_USERNAME="+username,
		"DEVLOOM_GIT_TOKEN="+git.Token,
	)
	return cmd, cleanup, nil
}

func validateGitBranch(ctx context.Context, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}
	if output, err := exec.CommandContext(ctx, "git", "check-ref-format", "--branch", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Server) prepareIsolatedBranch(ctx context.Context, workspace string, policy taskflow.WorkspacePolicy) error {
	baseBranch := strings.TrimSpace(policy.BaseBranch)
	workBranch := strings.TrimSpace(policy.WorkBranch)
	if err := validateGitBranch(ctx, baseBranch); err != nil {
		return fmt.Errorf("invalid protected branch: %w", err)
	}
	if err := validateGitBranch(ctx, workBranch); err != nil {
		return fmt.Errorf("invalid work branch: %w", err)
	}
	if baseBranch == workBranch {
		return fmt.Errorf("work branch must differ from protected branch")
	}
	if output, err := s.dockerGitCommand(ctx, "-C", workspace, "checkout", "-b", workBranch).CombinedOutput(); err != nil {
		return fmt.Errorf("create isolated work branch: %s", strings.TrimSpace(string(output)))
	}
	hookDir := filepath.Join(workspace, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o750); err != nil {
		return fmt.Errorf("create git hook directory: %w", err)
	}
	hook := fmt.Sprintf(`#!/bin/sh
base_ref=%s
work_ref=%s
push_mode=%s
if [ "$push_mode" = "disabled" ]; then
  echo "DevLoom policy rejects all pushes for this task" >&2
  exit 1
fi
while read -r _local_ref _local_sha remote_ref _remote_sha; do
  if [ "$remote_ref" = "$base_ref" ]; then
    echo "DevLoom policy rejects pushes to protected branch $base_ref" >&2
    exit 1
  fi
  if [ "$remote_ref" != "$work_ref" ]; then
    echo "DevLoom policy permits only $work_ref" >&2
    exit 1
  fi
done
`, shellQuote("refs/heads/"+baseBranch), shellQuote("refs/heads/"+workBranch), shellQuote(strings.TrimSpace(policy.PushMode)))
	if err := os.WriteFile(filepath.Join(hookDir, "pre-push"), []byte(hook), 0o750); err != nil {
		return fmt.Errorf("install protected branch hook: %w", err)
	}
	return nil
}

func (s *Server) deleteVM(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if registration, remote := s.remoteVM(id); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	ctx, cancel := commandContext()
	defer cancel()
	if err := s.docker.Delete(ctx, id); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	s.forgetVM(id)
	respond(w, http.StatusOK, nil)
}

func (s *Server) listVM(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if registration, remote := s.remoteVM(id); id != "" && remote {
		s.proxyRunner(w, r, registration)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*taskflow.VirtualMachine, 0, len(s.vms))
	for _, vm := range s.vms {
		if id == "" || vm.ID == id {
			clone := *vm
			result = append(result, &clone)
		}
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) infoVM(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if registration, remote := s.remoteVM(id); remote {
		s.proxyRunner(w, r, registration)
		return
	}
	vm := s.knownVM(id)
	if vm == nil {
		fail(w, http.StatusNotFound, fmt.Errorf("environment not found: %s", id))
		return
	}
	clone := *vm
	clone.Status = s.vmStatus(r.Context(), id)
	respond(w, http.StatusOK, &clone)
}

func (s *Server) vmStatus(ctx context.Context, id string) taskflow.VirtualMachineStatus {
	if s.docker.IsRunning(ctx, id) {
		return taskflow.VirtualMachineStatusOnline
	}
	return taskflow.VirtualMachineStatusOffline
}

func (s *Server) vmOnline(w http.ResponseWriter, r *http.Request) {
	var req taskflow.IsOnlineReq[string]
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	result := make(map[string]bool, len(req.IDs))
	for _, id := range req.IDs {
		if registration, remote := s.remoteVM(id); remote {
			result[id] = registration != nil
		} else {
			result[id] = s.docker.IsRunning(r.Context(), id)
		}
	}
	respond(w, http.StatusOK, &taskflow.IsOnlineResp{OnlineMap: result})
}

func (s *Server) hibernateVM(w http.ResponseWriter, r *http.Request) {
	var req taskflow.HibernateVirtualMachineReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/vm/hibernate", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, nil)
		return
	}
	ctx, cancel := commandContext()
	defer cancel()
	if err := s.docker.Stop(ctx, req.ID); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if vm := s.knownVM(req.ID); vm != nil {
		vm.Status = taskflow.VirtualMachineStatusHibernated
		s.rememberVM(vm)
	}
	respond(w, http.StatusOK, nil)
}

func (s *Server) resumeVM(w http.ResponseWriter, r *http.Request) {
	var req taskflow.ResumeVirtualMachineReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if registration, remote := s.remoteVM(req.ID); remote {
		if err := s.forwardJSON(r, registration, http.MethodPost, "/internal/vm/resume", &req, nil); err != nil {
			fail(w, http.StatusBadGateway, err)
			return
		}
		respond(w, http.StatusOK, nil)
		return
	}
	ctx, cancel := commandContext()
	defer cancel()
	if err := s.docker.Start(ctx, req.ID); err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	if vm := s.knownVM(req.ID); vm != nil {
		vm.Status = taskflow.VirtualMachineStatusOnline
		s.rememberVM(vm)
	}
	respond(w, http.StatusOK, nil)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	online := 0
	for id := range s.vms {
		if s.docker.IsRunning(r.Context(), id) {
			online++
		}
	}
	respond(w, http.StatusOK, &taskflow.Stats{OnlineHostCount: 1, OnlineVMCount: online, OnlineTaskCount: len(s.cancels)})
}

func downloadAndExtractZip(ctx context.Context, rawURL, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download zip: HTTP %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp("", "devloom-workspace-*.zip")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = io.Copy(tmp, io.LimitReader(resp.Body, 1<<30)); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	archive, err := zip.OpenReader(name)
	if err != nil {
		return err
	}
	defer archive.Close()
	root := filepath.Clean(destination) + string(os.PathSeparator)
	for _, file := range archive.File {
		target := filepath.Join(destination, file.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), root) {
			return fmt.Errorf("zip entry escapes workspace: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode()&0o777)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(output, source)
		source.Close()
		output.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}
