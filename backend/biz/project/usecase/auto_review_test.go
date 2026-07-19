package usecase

import (
	"slices"
	"testing"

	"github.com/Y-vQv-Y/DevLoom/backend/consts"
)

func TestAutoReviewEventsSubscribeGithubComments(t *testing.T) {
	events := autoReviewEvents(consts.GitPlatformGithub)
	for _, event := range []string{"pull_request", "issue_comment"} {
		if !slices.Contains(events, event) {
			t.Fatalf("GitHub auto-review events %v do not include %q", events, event)
		}
	}
}
