# DevLoom 内网离线部署

本文说明如何使用本仓库生成的自有离线包，让内网服务器具备完整的 Web、API、任务执行、终端、文件和预览能力。Taskflow、远程 Orchestrator、preview、devbox、前后端和安装脚本均从当前仓库构建；构建输入只有当前源码和显式固定的第三方基础设施制品。

## 1. 准备离线介质

在 Linux amd64 构建机生成自有离线包：

```text
bash deploy/package/build.sh --version v1.0.0
```

构建结果包含以下自有文件：

```text
/opt/devloom/static/project-tpl.zip
/opt/devloom/static/installer/<arch>/installer
/opt/devloom/static/installer/<arch>/host.tgz
/opt/devloom/static/installer/<arch>/docker.tgz
```

只有 PostgreSQL、Redis、ClickHouse、RustFS 是固定版本的第三方基础设施镜像；frontend、backend、ingress、Taskflow、preview、Orchestrator 和 devbox 均由构建脚本从本仓库源码生成并导出。

## 2. 一键安装

将 TGZ 和校验文件复制到内网 Linux amd64 服务器，以 root 执行可读安装脚本：

```bash
sha256sum -c devloom-offline-linux-amd64.tgz.sha256
tar -xzf devloom-offline-linux-amd64.tgz
cd devloom-offline-linux-amd64
sudo bash install.sh --host devloom.intra.example --admin-email admin@intra.example
```

安装和主机注册阶段不访问外部镜像仓库；Docker Engine、Compose、中心镜像、host 镜像、项目模板和校验清单均在包内。

## 3. 配置与启动前检查

```bash
sudo cp deploy/offline/.env.example /opt/devloom/.env
sudo chmod 600 /opt/devloom/.env
sudo vi /opt/devloom/.env
```

将所有 `CHANGE_ME`、`.example`、`RELEASE_TAG` 替换为真实值，`REMOTE_IP` 使用内网 DNS 名称或 IP，所有镜像使用固定 tag/digest。将受信任证书放置到：

```text
/opt/devloom/tls/server.crt
/opt/devloom/tls/server.key
```

然后执行严格检查。检查失败时不要启动服务：

```bash
sudo bash deploy/offline/preflight.sh --root /opt/devloom
```

检查内容包括 Docker Compose、变量和强密码、镜像占位符、TLS 证书、`project-tpl.zip`、架构匹配的 host 安装资源，以及 `docker compose config`。`--skip-docker` 仅用于在没有 Docker 的介质准备机上检查文件，不能作为服务器上线检查。

## 4. 启动和验收

安装器会自动启动服务；需要人工恢复时使用安装目录中的 Compose 文件：

```bash
cd /opt/devloom/source
docker compose --env-file /opt/devloom/.env \
  -f backend/docker-compose.yml \
  -f /opt/devloom/compose.override.yml up -d
```

启动后执行：

```bash
sudo bash deploy/offline/verify.sh --root /opt/devloom
```

验收脚本等待全部服务容器运行，检查 ingress 首页和 `/api/v1/users/info` 是否可达，并输出 backend、Taskflow、preview 的最近日志。未登录 API 返回 `401/403` 或业务错误均表示 HTTP 链路已到达后端；容器运行不代表任务功能可用，必须确认 Taskflow、preview 和 host 安装资源均通过检查。

## 5. 内网限制与安全

- 内网 DNS、CA、NTP、SMTP、GitLab 和模型 API 必须可从 backend、Taskflow、preview 及开发主机访问。
- 防火墙只开放 Web 端口、Taskflow TLS 端口 `50443` 和实际需要的 preview 端口范围；不要暴露 PostgreSQL、Redis、ClickHouse 或 RustFS 管理端口。
- `.env`、配置文件、私钥和 token 不得提交 Git。首次登录后立即修改管理员密码并配置模型、Git 平台、镜像和主机资源。
- 商业计费、企业授权、社区 playground、Git OAuth 快捷入口和自动审查等特性默认关闭；它们不是私有内网任务执行的前置条件。

模型推理是否完全离线取决于部署方配置的模型 `base_url`。将模型指向内网兼容服务后，DevLoom 本身的中心、Runner、开发镜像和预览链路无需公网。
