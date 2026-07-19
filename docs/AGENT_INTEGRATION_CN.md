# AI Agent 与隔离工作区集成手册

本文说明 DevLoom 如何接入内网 AI Agent 和云开发环境。当前仓库负责账号、项目、任务、Git Webhook、任务状态和 WebSocket；开发环境与 Agent Server 由部署方提供。

## 推荐架构

```text
浏览器 / 移动端 -> DevLoom API -> Workspace Adapter
                                      |
                               Eclipse Che / Coder
                                      |
                           独立 Pod + PVC + Git clone
                                      |
                         OpenHands Agent Server / Taskflow Agent
                                      |
                              内网模型 + 内网 GitLab
```

Eclipse Che 适合 Kubernetes 多租户内网部署，Coder 适合快速搭建模板化工作区。OpenHands SDK 的 Agent Server 提供 REST 和 WebSocket 接口，并支持 Docker/Kubernetes 临时工作区。不要把 Agent Server 直接暴露到公网，必须通过 DevLoom 登录、内部网络和每任务 API Key 访问。

## 工作区隔离契约

DevLoom 默认向兼容 Taskflow 的 VM 创建和任务创建请求发送：

```json
{
  "workspace": {
    "isolated": true,
    "base_branch": "main",
    "work_branch": "devloom/task/任务UUID",
    "push_mode": "pull_request",
    "protected_branch": true,
    "openhands_worktree": true
  }
}
```

运行时必须先 clone `base_branch`，再创建 `work_branch`。任务只能修改工作分支，禁止直接推送、强制推送、合并或删除保护分支。OpenHands Agent Server 创建会话时应把 `worktree` 设为 `true`，并将每个用户或任务映射到独立的 `working_dir`。完成后由工作分支创建 PR/MR，审查和 CI 通过后再人工合并。

配置入口为 `backend/config/server/config.yaml.example` 的 `workspace` 节。兼容旧运行时的环境变量也会传递：`DEVLOOM_WORKSPACE_BASE_BRANCH`、`DEVLOOM_WORKSPACE_BRANCH`、`DEVLOOM_WORKSPACE_PUSH_MODE` 和 `OPENHANDS_WORKTREE`。

## Git 审查命令

为仓库 Webhook 配置 `@devloom` 触发词。GitHub 使用 `pull_request` 和 `issue_comment` 事件，GitLab 使用 Merge Request 和 Note Hook。支持：

```text
@devloom review
@devloom fix 修复失败测试
@devloom explain 解释鉴权流程
@devloom plan 给出实现计划
```

普通 PR/MR 打开或更新时仍会自动触发审查。评论任务会读取 PR/MR 源分支，创建独立任务分支，并把命令和评论内容传给 Agent。Agent 输出只应作为审查建议；`fix` 可以提交工作分支，但不能自动合并。

## 需求到交付流程

推荐把项目 Issue 的需求文档、设计文档和任务拆分映射到 [GitHub Spec Kit](https://github.com/github/spec-kit) 的 `specify -> plan -> tasks -> implement -> converge` 流程。每一步都在独立工作区执行，并通过 DevLoom 任务记录、测试结果和 PR 关联起来。发布仍必须经过 CI、人工审批和部署系统，不允许 Agent 直接发布生产环境。

## 安全检查

- 每个工作区使用独立 PVC、服务账号、网络策略和资源配额。
- Git Token 只授予目标仓库最小权限，默认禁止删除和合并。
- OpenHands 必须设置 session API key、`OH_SECRET_KEY` 和受限 CORS。
- 内网模型、GitLab、镜像仓库和开发镜像必须使用企业 CA。
- 固定 Agent、运行时和开发镜像版本，并保存许可证清单。

没有兼容 Taskflow/Workspace Adapter 时，DevLoom 管理面可以启动，但任务执行、终端、文件、预览和 Agent 会话不会自动获得完整能力。
