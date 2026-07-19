package v1

import (
	"context"
	"testing"

	"github.com/google/go-github/v74/github"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
)

func TestBuildGithubIssueCommentTask(t *testing.T) {
	h := &GithubWebhookHandler{
		cfg: &config.Config{Task: config.Task{AtKeyword: "@devloom"}},
		loadPullRequest: func(context.Context, string, string, int) (*github.PullRequest, error) {
			return &github.PullRequest{
				ID:      github.Int64(9001),
				Number:  github.Int(7),
				Title:   github.String("Improve tests"),
				HTMLURL: github.String("https://github.example/group/repo/pull/7"),
				Head: &github.PullRequestBranch{
					Ref: github.String("fix/tests"),
					Repo: &github.Repository{
						ID:          github.Int64(12),
						Name:        github.String("repo"),
						FullName:    github.String("group/repo"),
						CloneURL:    github.String("https://github.example/group/repo.git"),
						Description: github.String("repository"),
						Private:     github.Bool(true),
					},
				},
			}, nil
		},
	}
	bot := &domain.GitBot{Token: "git-token", HostID: "host-1"}
	payload := []byte(`{
        "action":"created",
        "comment":{"id":42,"body":"@DevLoom explain the concurrency change","user":{"login":"alice","type":"User"}},
        "issue":{"number":7,"title":"Improve tests","html_url":"https://github.example/group/repo/pull/7","pull_request":{"url":"https://api.github.example/repos/group/repo/pulls/7"}},
        "repository":{"id":12,"full_name":"group/repo"}
    }`)

	req, key, ok, err := h.buildIssueCommentTask(context.Background(), bot, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || key != "github:comment:12:42" {
		t.Fatalf("unexpected result: ok=%v key=%q", ok, key)
	}
	if req.Command != "explain" || req.Prompt != "the concurrency change" {
		t.Fatalf("unexpected command: command=%q prompt=%q", req.Command, req.Prompt)
	}
	if req.Repo.Branch == nil || *req.Repo.Branch != "fix/tests" || req.HostID != "host-1" {
		t.Fatalf("unexpected runtime request: %+v", req)
	}
}
