# DevLoom 自有离线包构建手册

## 1. 官方包内部结构结论

现有 `monkeycode-offline-linux-amd64` 的顶层 `install.sh` 只检查 root、读取 `package.env`，随后执行：

```bash
./installer center
```

`installer` 是 64 位 Linux amd64 ELF，不是脚本。静态分析结果如下：

- Go 版本：`go1.25.8`；
- 私有模块：`github.com/chaitin/MonkeyCodePro/cmd/installer`；
- 源提交：`a7a8f9c6225bcf53753aea7c60a86c0fa55e4b92`；
- 内置中心安装、升级、回滚、Docker 安装、镜像导入、TLS、host 安装等函数；
- 只有二进制和符号信息，没有可还原、可维护的完整 Go 源码。

可以在 Linux 上自行查看：

```bash
file installer
sha256sum installer
go version -m installer
readelf -h installer
strings installer | less
```

仓库也提供了只读检查工具：

```bash
bash deploy/package/inspect-installer.sh deploy/monkeycode-offline-linux-amd64/installer
```

原包还包含九个中心镜像、Docker 静态包、Taskflow/preview、双架构 host runner、项目模板和扩展包。其 `manifest.json` 负责版本及 SHA-256 记录。

## 2. 自有包与外部边界

本仓库新增的构建链路不会读取官方离线目录，也不会复制或调用闭源 `installer`。以下内容从当前仓库构建：

- DevLoom frontend；
- DevLoom backend；
- DevLoom ingress；
- Taskflow；
- preview relay；
- 远程 Orchestrator；
- devbox 开发镜像；
- 中心安装、升级准备、校验和验收脚本；
- host 安装脚本；
- Compose、项目模板、包清单和 SHA-256 锁定文件；
- Docker Engine 和 Docker Compose 静态包。

构建前只需把固定版本的 PostgreSQL、Redis、ClickHouse 和 RustFS 基础设施镜像加载到本地 Docker。最终 TGZ 包含从当前源码构建的全部 DevLoom 应用运行时和这些基础镜像，部署服务器不再访问外部仓库。

## 3. 品牌配置

当前源码品牌为 `DevLoom`。界面名称、链接和素材按 [品牌替换手册](./BRANDING.md) 维护。构建包之前先完成源码级品牌替换，避免只修改容器名但页面仍显示旧品牌。

复制配置：

```bash
cp deploy/package/package.env.example deploy/package/package.env
```

关键字段：

```dotenv
BRAND_NAME=DevLoom
BRAND_SLUG=devloom
DEFAULT_INSTALL_DIR=/opt/devloom
IMAGE_PREFIX=registry.internal.example/devloom
```

`BRAND_SLUG` 用于包名、Compose 项目名、容器前缀和自构建镜像名。`BRAND_NAME` 用于初始团队和包元数据。镜像必须固定版本或 digest，构建器拒绝 `latest` 和占位值。

若由构建器联网下载 Docker/Compose 静态二进制，还必须填写官方发布页给出的 `DOCKER_TGZ_SHA256` 和 `COMPOSE_BINARY_SHA256`。完全离线构建时，可提前准备并校验 `DOCKER_BUNDLE_FILE`。

## 4. 准备镜像

构建机必须预先存在以下镜像：

```bash
docker image inspect "$POSTGRES_IMAGE"
docker image inspect "$REDIS_IMAGE"
docker image inspect "$CLICKHOUSE_IMAGE"
docker image inspect "$RUSTFS_IMAGE"
```

四个基础镜像可以来自企业内部仓库或 `docker load` 导入的合规制品。应用镜像由构建器生成；整个过程不依赖任何特定外部离线包目录。

## 5. 构建完整离线包

构建机要求 Linux amd64、Docker/Buildx、Node.js、pnpm、Python 3、curl、tar、gzip 和 sha256sum。

```bash
bash deploy/package/build.sh --version v1.0.0
```

执行过程：

1. 构建 frontend、backend、ingress、Taskflow、preview、Orchestrator 和 devbox 镜像；
2. 导出九个中心镜像；
3. 导出 orchestrator/devbox 并生成 host runner 包；
4. 下载固定版本 Docker Engine/Compose 静态二进制；
5. 生成自有 host 安装脚本、项目模板和 Compose；
6. 生成 `manifest.json`、`SHA256SUMS` 和最终 TGZ。

默认输出：

```text
deploy/out/devloom-offline-linux-amd64/
deploy/out/devloom-offline-linux-amd64.tgz
deploy/out/devloom-offline-linux-amd64.tgz.sha256
```

包结构：

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

其中两个 `installer` 均已被可读 shell 脚本替代。中心安装脚本不会执行来源不明的二进制。

## 6. 内网安装

把 TGZ 和 `.sha256` 传入内网服务器：

```bash
sha256sum -c devloom-offline-linux-amd64.tgz.sha256
tar -xzf devloom-offline-linux-amd64.tgz
cd devloom-offline-linux-amd64
sudo bash install.sh \
  --host devloom.intra.example \
  --admin-email admin@intra.example
```

安装器会验证包内所有文件、安装静态 Docker（服务器已有可用 Docker 时跳过）、导入全部镜像、生成随机密钥和 gRPC TLS 证书、启动 Compose 并执行健康检查。随机管理员密码只在首次安装结束时输出一次。

开发主机安装器同样是可读脚本，会验证 installer、host 和 Docker 包。团队扩展包配置了额外开发镜像时，host 需要 `python3` 解析后端生成的 JSON 清单，并逐个校验清单内 SHA-256 后导入。

重复运行新版本 `install.sh` 会保存 Compose 和 `.env` 快照，保留数据库、对象存储、证书和现有密钥，再导入新镜像并重新部署。数据库迁移仍由 backend 启动时执行，升级前必须完成数据备份。

## 7. 完整性与授权

- `manifest.json` 记录品牌、版本、Git commit、架构、构建时间、文件大小和 SHA-256；
- `SHA256SUMS` 在安装开始前逐项验证；
- 开发主机下载 `host.tgz`/`docker.tgz` 时验证同目录 `.sha256`；
- `package.env`、构建输出和外部离线包均已从 Git 排除；
- AGPL 源码、第三方基础镜像和 tldraw 等组件必须分别确认再分发许可。
