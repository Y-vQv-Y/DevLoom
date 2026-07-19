package v1

import (
	"testing"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
)

func TestBuildGitlabNoteTask(t *testing.T) {
	cfg := &config.Config{}
	cfg.Task.AtKeyword = "@devloom"
	bot := &domain.GitBot{Token: "git-token", Host: &domain.Host{ID: "host-1"}}
	payload := []byte(`{
        "object_attributes": {"id": 42, "note": "@DevLoom fix the failing test", "noteable_type": "MergeRequest"},
        "merge_request": {"id": 9001, "iid": 7, "title": "Improve tests", "state": "opened", "url": "https://gitlab.example/group/repo/-/merge_requests/7", "source_branch": "fix/tests"},
        "project": {"id": 12, "name": "repo", "path_with_namespace": "group/repo", "web_url": "https://gitlab.example/group/repo", "git_http_url": "https://gitlab.example/group/repo.git", "visibility": "private"},
        "user": {"username": "alice", "name": "Alice", "email": "alice@example.com"}
    }`)

	req, key, ok, err := buildGitlabNoteTask(bot, cfg, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || key != "gitlab:note:12:42" {
		t.Fatalf("unexpected note result: ok=%v key=%q", ok, key)
	}
	if req.Prompt != "fix the failing test" {
		t.Fatalf("prompt = %q", req.Prompt)
	}
	if req.Command != "fix" {
		t.Fatalf("command = %q", req.Command)
	}
	if req.HostID != "host-1" || req.Platform != consts.GitPlatformGitLab {
		t.Fatalf("unexpected runtime: host=%q platform=%q", req.HostID, req.Platform)
	}
	if req.Subject.Number != 7 || req.Repo.URL != "https://gitlab.example/group/repo.git" {
		t.Fatalf("unexpected merge request metadata: subject=%+v repo=%+v", req.Subject, req.Repo)
	}
}

func TestBuildGitlabNoteTaskIgnoresNonMention(t *testing.T) {
	cfg := &config.Config{}
	cfg.Task.AtKeyword = "@devloom"
	bot := &domain.GitBot{Host: &domain.Host{ID: "host-1"}}
	payload := []byte(`{"object_attributes":{"id":1,"note":"ordinary comment","noteable_type":"MergeRequest"},"merge_request":{"iid":1,"state":"opened"},"project":{"id":2},"user":{"username":"alice"}}`)

	_, _, ok, err := buildGitlabNoteTask(bot, cfg, payload)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("ordinary GitLab comments must not create tasks")
	}
}

func TestWebhookRuntimeUsesPersistedHostID(t *testing.T) {
	bot := &domain.GitBot{
		HostID: "actual-host-id",
		Host:   &domain.Host{ID: consts.PUBLIC_HOST_ID},
	}

	hostID, err := webhookRuntime(bot)
	if err != nil {
		t.Fatal(err)
	}
	if hostID != "actual-host-id" {
		t.Fatalf("runtime host = %q, want %q", hostID, "actual-host-id")
	}
}

func TestGitlabProjectIsPrivateSupportsLegacyVisibilityLevel(t *testing.T) {
	if !gitlabProjectIsPrivate("", float64(0)) {
		t.Fatal("GitLab visibility_level=0 must be treated as private")
	}
	if gitlabProjectIsPrivate("", float64(20)) {
		t.Fatal("GitLab visibility_level=20 must be treated as public")
	}
}
