package workspace

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
)

func TestPolicyCreatesIsolatedTaskBranch(t *testing.T) {
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	policy := Policy(config.WorkspaceConfig{
		Isolated:          true,
		BranchPrefix:      "Team Dev",
		PushMode:          PushModePullRequest,
		OpenHandsWorktree: true,
	}, "main", id, "review")

	if policy.BaseBranch != "main" || policy.WorkBranch != "teamdev/review/11111111-2222-3333-4444-555555555555" {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	if !policy.ProtectedBranch || !policy.OpenHandsWorktree {
		t.Fatalf("isolation flags not enabled: %+v", policy)
	}
	if got := Env(policy)["DEVLOOM_WORKSPACE_PUSH_MODE"]; got != PushModePullRequest {
		t.Fatalf("push mode env = %q", got)
	}
}

func TestAppendSystemPromptProtectsBaseBranch(t *testing.T) {
	policy := Policy(config.WorkspaceConfig{Isolated: true}, "release", uuid.New(), "task")
	prompt := AppendSystemPrompt("Implement the issue.", policy)
	for _, want := range []string{"base branch \"release\" as read-only", "Never force-push", "Implement the issue."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
}
