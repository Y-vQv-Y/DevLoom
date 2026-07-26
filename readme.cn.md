# ADTEC DevLoom

<p align="center">
  <img src="./frontend/public/logo-brand.png" alt="ADTEC DevLoom" width="260" />
</p>

ADTEC DevLoom 是面向企业内网的软件研发平台，统一管理代码仓库、项目、模型、AI
任务、隔离开发环境、远程终端、文件与差异以及在线预览。完整 Linux 运行时均由本仓库
源码构建，包括 Web、Go API、ingress、Taskflow、preview relay、远程
Orchestrator/Runner、devbox、中心安装器和开发主机安装器。

构建链路不读取也不执行任何外部应用离线包。PostgreSQL、Redis、ClickHouse、
RustFS、Docker Engine 和 Docker Compose 是固定版本的第三方基础设施依赖，会随自有
离线包一起交付。

## 完整功能面

- 密码账号、团队、成员、分组、权限策略与审计记录；
- Git 身份、仓库、项目、Issue、合并请求和 Webhook；
- OpenAI 兼容与 Anthropic 兼容模型服务；
- 基于 Docker 隔离工作区的 AI 开发任务与完整生命周期控制；
- 中心本机执行和带认证的远程 Runner；
- 浏览器终端、工作区文件、上传下载、仓库 diff 和端口管理；
- 通过自带 preview relay 访问任务中运行的 Web 应用；
- 技能、MCP、通知、开发镜像、环境变量和资源限制；
- React Web、Electron 桌面端和 Expo 移动端源码。

套餐计费、支付、邀请奖励、社区发布、Apple 登录、Git OAuth 快捷绑定和企业许可证
扩展默认关闭，因为这些入口需要另行运营的外部服务；关闭后不影响上述内网研发功能。

## 目录结构

| 目录 | 内容 |
|---|---|
| `backend/` | Go API、迁移、Taskflow、Orchestrator、preview 与 E2E 模型 |
| `frontend/` | Vite + React Web 控制台 |
| `desktop/` | Electron 桌面封装 |
| `mobile/` | Expo/React Native 客户端与自建更新服务 |
| `devbox/` | 从源码构建的 Linux 开发镜像 |
| `deploy/` | Compose、E2E、VMware、离线构建、安装与验收脚本 |
| `docs/` | 当前有效的部署、使用、集成和发布文档 |

## 一键离线部署

在 Linux amd64 构建机上安装 Docker Buildx，并在忽略提交的配置中固定四个基础镜像
与 Docker 静态包校验值：

```bash
cp deploy/package/package.env.example deploy/package/package.env
bash deploy/package/build.sh --version v1.0.0
```

输出为：

```text
deploy/out/devloom-offline-linux-amd64.tgz
deploy/out/devloom-offline-linux-amd64.tgz.sha256
```

在无公网的 Linux amd64 服务器安装：

```bash
sha256sum -c devloom-offline-linux-amd64.tgz.sha256
tar -xzf devloom-offline-linux-amd64.tgz
cd devloom-offline-linux-amd64
sudo bash install.sh --host devloom.intra.example --admin-email admin@intra.example
```

安装器会校验清单、按需安装包内 Docker、导入全部镜像、生成随机密钥和 TLS 材料、启动
Compose 并执行部署验收。登录后至少配置一个模型；远程开发主机使用系统从
`/static/installer/<arch>/` 提供的可读安装脚本注册。

完整说明见[离线部署手册](./docs/DEPLOYMENT_OFFLINE_CN.md)和
[离线包构建手册](./docs/OFFLINE_PACKAGE_BUILD_CN.md)。

## 源码验证

```bash
(cd backend && go test ./...)
pnpm --dir frontend test
pnpm --dir frontend lint
pnpm --dir frontend build:online
bash -n deploy/offline/install.sh deploy/offline/preflight.sh deploy/offline/verify.sh
```

`deploy/e2e/run-linux.sh <VM_IP>` 会在 Linux Docker 主机从源码构建并启动完整平台；
`deploy/vmware/Invoke-DevLoomE2E.ps1` 驱动仓库自带的 VMware 测试环境。外部模型不可用
时，`backend/cmd/e2ellm` 的确定性企业验收模型可生成带持久化 API、自动测试和预览的
全栈项目，用于离线回归。

## 文档

- [部署手册](./docs/DEPLOYMENT_CN.md)
- [离线部署手册](./docs/DEPLOYMENT_OFFLINE_CN.md)
- [用户操作手册](./docs/USER_GUIDE_CN.md)
- [Agent 与隔离工作区](./docs/AGENT_INTEGRATION_CN.md)
- [企业全链路验收需求](./docs/ENTERPRISE_E2E_ACCEPTANCE_CN.md)
- [运行边界](./docs/OPEN_SOURCE_BOUNDARIES.md)
- [品牌资源](./docs/BRANDING.md)
- [构建与发布](./docs/GITHUB_ACTIONS.md)

## 安全

禁止提交模型密钥、Git Token、数据库密码、Runner 凭据、TLS 私钥、签名材料和
`deploy/package/package.env`。生产环境使用随机密钥和内网 CA，不向外暴露数据库、缓存
和对象存储端口，只开放 Web、必要的 Runner 与 preview 路由。

## 许可证

源码使用 [GNU AGPL-3.0](./LICENSE)。重新分发离线包前还需分别审核第三方基础镜像和
前端组件的许可证。
