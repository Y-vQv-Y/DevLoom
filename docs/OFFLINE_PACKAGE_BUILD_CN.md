# ADTEC DevLoom 自有离线包构建手册

## 1. 构建边界

`deploy/package/build.sh` 只读取当前源码、`package.env` 指定的固定版本基础镜像和经过
SHA-256 校验的 Docker 静态包。它不会读取其他产品的离线目录，不会复制外部应用镜像，
也不会调用不可审计的安装二进制。

以下应用组件全部从本仓库构建：

- React frontend、Go backend 和 Nginx ingress；
- Taskflow 中心任务执行服务；
- preview relay；
- 远程 Orchestrator/Runner；
- devbox 开发镜像；
- 中心与开发主机安装脚本；
- Compose、项目模板、清单和验收脚本。

PostgreSQL、Redis、ClickHouse、RustFS、Docker Engine 和 Docker Compose 是第三方基础
设施制品。构建配置必须固定版本和校验值，最终服务器安装时不访问镜像仓库或软件下载站。

## 2. 构建机要求

- Linux amd64；
- Docker Engine 24+ 和 Docker Buildx；
- Python 3、curl、tar、gzip、sha256sum；
- 至少 150 GB 可用磁盘和足够的 Docker layer 空间；
- 已加载 `package.env` 中的四个基础设施镜像。

构建应用镜像使用 Dockerfile 内固定的 Node、Go、Alpine、Ubuntu 和 Nginx 基础镜像。
首次构建需要从企业镜像仓库或受控网络准备这些 layer；之后可以完全离线复现。

## 3. 配置

```bash
cp deploy/package/package.env.example deploy/package/package.env
```

当前 ADTEC 品牌配置：

```dotenv
BRAND_NAME="ADTEC DevLoom"
BRAND_SLUG=devloom
DEFAULT_INSTALL_DIR=/opt/devloom
TARGET_ARCH=amd64
IMAGE_PREFIX=devloom.local
```

`BRAND_SLUG` 决定包名、容器前缀和镜像名；`BRAND_NAME` 写入清单并用于默认团队名称。
若源码目录没有 `.git`，设置完整 40 位 `SOURCE_COMMIT`；同时通过 `--version` 明确包版本。

基础镜像必须使用固定版本或 digest，不能使用 `latest`：

```dotenv
POSTGRES_IMAGE=postgres:17.4-alpine3.21
REDIS_IMAGE=redis:8.0-alpine3.21
CLICKHOUSE_IMAGE=clickhouse/clickhouse-server:26.3.9
RUSTFS_IMAGE=rustfs/rustfs:1.0.0-beta.2
```

Docker 静态包可以由构建器下载，也可以提前放入构建机并设置 `DOCKER_BUNDLE_FILE`。两种
方式都必须填写 Docker Engine 与 Compose 的 SHA-256。`package.env` 已被 Git 忽略，
不得在其中保存业务密码或模型密钥。

## 4. 构建

```bash
docker image inspect "$POSTGRES_IMAGE"
docker image inspect "$REDIS_IMAGE"
docker image inspect "$CLICKHOUSE_IMAGE"
docker image inspect "$RUSTFS_IMAGE"
bash deploy/package/build.sh --version v1.0.0-dtec.1
```

构建器依次完成：

1. 构建 frontend、backend、ingress、Taskflow、preview、Orchestrator 和 devbox；
2. 导出中心运行所需的九个镜像；
3. 把 Orchestrator、preview 和 devbox 组成开发主机 `host.tgz`；
4. 生成可读的中心与开发主机安装器；
5. 生成项目模板、`manifest.json` 和 `SHA256SUMS`；
6. 逐项验证 23 个清单文件并生成最终 TGZ 与外层 SHA-256。

输出：

```text
deploy/out/devloom-offline-linux-amd64/
deploy/out/devloom-offline-linux-amd64.tgz
deploy/out/devloom-offline-linux-amd64.tgz.sha256
```

主包结构：

```text
install.sh
preflight.sh
verify.sh
manifest.json
SHA256SUMS
.env.example
docker-compose.yml
docker.tgz
images/*.tar.gz
static/project-tpl.zip
static/installer/x86_64/installer
static/installer/x86_64/host.tgz
static/installer/x86_64/docker.tgz
extensions/packages/
```

`install.sh` 与 `static/installer/x86_64/installer` 都是可读 ASCII shell 脚本。

## 5. 构建验收

```bash
sha256sum -c deploy/out/devloom-offline-linux-amd64.tgz.sha256
python3 deploy/package/manifest_tool.py verify deploy/out/devloom-offline-linux-amd64
tar -tzf deploy/out/devloom-offline-linux-amd64.tgz | sort
file deploy/out/devloom-offline-linux-amd64/install.sh
file deploy/out/devloom-offline-linux-amd64/static/installer/x86_64/installer
```

还必须确认：

- `manifest.json` 的 `version`、`commit`、`brand` 和 `arch` 与本次发布一致；
- 包路径中没有专利材料、构建缓存、密钥、`package.env` 或外部应用包目录；
- `host.tgz` 中存在 orchestrator、preview 和 devbox 镜像；
- 在全新 Linux 目录完成实际安装，而不是只验证压缩包可解开。

## 6. 内网安装与升级

```bash
sha256sum -c devloom-offline-linux-amd64.tgz.sha256
tar -xzf devloom-offline-linux-amd64.tgz
cd devloom-offline-linux-amd64
sudo bash install.sh --host devloom.intra.example --admin-email admin@intra.example
```

安装器校验全部文件、按需安装包内 Docker、导入镜像、生成随机密码与 TLS 证书、启动
Compose 并运行健康检查。首次管理员密码只输出一次。重复执行新版本安装器会保存 `.env` 与
Compose 快照，保留数据库、对象存储、证书和现有密钥，再导入新镜像并执行后端迁移。
升级前仍必须做数据库与对象存储备份。

开发主机安装器会验证 `installer`、`host.tgz` 和 `docker.tgz` 的 SHA-256，再启动带认证的
Orchestrator。中心签名密钥不会下发到 Runner；Runner 使用后端校验的安装 Token 换取绑定
机器 ID 的会话凭据。

## 7. 可重现与安全

- 应用镜像由当前源码和固定 Dockerfile 构建；
- `manifest.json` 记录源提交、版本、构建时间、架构、文件大小和 SHA-256；
- `SOURCE_COMMIT` 支持没有 Git 元数据的源码归档；
- Runner 的持久化状态不保存模型 API Key；
- 模型 Key、Git Token、管理员密码和签名密钥只在部署后的私有配置或数据库中保存；
- 重新分发前分别审核 AGPL 源码、基础镜像和 tldraw 等第三方组件许可证。
