package v1

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/GoYoko/web"
	"github.com/google/go-github/v74/github"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/gitreview"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

type githubPullRequestLoader func(context.Context, string, string, int) (*github.PullRequest, error)

// GithubWebhookHandler GitHub Webhook 处理器
type GithubWebhookHandler struct {
	cfg             *config.Config
	logger          *slog.Logger
	redis           *redis.Client
	gitbotUsecase   domain.GitBotUsecase
	gitTaskUsecase  domain.GitTaskUsecase
	loadPullRequest githubPullRequestLoader
}

// NewGithubWebhookHandler 创建 GitHub Webhook 处理器
func NewGithubWebhookHandler(i *do.Injector) (*GithubWebhookHandler, error) {
	h := &GithubWebhookHandler{
		cfg:             do.MustInvoke[*config.Config](i),
		logger:          do.MustInvoke[*slog.Logger](i).With("module", "GithubWebhookHandler"),
		redis:           do.MustInvoke[*redis.Client](i),
		gitbotUsecase:   do.MustInvoke[domain.GitBotUsecase](i),
		gitTaskUsecase:  do.MustInvoke[domain.GitTaskUsecase](i),
		loadPullRequest: loadGithubPullRequest,
	}

	w := do.MustInvoke[*web.Web](i)
	w.Group("/api/v1").POST("/github/webhook/:id", web.BaseHandler(h.Webhook))

	return h, nil
}

// Webhook 处理 GitHub Webhook 请求
func (h *GithubWebhookHandler) Webhook(c *web.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid id")
	}

	bot, err := h.gitbotUsecase.GetByID(ctx, id)
	if err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to get gitbot")
		return c.String(http.StatusNotFound, "bot not found")
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}

	// 验证签名
	sig := c.Request().Header.Get("X-Hub-Signature-256")
	if err := validateHMACSHA256(bot.SecretToken, sig, body); err != nil {
		h.logger.With("error", err).WarnContext(ctx, "github webhook signature validation failed")
		return c.String(http.StatusUnauthorized, "invalid signature")
	}

	event := c.Request().Header.Get("X-Github-Event")
	h.logger.With("bot", bot.ID, "body", string(body), "event", event).DebugContext(c.Request().Context(), "github webhook")
	switch event {
	case "pull_request":
		h.handlePullRequest(ctx, bot, body)
	case "issue_comment":
		h.handleIssueComment(ctx, bot, body)
	}

	return c.String(http.StatusOK, "ok")
}

type githubIssueCommentEvent struct {
	Action  string `json:"action"`
	Comment *struct {
		ID      int64  `json:"id"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		User    *struct {
			ID        int64  `json:"id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
			Email     string `json:"email"`
			Type      string `json:"type"`
		} `json:"user"`
	} `json:"comment"`
	Issue *struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`
	Repository *struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *GithubWebhookHandler) handleIssueComment(ctx context.Context, bot *domain.GitBot, payload []byte) {
	req, dedupKey, ok, err := h.buildIssueCommentTask(ctx, bot, payload)
	if err != nil {
		h.logger.With("error", err).WarnContext(ctx, "invalid github issue comment webhook")
		return
	}
	if !ok || !dedup(ctx, h.redis, dedupKey, h.logger) {
		return
	}
	if _, err := h.gitTaskUsecase.Create(ctx, req); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to create github comment task")
	}
}

func (h *GithubWebhookHandler) buildIssueCommentTask(ctx context.Context, bot *domain.GitBot, payload []byte) (domain.CreateGitTaskReq, string, bool, error) {
	var ev githubIssueCommentEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return domain.CreateGitTaskReq{}, "", false, fmt.Errorf("decode issue comment payload: %w", err)
	}
	if !strings.EqualFold(ev.Action, "created") || ev.Comment == nil || ev.Comment.User == nil || ev.Issue == nil || ev.Repository == nil || ev.Issue.PullRequest == nil {
		return domain.CreateGitTaskReq{}, "", false, nil
	}
	if strings.EqualFold(ev.Comment.User.Type, "Bot") {
		return domain.CreateGitTaskReq{}, "", false, nil
	}
	command, detail, mentioned := gitreview.ParseMention(ev.Comment.Body, h.cfg.Task.AtKeyword)
	if !mentioned {
		return domain.CreateGitTaskReq{}, "", false, nil
	}
	if h.loadPullRequest == nil {
		return domain.CreateGitTaskReq{}, "", false, fmt.Errorf("github pull request loader is not configured")
	}
	pr, err := h.loadPullRequest(ctx, bot.Token, ev.Repository.FullName, ev.Issue.Number)
	if err != nil {
		return domain.CreateGitTaskReq{}, "", false, fmt.Errorf("load pull request: %w", err)
	}
	if pr == nil || pr.Head == nil || pr.Head.Repo == nil {
		return domain.CreateGitTaskReq{}, "", false, fmt.Errorf("pull request head repository is missing")
	}
	hostID, err := webhookRuntime(bot)
	if err != nil {
		return domain.CreateGitTaskReq{}, "", false, err
	}

	repo := pr.Head.Repo
	branch := pr.Head.GetRef()
	prURL := firstNonEmpty(pr.GetHTMLURL(), ev.Issue.HTMLURL)
	return domain.CreateGitTaskReq{
		HostID:  hostID,
		Prompt:  detail,
		Command: string(command),
		Git:     taskflow.Git{Token: bot.Token},
		Subject: domain.Subject{
			ID: fmt.Sprintf("%d", pr.GetID()), Type: "PullRequest",
			Title: firstNonEmpty(pr.GetTitle(), ev.Issue.Title), URL: prURL, Number: ev.Issue.Number,
		},
		Repo: domain.Repo{
			ID: fmt.Sprintf("%d", repo.GetID()), Name: repo.GetName(),
			FullName: repo.GetFullName(), URL: firstNonEmpty(repo.GetCloneURL(), repo.GetHTMLURL()),
			Desc: repo.GetDescription(), IsPrivate: repo.GetPrivate(), Branch: &branch,
		},
		Platform: consts.GitPlatformGithub,
		User: domain.User{
			Name: ev.Comment.User.Login, AvatarURL: ev.Comment.User.AvatarURL, Email: ev.Comment.User.Email,
		},
		Body: ev.Comment.Body,
		Time: time.Now(),
		Env: map[string]string{
			"GITHUB_TOKEN":           bot.Token,
			"DEVLOOM_REVIEW_COMMAND": string(command),
		},
		Bot: bot,
	}, fmt.Sprintf("github:comment:%d:%d", ev.Repository.ID, ev.Comment.ID), true, nil
}

func loadGithubPullRequest(ctx context.Context, token, fullName string, number int) (*github.PullRequest, error) {
	parts := strings.SplitN(strings.TrimSpace(fullName), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid github repository name %q", fullName)
	}
	client := github.NewClient(nil)
	if strings.TrimSpace(token) != "" {
		client = client.WithAuthToken(token)
	}
	pr, _, err := client.PullRequests.Get(ctx, parts[0], parts[1], number)
	return pr, err
}

func (h *GithubWebhookHandler) handlePullRequest(ctx context.Context, bot *domain.GitBot, payload []byte) {
	var ev struct {
		Action      string `json:"action"`
		PullRequest *struct {
			ID      int    `json:"id"`
			Number  int    `json:"number"`
			Title   string `json:"title"`
			Body    string `json:"body"`
			State   string `json:"state"`
			HTMLURL string `json:"html_url"`
			Head    *struct {
				Ref  string `json:"ref"`
				Repo *struct {
					ID          int    `json:"id"`
					Name        string `json:"name"`
					FullName    string `json:"full_name"`
					HTMLURL     string `json:"html_url"`
					Description string `json:"description"`
					Private     bool   `json:"private"`
				} `json:"repo"`
			} `json:"head"`
			User *struct {
				ID        int    `json:"id"`
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				Email     string `json:"email"`
			} `json:"user"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to unmarshal github pr event")
		return
	}

	pr := ev.PullRequest
	if pr == nil || pr.Head == nil || pr.Head.Repo == nil || pr.User == nil {
		return
	}

	switch ev.Action {
	case "opened", "synchronize", "reopened":
	default:
		return
	}

	if !dedup(ctx, h.redis, pr.HTMLURL, h.logger) {
		return
	}

	hostID, err := webhookRuntime(bot)
	if err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "github webhook runtime is not configured")
		return
	}

	branch := pr.Head.Ref
	repo := pr.Head.Repo
	if _, err := h.gitTaskUsecase.Create(ctx, domain.CreateGitTaskReq{
		HostID:  hostID,
		Prompt:  pr.HTMLURL,
		Git:     taskflow.Git{Token: bot.Token},
		Subject: domain.Subject{
			ID:     fmt.Sprintf("%d", pr.ID),
			Type:   "PullRequest",
			Title:  pr.Title,
			URL:    pr.HTMLURL,
			Number: pr.Number,
		},
		Repo: domain.Repo{
			ID:        fmt.Sprintf("%d", repo.ID),
			Name:      repo.Name,
			FullName:  repo.FullName,
			URL:       repo.HTMLURL,
			Desc:      repo.Description,
			IsPrivate: repo.Private,
			Branch:    &branch,
		},
		Platform: consts.GitPlatformGithub,
		User:     domain.User{Name: pr.User.Login, AvatarURL: pr.User.AvatarURL, Email: pr.User.Email},
		Body:     pr.Body,
		Time:     time.Now(),
		Env:      map[string]string{"GITHUB_TOKEN": bot.Token},
		Bot:      bot,
	}); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to create git task")
	}
}

// --- 公共工具函数 ---

func validateHMACSHA256(secret, signature string, body []byte) error {
	if secret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func dedup(ctx context.Context, rdb *redis.Client, key string, logger *slog.Logger) bool {
	ok, err := rdb.SetNX(ctx, fmt.Sprintf("pr_review:%s", key), 1, 5*time.Minute).Result()
	if err != nil {
		logger.With("pr", key).ErrorContext(ctx, "failed to setnx pr review")
		return false
	}
	if !ok {
		logger.With("pr", key).WarnContext(ctx, "skip duplicate pr review")
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
