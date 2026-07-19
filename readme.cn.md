# DevLoom

<p align="center">
  <img src="./frontend/public/logo-dark.png" alt="DevLoom" width="200" />
</p>

<p align="center">
  <a href="https://github.com/Y-vQv-Y/DevLoom/actions/workflows/build.yml"><img src="https://github.com/Y-vQv-Y/DevLoom/actions/workflows/build.yml/badge.svg" alt="Service Images" /></a>
  <a href="https://github.com/Y-vQv-Y/DevLoom/actions/workflows/electron-release.yml"><img src="https://github.com/Y-vQv-Y/DevLoom/actions/workflows/electron-release.yml/badge.svg" alt="Client Release" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg" alt="License: AGPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://github.com/Y-vQv-Y/DevLoom">项目主页</a> ·
  <a href="https://github.com/Y-vQv-Y/DevLoom/releases">版本公告</a> ·
  <a href="./docs/DEPLOYMENT_CN.md">部署文档</a> ·
  <a href="./docs/USER_GUIDE_CN.md">用户手册</a> ·
  <a href="#独立部署使用">独立部署</a> ·
  <a href="https://github.com/Y-vQv-Y/DevLoom/issues">问题与支持</a>
</p>

## DevLoom 是什么

DevLoom 是按 AGPL-3.0 发布的 AI 开发控制台，用于管理代码仓库、模型、项目和远程任务环境。本仓库包含 Go API、React Web、Electron 桌面端封装和 Expo 移动客户端。

本仓库不是完整的独立运行时。完整 AI 任务执行依赖 Taskflow、runner/host、preview、开发镜像和安装器产物；这些组件仅被配置引用，不在本仓库中构建。生产部署前必须提供兼容实现或镜像。

商业计费、广场发布、Git 身份 OAuth 快捷绑定、Apple 登录/账号注销和企业许可证界面默认关闭，因为开源 Go 后端没有实现这些接口。手动 Git Token 身份和密码账号仍可使用。项目自动审查已经由开源后端实现，但默认采用显式开启方式，并要求配置开发主机、审查模型和开发镜像。

## 界面展示

<table>
  <tr>
    <td align="center">
      <img src="./frontend/public/devloom-1.png" alt="DevLoom AI 任务工作台" />
      <br />
      <sub>AI 任务工作台</sub>
    </td>
    <td align="center">
      <img src="./frontend/public/devloom-2.png" alt="DevLoom 云端终端与任务执行" />
      <br />
      <sub>云端终端与任务执行</sub>
    </td>
  </tr>
  <tr>
    <td align="center">
      <img src="./frontend/public/devloom-3.png" alt="DevLoom 项目协作与文件管理" />
      <br />
      <sub>项目协作与文件管理</sub>
    </td>
    <td align="center">
      <img src="./frontend/public/devloom-mobile.png" alt="DevLoom 移动端任务与文件管理" />
      <br />
      <sub>移动端任务与文件管理</sub>
    </td>
  </tr>
</table>

## 功能与特色

- 管理仓库、项目、任务、模型、镜像、成员和团队策略。
- 提供 Web 控制台、Electron 桌面端和 Expo 移动端代码。
- 支持 Git 集成、MCP 配置、通知、审计记录和对象存储。
- 接入兼容运行服务后，可使用远程终端、文件和端口预览流程。
- 模型提供方、权限、资源限制和数据边界均由部署方管理。

## 使用指南

### 项目主页

以 GitHub 仓库作为项目主页、文档和版本发布入口：

[https://github.com/Y-vQv-Y/DevLoom](https://github.com/Y-vQv-Y/DevLoom)

### 独立部署使用

完整部署步骤见 [`docs/DEPLOYMENT_CN.md`](./docs/DEPLOYMENT_CN.md)，包括 Compose 变量、TLS、内网/离线资源、备份升级和故障排查。终端用户操作见 [`docs/USER_GUIDE_CN.md`](./docs/USER_GUIDE_CN.md)。

获取源码：

```bash
git clone https://github.com/Y-vQv-Y/DevLoom.git
cd DevLoom
```

部署配置从 `backend/config/server/config.yaml.example` 开始。`backend/docker-compose.yml` 还要求配置 `TASKFLOW_IMAGE`、`PREVIEW_IMAGE` 等镜像变量，这些镜像不由本仓库产出。

### 商业功能

开源后端没有实现套餐购买、充值、签到、邀请和支付接口。Web 与移动端默认分别使用 `VITE_ENABLE_COMMERCIAL_BILLING=false` 和 `EXPO_PUBLIC_ENABLE_COMMERCIAL_BILLING=false` 关闭相关入口；开启前必须自行实现兼容商业后端。

前端生成客户端还保留了注释为外部企业扩展实现的 License 接口，当前 Go 后端没有注册这些路由。默认使用 `VITE_ENABLE_ENTERPRISE_LICENSE=false` 隐藏授权页面并停止席位状态请求。

广场发布、Git 身份 OAuth 快捷绑定和移动端 Apple 登录/账号注销也默认关闭。项目自动审查由 `VITE_ENABLE_AUTO_REVIEW` 控制，完整路由和运行组件边界见 [`docs/OPEN_SOURCE_BOUNDARIES.md`](./docs/OPEN_SOURCE_BOUNDARIES.md)。

### 构建与发布

按要求使用 GitHub Actions，不依赖本地构建：

- `.github/workflows/build.yml` 测试并构建后端、前端、移动 Web、重新生成的后端 API 产物和 Docker 镜像。
- `.github/workflows/electron-release.yml` 构建发布二进制、前端压缩包、桌面安装包、原生 Android/iOS 移动端制品和 GHCR 镜像。移动端在 GitHub Runner 上本地构建，不使用 Expo/EAS 云构建。
- [`docs/AGENT_INTEGRATION_CN.md`](./docs/AGENT_INTEGRATION_CN.md) 说明 OpenHands、Taskflow 与隔离工作区的接入协议。

仓库变量、Secrets、产物和发布步骤见 [`docs/GITHUB_ACTIONS.md`](./docs/GITHUB_ACTIONS.md)。

## 社区与支持

欢迎加入技术社区，与更多开发者交流 DevLoom 的使用、部署和开发经验。

<table>
  <tr>
    <td align="center"><img src="./frontend/public/wechat.png" width="160" /><br/>微信交流群</td>
    <td align="center"><img src="./frontend/public/feishu.png" width="160" /><br/>飞书交流群</td>
    <td align="center"><img src="./frontend/public/dingtalk.png" width="160" /><br/>钉钉交流群</td>
  </tr>
</table>

你也可以通过以下入口获取支持：

- 使用文档：[README](https://github.com/Y-vQv-Y/DevLoom#readme)
- 版本公告：[GitHub Releases](https://github.com/Y-vQv-Y/DevLoom/releases)
- 问题与支持：[GitHub Issues](https://github.com/Y-vQv-Y/DevLoom/issues)
- GitHub Issues：[https://github.com/Y-vQv-Y/DevLoom/issues](https://github.com/Y-vQv-Y/DevLoom/issues)

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Y-vQv-Y/DevLoom&type=Date)](https://star-history.com/#Y-vQv-Y/DevLoom&Date)

## License

DevLoom 使用 [GNU Affero General Public License v3.0](./LICENSE) 开源。
