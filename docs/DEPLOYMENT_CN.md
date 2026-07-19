# DevLoom 部署手册

本文面向系统管理员，说明如何通过 Docker Compose 部署 DevLoom，以及如何在内网或离线网络中准备运行依赖。构建与发布请另见 [GitHub Actions 编译手册](./GITHUB_ACTIONS.md)。

## 1. 部署边界

本仓库可构建 Go 后端、React Web、Nginx 入口、桌面端和移动端。完整 AI 开发任务还依赖本仓库不提供的兼容组件：

- Taskflow 服务及 runner/host 安装包；
- preview 端口中继服务；
- Ubuntu 等开发环境 OCI 镜像；
- 离线安装器、Docker 包和项目模板；
- 部署方选择的大模型服务、Git 平台和基础设施。

隔离工作区和 Agent 接入协议见 [AI Agent 与隔离工作区集成手册](./AGENT_INTEGRATION_CN.md)。

只启动前后端可以完成账号和管理配置，但不能创建可执行的开发任务。套餐、充值、支付、邀请和签到默认关闭，不影响私有部署的项目、任务、终端和文件功能。详细边界见 [开源运行边界](./OPEN_SOURCE_BOUNDARIES.md)。

## 2. 架构与端口

`backend/docker-compose.yml` 编排 PostgreSQL、Redis、ClickHouse、RustFS、ingress、frontend、backend、Taskflow 和 preview。

| 端口 | 访问方 | 用途 |
|---|---|---|
| `80` 或 `NGINX_PORT` | 浏览器、桌面端、移动端 | Web、HTTP API、WebSocket、静态文件和对象存储代理 |
| `443` | 用户客户端 | 建议由企业反向代理终止 HTTPS 后转发到 ingress HTTP |
| `50443` | 开发主机 | ingress 到 Taskflow 的 TLS gRPC 长连接 |
| `30000-50000` | 浏览器、移动端 | preview 动态端口范围，按外部 preview 实现开放 |

外部 Taskflow 实现还可能要求 `7000`，preview 管理端可能使用 `9080`。当前 Compose 不通过 ingress 发布这两个端口，必须根据所采用运行时的文档确认后再开放。数据库、Redis、ClickHouse 和 RustFS 控制台不应暴露到公网。

## 3. 部署前准备

建议使用 Linux x86_64/arm64 主机、Docker Engine 24+、Compose v2、至少 4 核 CPU、16 GB 内存和 100 GB 可用磁盘。运行开发环境的 host 主机需要额外 CPU、内存、磁盘、Docker 和 KVM/容器能力，容量按并发任务计算。

准备以下内容：

1. 一个客户端和开发主机均可解析的域名，例如 `devloom.intra.example`。
2. PostgreSQL、Redis、ClickHouse、RustFS 及 DevLoom 三个公开镜像。
3. 与当前后端协议兼容的 Taskflow、preview、host、Workspace Adapter 和开发镜像。
4. 内网 CA 或受信任证书。`backend/build/nginx.conf` 启动时必须读取 `server.crt` 和 `server.key`。
5. 管理员邮箱、强密码，以及随机生成的数据库、缓存、对象存储和 relay 密钥。

## 4. 准备目录与静态资源

```bash
sudo mkdir -p /opt/devloom/{data,logs,static,tls,config,backup}
sudo chown -R "$USER":"$USER" /opt/devloom
git clone https://github.com/Y-vQv-Y/DevLoom.git /opt/devloom/source
cd /opt/devloom/source
```

离线 host 安装模式要求部署方将兼容文件放到以下位置：

```text
/opt/devloom/static/project-tpl.zip
/opt/devloom/static/installer/amd64/installer
/opt/devloom/static/installer/amd64/host.tgz
/opt/devloom/static/installer/amd64/docker.tgz
/opt/devloom/static/installer/arm64/installer
/opt/devloom/static/installer/arm64/host.tgz
/opt/devloom/static/installer/arm64/docker.tgz
```

这些文件不在本仓库中，缺失时“开发主机”页面生成的离线安装命令无法完成安装。

## 5. 配置 Compose

在 `/opt/devloom/.env` 创建以下配置。生产环境应固定版本或镜像 digest，不要长期使用 `latest`。

```dotenv
INSTALL_DIR=/opt/devloom
REMOTE_IP=devloom.intra.example
NGINX_PORT=80
SUBNET_PREFIX=10.100.50

POSTGRES_IMAGE=postgres:16-alpine
POSTGRES_DB=devloom
POSTGRES_USER=devloom
POSTGRES_PASSWORD=replace-with-a-long-random-password

REDIS_IMAGE=redis:7-alpine
REDIS_PASSWORD=replace-with-a-long-random-password

CLICKHOUSE_IMAGE=clickhouse/clickhouse-server:25.8
CLICKHOUSE_DB=devloom
CLICKHOUSE_USER=devloom
CLICKHOUSE_PASSWORD=replace-with-a-long-random-password

RUSTFS_IMAGE=rustfs/rustfs:latest
RUSTFS_ACCESS_KEY=replace-with-an-access-key
RUSTFS_SECRET_KEY=replace-with-a-long-random-secret

FRONTEND_IMAGE=ghcr.io/y-vqv-y/devloom-frontend:latest
BACKEND_IMAGE=ghcr.io/y-vqv-y/devloom-backend:latest
INGRESS_IMAGE=ghcr.io/y-vqv-y/devloom-ingress:latest
TASKFLOW_IMAGE=registry.intra.example/devloom/taskflow:compatible-version
PREVIEW_IMAGE=registry.intra.example/devloom/preview:compatible-version

TEAM_EMAIL=admin@example.com
TEAM_NAME=Internal Development Team
TEAM_PASSWORD=replace-with-a-strong-initial-password
# 可选：已准备好且 host 可拉取的默认开发镜像；留空时登录后在界面添加镜像
INIT_TEAM_IMAGE=
RELAY_SECRET=replace-with-at-least-32-random-bytes

# 工作区隔离策略：默认每任务独立分支，只允许工作分支创建 PR/MR
WORKSPACE_ISOLATED=true
WORKSPACE_BRANCH_PREFIX=devloom
WORKSPACE_PUSH_MODE=pull_request
WORKSPACE_OPENHANDS_WORKTREE=true
```

可用 `openssl rand -hex 32` 生成密钥。若 `10.100.50.0/24` 与现有网络冲突，修改 `SUBNET_PREFIX`。

### gRPC TLS 证书

将正式证书放到：

```text
/opt/devloom/tls/server.crt
/opt/devloom/tls/server.key
```

证书 SAN 必须包含 `REMOTE_IP` 对应域名或 IP。测试环境可生成自签名证书，但必须把签发 CA 导入所有开发主机的信任库：

```bash
openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
  -keyout /opt/devloom/tls/server.key \
  -out /opt/devloom/tls/server.crt \
  -subj "/CN=devloom.intra.example" \
  -addext "subjectAltName=DNS:devloom.intra.example"
chmod 600 /opt/devloom/tls/server.key
```

### 可选后端配置文件

普通标量配置可用 `DEVLOOM_` 前缀环境变量覆盖，例如 `server.base_url` 对应 `DEVLOOM_SERVER_BASE_URL`。GitLab 多实例等动态配置建议使用 YAML。复制示例并只保留需要的项目：

```bash
cp backend/config/server/config.yaml.example /opt/devloom/config/config.yaml
chmod 600 /opt/devloom/config/config.yaml
```

内网 GitLab 示例：

```yaml
gitlab:
  default: internal
  allowed_domains: ["gitlab.intra.example"]
  webhook_secret: "replace-with-random-secret"
  instances:
    internal:
      enabled: true
      base_url: "https://gitlab.intra.example"
      token: ""
      tls_insecure_skip_verify: false

review_agent:
  model_id: ""
  image: ""
```

创建 `/opt/devloom/compose.override.yml`：

```yaml
services:
  backend:
    environment:
      DEVLOOM_LOGGER_LEVEL: info
      DEVLOOM_SERVER_BASE_URL: ${PUBLIC_BASE_URL}
      DEVLOOM_LLM_PROXY_BASE_URL: ${PUBLIC_BASE_URL}
      DEVLOOM_OBJECT_STORAGE_ACCESS_ENDPOINT: ${PUBLIC_BASE_URL}/oss
    volumes:
      - ${CONFIG_FILE}:/app/config/server/config.yaml:ro
  taskflow:
    environment:
      DEVLOOM_TEMPLATE_PROJECT_ZIP_URL: ${PUBLIC_BASE_URL}/static/project-tpl.zip
```

并在 `.env` 追加：

```dotenv
PUBLIC_BASE_URL=https://devloom.intra.example
CONFIG_FILE=/opt/devloom/config/config.yaml
```

`server.base_url` 必须是用户和 Git webhook 可访问的地址。生产环境不要保持 `debug` 日志级别，避免敏感配置出现在日志中。

## 6. 启动与首次验收

若 GHCR 或内部镜像仓库需要认证，先执行 `docker login`。然后验证展开后的 Compose 配置并启动：

```bash
docker compose --env-file /opt/devloom/.env \
  -f backend/docker-compose.yml \
  -f /opt/devloom/compose.override.yml config

docker compose --env-file /opt/devloom/.env \
  -f backend/docker-compose.yml \
  -f /opt/devloom/compose.override.yml pull

docker compose --env-file /opt/devloom/.env \
  -f backend/docker-compose.yml \
  -f /opt/devloom/compose.override.yml up -d
```

后端每次启动都会自动执行 `backend/migration/` 中的数据库迁移。检查状态：

```bash
docker compose --env-file /opt/devloom/.env \
  -f backend/docker-compose.yml \
  -f /opt/devloom/compose.override.yml ps
docker logs --tail 200 devloom-backend
docker logs --tail 200 devloom-taskflow
curl -I http://devloom.intra.example/
curl -i http://devloom.intra.example/api/v1/users/info
```

首页返回 `200`，未登录 API 返回业务错误或 `401` 均表示 HTTP 链路已到达后端。当前后端没有独立 `/healthz` 路由，不要用不存在的路径判断服务状态。

如果前面已配置 HTTPS 反向代理，将上面两个 URL 换成 `https://devloom.intra.example/` 和 `https://devloom.intra.example/api/v1/users/info`，并确认代理保留 WebSocket、`Host`、`X-Forwarded-*` 请求头。

使用 `TEAM_EMAIL` 和 `TEAM_PASSWORD` 登录。初始化是幂等的，已有邮箱不会因修改环境变量而重置密码。首次登录后立即修改初始密码，并按 [用户操作手册](./USER_GUIDE_CN.md) 完成模型、镜像和主机配置。

## 7. Web、桌面与移动端连接

- Web 浏览器访问 `PUBLIC_BASE_URL`。
- Electron 桌面端连接同一站点，构建时通过相应发行工作流生成安装包。
- Android/iOS 的服务地址由登录页“配置服务地址”填写，也可通过 GitHub Actions 变量 `DEVLOOM_MOBILE_API_URL` 预置。
- 移动端不使用 Expo/EAS 云构建，不需要 `EXPO_TOKEN` 或 `EXPO_PROJECT_ID`。iPhone 真机 IPA 仍必须具有 Apple 证书和描述文件。

前端 `VITE_*` 和移动端 `EXPO_PUBLIC_*` 为编译期配置，不能通过给已构建容器追加环境变量来修改。变更品牌链接、功能开关、API 默认地址或移动端包名后，应重新运行 GitHub Actions。完整变量、签名和 Release 制品说明见 [GitHub Actions 编译手册](./GITHUB_ACTIONS.md)。

## 8. 内网与完全离线部署

内网部署支持浏览器访问、内部 GitLab、内部模型服务和内部开发主机。若启用 Eclipse Che/Coder 或 OpenHands，还必须将其部署到同一受控网络并使用独立工作区。完全离线部署还必须完成以下工作：

1. 将全部基础镜像、DevLoom 镜像、Taskflow、preview 和开发镜像同步到内网仓库，或使用 `docker save`/`docker load` 导入。
2. 提供 `project-tpl.zip`、host、Docker 和安装器离线包。
3. 将模型 `base_url` 指向内网 OpenAI 兼容、Anthropic 或其他受支持服务。
4. 使用内网 DNS、NTP、CA、SMTP 和 GitLab，禁用不需要的公网 OAuth、更新和社区链接。
5. 确保后端、Taskflow、preview、开发主机、模型和 GitLab 之间路由可达。

“源码可离线部署”不等于“本仓库包含全部 AI 运行时”。缺少外部 Taskflow/runner/preview 时，管理面可用，但任务执行、终端、文件和预览不可用。

## 9. 安全基线

- 对外仅开放 HTTPS、`50443` 和确有需要的预览端口，使用防火墙限制来源。
- 不提交 `.env`、`config.yaml`、Token、证书、keystore、`.p12` 或 provisioning profile。
- 内网 GitLab 应安装企业 CA；`tls_insecure_skip_verify: true` 只用于临时排障。
- Web 与 API 使用同源部署。仅在明确理解风险时设置 `DEVLOOM_WS_INSECURE_SKIP_VERIFY=true`。
- Git PAT 使用最小权限，模型 API Key 由部署方控制并定期轮换。
- RustFS、PostgreSQL、Redis 和 ClickHouse 使用独立强密码，备份文件加密并限制权限。
- 公开分发前检查 AGPL-3.0、tldraw 和所用第三方模型、镜像及 SDK 的许可证。

## 10. 备份、升级与回滚

数据库备份：

```bash
mkdir -p /opt/devloom/backup/$(date +%F)
docker exec devloom-db sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  > /opt/devloom/backup/$(date +%F)/postgres.sql
```

完整备份还应包含 `/opt/devloom/data/redis`、`clickhouse`、`rustfs`、`/opt/devloom/static`、`.env`、配置和证书。对 bind mount 做一致性快照前先停止写入，或使用各数据库的在线备份机制。

升级步骤：

1. 记录当前所有镜像 digest，阅读 Release 说明并完成备份。
2. 在 `.env` 中固定新版本标签，执行 `docker compose pull`。
3. 先在预发布环境验证迁移，再执行 `docker compose up -d`。
4. 检查后端迁移日志、登录、模型健康检查、主机在线、任务、终端和预览。

回滚应用时恢复旧镜像标签。由于后端启动会自动迁移数据库，若新版本迁移不向后兼容，必须同时恢复升级前 PostgreSQL 备份，不能只回滚容器。

恢复 PostgreSQL 示例：

```bash
docker exec -i devloom-db sh -c 'psql -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  < /opt/devloom/backup/2026-07-19/postgres.sql
```

## 11. 常见故障

| 现象 | 检查项 |
|---|---|
| 后端立即退出 | `TASKFLOW_SERVER` 必须是绝对 `http://` 或 `https://` URL；检查 PostgreSQL、Redis 和迁移日志 |
| ingress 反复重启 | `/opt/devloom/tls/server.crt`、`server.key` 是否存在且可读 |
| 可登录但不能创建任务 | 是否存在在线私有主机、可用模型、开发镜像和兼容 Taskflow/runner |
| host 安装失败 | `/static/installer/<arch>/` 文件是否齐全，架构、CA、DNS 和 Docker 是否正确 |
| 模型检查失败 | `base_url`、接口类型、模型名、API Key、代理和内网 DNS |
| 上传或头像失败 | RustFS 健康状态、凭据、bucket 初始化和 `/oss` 代理地址 |
| WebSocket 断开 | 反向代理 Upgrade/Connection、超时、同源 Origin 和 TLS |
| GitLab 拉取失败 | PAT 权限、仓库 URL、CA、`allowed_domains` 和实例 `base_url` |
| `@devloom` 无响应 | 自动审查开关、webhook、`review_agent`、在线主机和任务资源是否完整 |
| 移动任务被跳过 | Release 工作流需启用 `mobile=true` 或仓库变量 `ENABLE_MOBILE_RELEASE=true` |
| Actions 要求 `EXPO_TOKEN` | 使用了旧工作流；当前原生 Gradle/Xcode 工作流不依赖 Expo/EAS 云凭据 |

排障时优先收集 `docker compose ps`、backend/Taskflow/preview/ingress 日志、失败请求状态码和主机状态，不要在工单中附带 Token、密码或完整配置文件。
