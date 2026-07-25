# DevLoom 内网离线部署

本文对应官方部署说明，目标是让内网服务器具备完整的 Web、API、任务执行、终端、文件和预览能力。仓库源码本身不包含 Taskflow/runner、preview、开发主机安装包和全部 OCI 镜像；这些运行时必须从官方离线包或企业内部镜像仓库准备，不能用占位镜像替代。

## 1. 准备离线介质

在可联网机器下载官方离线包，并将它和镜像、静态资源转移到内网：

```text
https://monkeycode-release.oss-cn-hangzhou.aliyuncs.com/public/offline-package/monkeycode-offline-linux-amd64.tgz
```

同时准备以下文件（按服务器架构选择 `amd64` 或 `arm64`）：

```text
/opt/devloom/static/project-tpl.zip
/opt/devloom/static/installer/<arch>/installer
/opt/devloom/static/installer/<arch>/host.tgz
/opt/devloom/static/installer/<arch>/docker.tgz
```

镜像仓库必须提供 PostgreSQL、Redis、ClickHouse、RustFS、frontend、backend、ingress、Taskflow 和 preview 的固定版本或 digest。Taskflow 和 preview 是任务执行、终端和预览功能的必要依赖。

## 2. 一键安装

在 Linux 服务器以 root 执行。脚本会检查架构、下载/校验离线包并调用包内官方安装器：

```bash
sudo bash deploy/offline/install.sh \
  --package /media/monkeycode-offline-linux-amd64.tgz \
  --install-root /opt/devloom \
  --sha256 '<官方发布的 SHA256>'
```

如果服务器不能访问外网，必须使用 `--package`；脚本不会自动制造缺失的 AI 运行时。

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

若官方安装器没有自动启动服务，使用安装目录中的 Compose 文件：

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

源码离线部署不等于 AI 运行时离线。缺少官方 Taskflow/runner/preview 或 host 包时，管理页面可以启动，但任务执行、终端、文件和预览无法完整使用。
