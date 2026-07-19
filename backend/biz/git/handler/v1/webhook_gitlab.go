package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/GoYoko/web"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"

	"github.com/Y-vQv-Y/DevLoom/backend/config"
	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/gitreview"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

// GitlabWebhookHandler GitLab Webhook 处理器
type GitlabWebhookHandler struct {
	cfg            *config.Config
	logger         *slog.Logger
	redis          *redis.Client
	gitbotUsecase  domain.GitBotUsecase
	gitTaskUsecase domain.GitTaskUsecase
}

// NewGitlabWebhookHandler 创建 GitLab Webhook 处理器
func NewGitlabWebhookHandler(i *do.Injector) (*GitlabWebhookHandler, error) {
	h := &GitlabWebhookHandler{
		cfg:            do.MustInvoke[*config.Config](i),
		logger:         do.MustInvoke[*slog.Logger](i).With("module", "GitlabWebhookHandler"),
		redis:          do.MustInvoke[*redis.Client](i),
		gitbotUsecase:  do.MustInvoke[domain.GitBotUsecase](i),
		gitTaskUsecase: do.MustInvoke[domain.GitTaskUsecase](i),
	}

	w := do.MustInvoke[*web.Web](i)
	w.Group("/api/v1").POST("/gitlab/webhook/:id", web.BaseHandler(h.Webhook))

	return h, nil
}

// Webhook 处理 GitLab Webhook 请求
func (h *GitlabWebhookHandler) Webhook(c *web.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "invalid id")
	}

	bot, err := h.gitbotUsecase.GetByID(ctx, id)
	if err != nil {
		return c.String(http.StatusNotFound, "bot not found")
	}

	// GitLab 使用 X-Gitlab-Token 验证
	token := c.Request().Header.Get("X-Gitlab-Token")
	if bot.SecretToken != "" && token != bot.SecretToken {
		return c.String(http.StatusUnauthorized, "invalid token")
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}

	event := strings.ToLower(strings.TrimSpace(c.Request().Header.Get("X-Gitlab-Event")))
	switch event {
	case "merge request hook":
		h.handleMergeRequest(ctx, bot, body)
	case "note hook":
		h.handleNote(ctx, bot, body)
	}

	return c.String(http.StatusOK, "ok")
}

func (h *GitlabWebhookHandler) handleMergeRequest(ctx context.Context, bot *domain.GitBot, payload []byte) {
	var ev struct {
		ObjectAttributes *struct {
			IID          int    `json:"iid"`
			Title        string `json:"title"`
			Description  string `json:"description"`
			State        string `json:"state"`
			Action       string `json:"action"`
			URL          string `json:"url"`
			SourceBranch string `json:"source_branch"`
		} `json:"object_attributes"`
		Project *struct {
			ID                int    `json:"id"`
			Name              string `json:"name"`
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
			GitHTTPURL         string `json:"git_http_url"`
			Description       string `json:"description"`
			Visibility        string `json:"visibility"`
			VisibilityLevel   any    `json:"visibility_level"`
		} `json:"project"`
		User *struct {
			Username  string `json:"username"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to unmarshal gitlab mr event")
		return
	}

	mr := ev.ObjectAttributes
	proj := ev.Project
	user := ev.User
	if mr == nil || proj == nil || user == nil {
		return
	}

	if mr.State != "open" && mr.State != "opened" && mr.State != "reopened" {
		return
	}
	switch mr.Action {
	case "open", "reopen", "update":
	default:
		return
	}

	if !dedup(ctx, h.redis, mr.URL, h.logger) {
		return
	}
	hostID, err := webhookRuntime(bot)
	if err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "gitlab webhook runtime is not configured")
		return
	}

	branch := mr.SourceBranch
	isPrivate := gitlabProjectIsPrivate(proj.Visibility, proj.VisibilityLevel)
	if _, err := h.gitTaskUsecase.Create(ctx, domain.CreateGitTaskReq{
		HostID:  hostID,
		Prompt:  mr.URL,
		Git:     taskflow.Git{Token: bot.Token},
		Subject: domain.Subject{
			ID: fmt.Sprintf("%d", mr.IID), Type: "MergeRequest",
			Title: mr.Title, URL: mr.URL, Number: mr.IID,
		},
		Repo: domain.Repo{
			ID: fmt.Sprintf("%d", proj.ID), Name: proj.Name,
			FullName: proj.PathWithNamespace, URL: firstNonEmpty(proj.GitHTTPURL, proj.WebURL),
			Desc: proj.Description, IsPrivate: isPrivate, Branch: &branch,
		},
		Platform: consts.GitPlatformGitLab,
		User:     domain.User{Name: firstNonEmpty(user.Name, user.Username), AvatarURL: user.AvatarURL, Email: user.Email},
		Body:     mr.Description,
		Time:     time.Now(),
		Env:      map[string]string{"GITLAB_TOKEN": bot.Token},
		Bot:      bot,
	}); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to create gitlab merge request task")
	}
}

type gitlabNoteEvent struct {
	ObjectAttributes *struct {
		ID           int    `json:"id"`
		Note         string `json:"note"`
		NoteableType string `json:"noteable_type"`
		URL          string `json:"url"`
	} `json:"object_attributes"`
	MergeRequest *struct {
		ID           int    `json:"id"`
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		State        string `json:"state"`
		URL          string `json:"url"`
		SourceBranch string `json:"source_branch"`
	} `json:"merge_request"`
	Project *struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
		GitHTTPURL         string `json:"git_http_url"`
		Description       string `json:"description"`
		Visibility        string `json:"visibility"`
		VisibilityLevel   any    `json:"visibility_level"`
	} `json:"project"`
	User *struct {
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
}

func (h *GitlabWebhookHandler) handleNote(ctx context.Context, bot *domain.GitBot, payload []byte) {
	req, dedupKey, ok, err := buildGitlabNoteTask(bot, h.cfg, payload)
	if err != nil {
		h.logger.With("error", err).WarnContext(ctx, "invalid gitlab note webhook")
		return
	}
	if !ok || !dedup(ctx, h.redis, dedupKey, h.logger) {
		return
	}
	if _, err := h.gitTaskUsecase.Create(ctx, req); err != nil {
		h.logger.With("error", err).ErrorContext(ctx, "failed to create gitlab note task")
	}
}

func buildGitlabNoteTask(bot *domain.GitBot, cfg *config.Config, payload []byte) (domain.CreateGitTaskReq, string, bool, error) {
	var ev gitlabNoteEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return domain.CreateGitTaskReq{}, "", false, fmt.Errorf("decode note payload: %w", err)
	}
	if ev.ObjectAttributes == nil || ev.MergeRequest == nil || ev.Project == nil || ev.User == nil {
		return domain.CreateGitTaskReq{}, "", false, nil
	}
	note := ev.ObjectAttributes
	mr := ev.MergeRequest
	if !strings.EqualFold(note.NoteableType, "MergeRequest") ||
		(mr.State != "open" && mr.State != "opened" && mr.State != "reopened") {
		return domain.CreateGitTaskReq{}, "", false, nil
	}
	keyword := strings.TrimSpace(cfg.Task.AtKeyword)
	command, prompt, mentioned := gitreview.ParseMention(note.Note, keyword)
	if !mentioned {
		return domain.CreateGitTaskReq{}, "", false, nil
	}

	hostID, err := webhookRuntime(bot)
	if err != nil {
		return domain.CreateGitTaskReq{}, "", false, err
	}
	mrURL := strings.TrimSpace(mr.URL)
	if mrURL == "" && ev.Project.WebURL != "" && mr.IID > 0 {
		mrURL = fmt.Sprintf("%s/-/merge_requests/%d", strings.TrimSuffix(ev.Project.WebURL, "/"), mr.IID)
	}
	branch := mr.SourceBranch
	subjectID := mr.ID
	if subjectID == 0 {
		subjectID = mr.IID
	}
	dedupKey := fmt.Sprintf("gitlab:note:%d:%d", ev.Project.ID, note.ID)

	return domain.CreateGitTaskReq{
		HostID:  hostID,
		Prompt:  prompt,
		Command: string(command),
		Git:     taskflow.Git{Token: bot.Token},
		Subject: domain.Subject{
			ID: fmt.Sprintf("%d", subjectID), Type: "MergeRequest",
			Title: mr.Title, URL: mrURL, Number: mr.IID,
		},
		Repo: domain.Repo{
			ID: fmt.Sprintf("%d", ev.Project.ID), Name: ev.Project.Name,
			FullName: ev.Project.PathWithNamespace,
			URL: firstNonEmpty(ev.Project.GitHTTPURL, ev.Project.WebURL),
			Desc: ev.Project.Description,
			IsPrivate: gitlabProjectIsPrivate(ev.Project.Visibility, ev.Project.VisibilityLevel), Branch: &branch,
		},
		Platform: consts.GitPlatformGitLab,
		User: domain.User{
			Name: firstNonEmpty(ev.User.Name, ev.User.Username),
			AvatarURL: ev.User.AvatarURL, Email: ev.User.Email,
		},
		Body: note.Note,
		Time: time.Now(),
		Env: map[string]string{
			"GITLAB_TOKEN":           bot.Token,
			"DEVLOOM_REVIEW_COMMAND": string(command),
		},
		Bot:  bot,
	}, dedupKey, true, nil
}

func gitlabProjectIsPrivate(visibility string, visibilityLevel any) bool {
	if strings.EqualFold(strings.TrimSpace(visibility), "private") {
		return true
	}
	switch value := visibilityLevel.(type) {
	case float64:
		return value == 0
	case string:
		value = strings.TrimSpace(value)
		return value == "0" || strings.EqualFold(value, "private")
	default:
		return false
	}
}
