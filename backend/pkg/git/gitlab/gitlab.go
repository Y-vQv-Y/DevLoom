// Package gitlab 提供 GitLab 客户端功能，支持多个 GitLab 实例
package gitlab

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Y-vQv-Y/DevLoom/backend/domain"
)

// Gitlab 客户端
type Gitlab struct {
	*gitlab.Client

	logger                  *slog.Logger
	baseURL                 string
	token                   string
	tlsInsecureSkipVerify   bool
}

// Option configures a GitLab client.
type Option func(*Gitlab)

// WithTLSInsecureSkipVerify allows an explicitly configured private GitLab
// instance with an untrusted certificate. Prefer installing the internal CA.
func WithTLSInsecureSkipVerify(enabled bool) Option {
	return func(g *Gitlab) {
		g.tlsInsecureSkipVerify = enabled
	}
}

// NewGitlab 根据 baseURL 和 token 创建 GitLab 客户端
func NewGitlab(baseURL, token string, logger *slog.Logger, opts ...Option) *Gitlab {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		return nil
	}
	g := &Gitlab{
		logger:  logger.With("module", "gitlab"),
		baseURL: baseURL,
		token:   token,
	}
	for _, opt := range opts {
		opt(g)
	}
	gitlabClient, err := g.newClientWithToken(token, false)
	if err != nil {
		logger.Error("Failed to create GitLab client", "error", err, "base_url", baseURL)
		return nil
	}
	g.Client = gitlabClient
	return g
}

// NewGitlabForBaseURL 根据任意 base_url 创建 GitLab 客户端（无默认 token）
func NewGitlabForBaseURL(baseURL string, logger *slog.Logger, opts ...Option) *Gitlab {
	return NewGitlab(baseURL, "", logger, opts...)
}

// BaseURL 返回 GitLab base URL
func (g *Gitlab) BaseURL() string {
	return g.baseURL
}

// Token 返回 GitLab token
func (g *Gitlab) Token() string {
	return g.token
}

// newClientWithToken 使用指定 token 和当前实例 baseURL 创建 GitLab 客户端
func (g *Gitlab) newClientWithToken(token string, isOAuth bool) (*gitlab.Client, error) {
	c := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: g.tlsInsecureSkipVerify, // #nosec G402 -- explicit per-instance escape hatch
			},
		},
	}
	if isOAuth {
		return gitlab.NewOAuthClient(token, gitlab.WithBaseURL(g.baseURL), gitlab.WithHTTPClient(c))
	}
	return gitlab.NewClient(token, gitlab.WithBaseURL(g.baseURL), gitlab.WithHTTPClient(c))
}

// GetRepoInfoByPAT 根据 PAT 获取仓库信息
func (g *Gitlab) GetRepoInfoByPAT(ctx context.Context, token string, repoURL string) (*gitlab.Project, error) {
	projectPath, err := ParseProjectPath(repoURL)
	if err != nil {
		return nil, err
	}
	client, err := g.newClientWithToken(token, false)
	if err != nil {
		return nil, err
	}
	project, _, err := client.Projects.GetProject(projectPath, &gitlab.GetProjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

// CheckPAT 校验 PAT
func (g *Gitlab) CheckPAT(ctx context.Context, token string, repoURL string) (bool, *domain.BindRepository, error) {
	repository, err := g.GetRepoInfoByPAT(ctx, token, repoURL)
	if err != nil {
		return false, nil, err
	}
	if repository == nil {
		return false, nil, fmt.Errorf("repository not found")
	}

	var level int
	if repository.Permissions != nil {
		if repository.Permissions.ProjectAccess != nil {
			level = int(repository.Permissions.ProjectAccess.AccessLevel)
		} else if repository.Permissions.GroupAccess != nil {
			level = int(repository.Permissions.GroupAccess.AccessLevel)
		}
	}

	if level >= 0 {
		bindRepo := &domain.BindRepository{
			RepoID:          fmt.Sprintf("%d", repository.ID),
			RepoName:        repository.Name,
			FullName:        repository.PathWithNamespace,
			RepoURL:         repository.WebURL,
			RepoDescription: repository.Description,
			IsPrivate:       repository.Visibility == "private",
			Platform:        "gitlab",
		}
		return true, bindRepo, nil
	}
	return false, nil, fmt.Errorf("token has no access to this repository")
}

// GetUserInfoByPAT 根据 PAT 获取用户信息
func (g *Gitlab) GetUserInfoByPAT(ctx context.Context, token string) (*domain.PlatformUserInfo, error) {
	client, err := g.newClientWithToken(token, false)
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}
	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("get gitlab user: %w", err)
	}
	return &domain.PlatformUserInfo{
		Name: user.Username,
	}, nil
}
