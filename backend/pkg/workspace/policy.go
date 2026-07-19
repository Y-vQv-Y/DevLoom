// Package workspace defines the isolation contract shared by DevLoom and its
// external development runtime.
package workspace

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

const (
	PushModePullRequest = "pull_request"
	PushModeDisabled    = "disabled"
)

// Policy creates a deterministic, task-scoped branch policy. The external
// runtime must clone BaseBranch, create WorkBranch, and never push BaseBranch.
func Policy(cfg config.WorkspaceConfig, baseBranch string, taskID uuid.UUID, kind string) taskflow.WorkspacePolicy {
	baseBranch = strings.TrimSpace(baseBranch)
	if baseBranch == "" {
		baseBranch = "main"
	}
	prefix := sanitizeSegment(cfg.BranchPrefix)
	if prefix == "" {
		prefix = "devloom"
	}
	kind = sanitizeSegment(kind)
	if kind == "" {
		kind = "task"
	}
	pushMode := strings.TrimSpace(cfg.PushMode)
	switch pushMode {
	case PushModePullRequest, PushModeDisabled:
	default:
		pushMode = PushModePullRequest
	}

	return taskflow.WorkspacePolicy{
		Isolated:          cfg.Isolated,
		BaseBranch:        baseBranch,
		WorkBranch:        fmt.Sprintf("%s/%s/%s", prefix, kind, taskID.String()),
		PushMode:          pushMode,
		ProtectedBranch:   cfg.Isolated,
		OpenHandsWorktree: cfg.Isolated && cfg.OpenHandsWorktree,
	}
}

// Env returns reserved environment variables for runtimes that have not yet
// implemented the structured WorkspacePolicy field.
func Env(policy taskflow.WorkspacePolicy) map[string]string {
	return map[string]string{
		"DEVLOOM_WORKSPACE_ISOLATED":         fmt.Sprintf("%t", policy.Isolated),
		"DEVLOOM_WORKSPACE_BASE_BRANCH":      policy.BaseBranch,
		"DEVLOOM_WORKSPACE_BRANCH":           policy.WorkBranch,
		"DEVLOOM_WORKSPACE_PUSH_MODE":        policy.PushMode,
		"DEVLOOM_WORKSPACE_PROTECTED_BRANCH": fmt.Sprintf("%t", policy.ProtectedBranch),
		"OPENHANDS_WORKTREE":                 fmt.Sprintf("%t", policy.OpenHandsWorktree),
	}
}

// Environ returns a deterministic KEY=value representation for VM creation.
func Environ(policy taskflow.WorkspacePolicy) []string {
	values := Env(policy)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

// AppendSystemPrompt documents the non-negotiable Git boundaries for agents.
// Runtime-level branch protection remains authoritative.
func AppendSystemPrompt(current string, policy taskflow.WorkspacePolicy) string {
	if !policy.Isolated {
		return current
	}
	instruction := fmt.Sprintf("Repository safety policy: treat base branch %q as read-only. Work only on %q. Never force-push, merge, or delete protected branches.", policy.BaseBranch, policy.WorkBranch)
	if policy.PushMode == PushModeDisabled {
		instruction += " Do not push any branch or create a pull/merge request."
	} else {
		instruction += fmt.Sprintf(" Push only the work branch and open a pull/merge request when push mode is %q.", policy.PushMode)
	}
	if strings.TrimSpace(current) == "" {
		return instruction
	}
	return instruction + "\n\n" + current
}

func sanitizeSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-_")
}
