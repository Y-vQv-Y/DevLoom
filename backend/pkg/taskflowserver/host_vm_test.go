package taskflowserver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed in the test environment")
	}
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func createTestRepository(t *testing.T) string {
	t.Helper()
	repository := filepath.Join(t.TempDir(), "origin")
	runGit(t, "init", "--initial-branch=main", repository)
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("source\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	runGit(t, "-C", repository, "add", "README.md")
	runGit(t, "-C", repository, "-c", "user.name=DevLoom Test", "-c", "user.email=test@devloom.local", "commit", "-m", "initial")
	return repository
}

func TestPrepareWorkspaceCreatesIsolatedBranchAndPushGuard(t *testing.T) {
	requireGit(t)
	server := testServer(t)
	repository := createTestRepository(t)
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		t.Fatal(err)
	}
	policy := &taskflow.WorkspacePolicy{Isolated: true, BaseBranch: "main", WorkBranch: "devloom/develop/task-1", PushMode: "pull_request", ProtectedBranch: true}
	git := taskflow.Git{URL: repository, Branch: "main", Username: "ADTEC Agent", Email: "agent@adtec.local"}
	if err := server.prepareWorkspace(context.Background(), workspace, git, "", policy); err != nil {
		t.Fatalf("prepareWorkspace() error = %v", err)
	}
	if branch := runGit(t, "-C", workspace, "branch", "--show-current"); branch != policy.WorkBranch {
		t.Fatalf("branch = %q, want %q", branch, policy.WorkBranch)
	}
	if name := runGit(t, "-C", workspace, "config", "user.name"); name != git.Username {
		t.Fatalf("user.name = %q, want %q", name, git.Username)
	}
	hook, err := os.ReadFile(filepath.Join(workspace, ".git", "hooks", "pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"refs/heads/main", "refs/heads/devloom/develop/task-1", "permits only"} {
		if !strings.Contains(string(hook), expected) {
			t.Errorf("pre-push hook missing %q", expected)
		}
	}
}

func TestPrepareWorkspaceRejectsInvalidPolicyBranch(t *testing.T) {
	requireGit(t)
	server := testServer(t)
	repository := createTestRepository(t)
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		t.Fatal(err)
	}
	policy := &taskflow.WorkspacePolicy{Isolated: true, BaseBranch: "main", WorkBranch: "../protected", PushMode: "pull_request"}
	err := server.prepareWorkspace(context.Background(), workspace, taskflow.Git{URL: repository, Branch: "main"}, "", policy)
	if err == nil || !strings.Contains(err.Error(), "invalid work branch") {
		t.Fatalf("prepareWorkspace() error = %v, want invalid work branch", err)
	}
}

func TestAuthenticatedGitCommandKeepsTokenOutOfArguments(t *testing.T) {
	server := testServer(t)
	const token = "private-token-value"
	cmd, cleanup, err := server.authenticatedGitCommand(context.Background(), taskflow.Git{Token: token, Username: "git-user"}, "version")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if strings.Contains(strings.Join(cmd.Args, " "), token) {
		t.Fatal("git token leaked into process arguments")
	}
	joined := strings.Join(cmd.Env, "\n")
	if !strings.Contains(joined, "GIT_ASKPASS=") || !strings.Contains(joined, "DEVLOOM_GIT_TOKEN="+token) {
		t.Fatal("git authentication environment is incomplete")
	}
}
