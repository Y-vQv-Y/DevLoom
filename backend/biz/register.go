package biz

import (
	"context"

	"github.com/google/uuid"
	"github.com/samber/do"

	"github.com/Y-vQv-Y/DevLoom/backend/biz/agentresource"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/file"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/git"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/host"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/llmproxy"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/mcphub"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/notify"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/plugin"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/project"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/public"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/server"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/setting"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/skill"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/static"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/subscription"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/task"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/team"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/uploader"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/user"
	"github.com/Y-vQv-Y/DevLoom/backend/biz/vmidle"
	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/domain"
)

// RegisterAll 注册所有 biz 模块
// 分两阶段：先 Provide（懒注册），再 Invoke（解析依赖），避免模块间循环依赖
func RegisterAll(i *do.Injector) error {
	notify.ProvideNotify(i)
	public.ProvidePublic(i)
	user.ProvideUser(i)
	setting.ProvideSetting(i)
	team.ProvideTeam(i)
	host.ProvideHost(i)
	agentresource.ProvideAgentResource(i)
	task.ProvideTask(i)
	git.ProvideGit(i)
	project.ProvideProject(i)
	file.ProvideFile(i)
	vmidle.ProvideVMIdle(i)
	skill.ProvideSkill(i)
	plugin.ProvidePlugin(i)
	server.ProvideServer(i)
	return nil
}

func InvokeAll(i *do.Injector) {
	notify.InvokeNotify(i)
	public.InvokePublic(i)
	user.InvokeUser(i)
	setting.InvokeSetting(i)
	team.InvokeTeam(i)
	host.InvokeHost(i)
	task.InvokeTask(i)
	git.InvokeGit(i)
	project.InvokeProject(i)
	file.InvokeFile(i)
	vmidle.InvokeVMIdle(i)
	skill.InvokeSkill(i)
	plugin.InvokePlugin(i)
	server.InvokeServer(i)
}

// RegisterOpenSource 注册仅在开源项目中使用的模块
func RegisterOpenSource(i *do.Injector) {
	subscription.ProvideSubscription(i)
	uploader.ProvideUploader(i)
	llmproxy.ProvideLLMProxy(i)
	mcphub.ProvideMCPHub(i)
	static.ProviderStatic(i)
	do.ProvideValue[domain.TaskHook](i, &taskhook{})
}

func InvokeOpenSource(i *do.Injector) {
	subscription.InvokeSubscription(i)
	uploader.InvokeUploader(i)
	llmproxy.InvokeLLMProxy(i)
	mcphub.InvokeMCPHub(i)
	static.InvokeStatic(i)
}

type taskhook struct{}

// GetMaxConcurrent implements [domain.TaskHook].
func (t *taskhook) GetMaxConcurrent(ctx context.Context, uid uuid.UUID) (int, error) {
	return 3, nil
}

// GetSystemPrompt implements [domain.TaskHook].
func (t *taskhook) GetSystemPrompt(ctx context.Context, taskType consts.TaskType, subType consts.TaskSubType) (string, error) {
	switch subType {
	case consts.TaskSubTypeGenerateRequirement:
		return "Act as a senior product engineer. Produce a testable requirement specification with context, goals, non-goals, user stories, functional and non-functional requirements, acceptance criteria, constraints, risks, and open questions. Do not modify source files.", nil
	case consts.TaskSubTypeGenerateDesign:
		return "Act as a senior software architect. Read the repository and requirement, then produce an implementation design covering architecture, affected modules, data and API changes, security, migrations, testing, rollout, and rollback. Do not implement the design or modify source files.", nil
	case consts.TaskSubTypeGenerateTasklist:
		return "Turn the approved requirement and design into an ordered implementation task list. Include dependencies, expected files, tests, acceptance checks, risks, and rollback work for every task. Do not modify source files.", nil
	case consts.TaskSubTypeExecuteTask:
		return "Implement only the assigned task from the approved plan. Inspect existing conventions, make focused changes in the isolated work branch, run relevant tests, and summarize changes, verification, and remaining risks. Never merge or modify the protected base branch.", nil
	case consts.TaskSubTypeGenerateDocs:
		return "Create concise project documentation from the current repository state. Verify commands and paths against source, identify external runtime requirements, and do not claim unverified functionality.", nil
	case consts.TaskSubTypePrReview:
		return "Review the pull or merge request for correctness, security, regressions, and missing tests. Lead with actionable findings and file references. Do not merge or modify the protected base branch.", nil
	}

	switch taskType {
	case consts.TaskTypeDesign:
		return "Analyze the requirement and repository, then produce a concrete technical design with interfaces, data flow, risks, tests, rollout, and rollback. Do not modify source files unless explicitly asked.", nil
	case consts.TaskTypeDevelop:
		return "Follow the approved requirement and design. Inspect the repository, plan the change, implement it in the isolated work branch, run focused tests, and summarize verification and residual risks. Never merge or modify the protected base branch.", nil
	case consts.TaskTypeReview:
		return "Review the requested change and report actionable correctness, security, regression, and testing findings. Do not modify or merge the protected base branch unless the user explicitly requests a fix.", nil
	default:
		return "", nil
	}
}

// GitTask implements [domain.TaskHook].
func (t *taskhook) GitTask(ctx context.Context, id uuid.UUID) (*domain.GitTask, error) {
	return &domain.GitTask{}, nil
}

// OnTaskCreated implements [domain.TaskHook].
func (t *taskhook) OnTaskCreated(ctx context.Context, task *domain.ProjectTask) error {
	return nil
}

var _ domain.TaskHook = &taskhook{}
