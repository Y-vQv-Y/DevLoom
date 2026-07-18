# GitHub Actions 编译与发布手册

## 工作流说明

仓库提供两份可直接运行的工作流：

- `.github/workflows/build.yml`：在 Pull Request、推送到 `main` 或手动触发时执行 CI。
- `.github/workflows/electron-release.yml`：在推送 `v*` 标签或手动触发时构建发布产物。

CI 使用 Node.js 22、pnpm 9 和 Go 1.25.x。无需在本地编译。

## 首次启用

1. 将代码推送到 `https://github.com/Y-vQv-Y/DevLoom` 的 `main` 分支。
2. 打开仓库的 `Settings > Actions > General`，允许 GitHub Actions 运行。
3. 确认工作流可以使用 `GITHUB_TOKEN`；发布工作流已声明 `contents: write` 和 `packages: write`。
4. 打开 `Actions > CI > Run workflow`，首次手动运行 CI。

普通 CI 不要求任何 Secrets。商业、企业授权、广场、Git OAuth 和 Apple 登录默认关闭。

## Repository Variables

在 `Settings > Secrets and variables > Actions > Variables` 中按需配置：

| 变量 | 默认值 | 用途 |
|---|---|---|
| `VITE_ENABLE_COMMERCIAL_BILLING` | `false` | Web 套餐、钱包、充值与积分入口 |
| `EXPO_PUBLIC_ENABLE_COMMERCIAL_BILLING` | `false` | 移动端商业入口 |
| `VITE_ENABLE_ENTERPRISE_LICENSE` | `false` | 外部企业许可证接口 |
| `VITE_ENABLE_COMMUNITY_PLAYGROUND` | `false` | 外部广场列表与发布接口 |
| `VITE_ENABLE_GIT_IDENTITY_OAUTH` | `false` | Git 身份 OAuth 快捷绑定 |
| `VITE_GITHUB_APP_INSTALL_URL` | 空 | GitHub App 安装地址 |
| `VITE_GLOBAL_GITHUB_APP_INSTALL_URL` | 空 | 国际区域 GitHub App 安装地址 |
| `ENABLE_MOBILE_RELEASE` | `false` | 每次 Release 自动执行 EAS |
| `EXPO_PROJECT_ID` | 无 | EAS 项目 ID，移动构建必填 |
| `EXPO_OWNER` | 无 | Expo 账号或组织 |
| `DEVLOOM_MOBILE_API_URL` | 无 | 移动端生产 API 地址 |
| `DEVLOOM_EXPO_UPDATES_URL` | 空 | Expo Updates 地址 |
| `DEVLOOM_UPDATES_SERVER` | 空 | 应用内更新服务地址 |
| `IOS_BUNDLE_ID` | `io.github.yvqvy.devloom` | iOS Bundle Identifier |
| `ANDROID_PACKAGE` | `io.github.yvqvy.devloom` | Android Application ID |
| `EXPO_PUBLIC_ENABLE_APPLE_AUTH` | `false` | Apple 登录与账号注销扩展 |

不要在缺少对应后端接口时开启前五个功能开关。手动 Git Access Token 绑定不依赖 Git OAuth。

## Repository Secrets

| Secret | 是否必需 | 用途 |
|---|---|---|
| `VITE_TLDRAW_LICENSE_KEY` | 按分发场景 | tldraw 生产许可证 |
| `EXPO_TOKEN` | EAS 必需 | Expo/EAS 非交互认证 |
| `IOS_TEAM_ID` | 按凭据配置 | Apple Developer Team ID |

GHCR 使用工作流自带的 `GITHUB_TOKEN`，不需要额外 Docker 密码。Electron 当前输出未签名产物；正式分发前需要另行配置 Windows 代码签名和 macOS 签名、公证凭据。

## 运行 CI

CI 自动执行：

- `go test ./...` 并生成 Linux AMD64 服务端二进制。
- Web ESLint、Node 回归测试、在线版和离线版 Vite 构建。
- Mobile ESLint、Jest、Expo 配置检查和 Web 导出。
- Ent 与 Swagger 再生成一致性检查。
- Frontend、Backend、Ingress Docker 镜像构建检查，不推送镜像。

成功后可在本次 Actions Run 的 `Artifacts` 下载：

```text
backend-linux-amd64
frontend-builds
mobile-web
regenerated-backend-api
```

`frontend/src/api/Api.ts` 同时包含开源接口和可选外部扩展契约，因此 CI 通过 Web 构建进行类型检查，但不会用较小的开源 Swagger 覆盖该文件。

## 构建正式版本

推荐通过 SemVer 标签发布：

```bash
git tag v1.0.0
git push origin v1.0.0
```

标签必须符合 `v1.2.3` 或 `v1.2.3-beta.1`。标签触发后会自动：

- 构建 Linux AMD64/ARM64、Windows AMD64、macOS AMD64/ARM64 Go 二进制。
- 构建 Web 在线版和离线版压缩包。
- 构建 Windows Portable/NSIS 和 macOS DMG。
- 构建并推送 `ghcr.io/y-vqv-y/devloom-{frontend,backend,ingress}` 多架构镜像。
- 打包 `devloom-source.tar.gz` 并创建 GitHub Release。

也可在 `Actions > Release > Run workflow` 手动运行：

- `publish=false`：仅构建并保留 Actions Artifacts，不发布 GHCR 或 GitHub Release。
- `publish=true`：使用 `0.0.<run_number>` 作为版本并发布。
- `mobile=true`：同时启动 Android 和 iOS EAS production 构建。

## Release 产物

发布工作流会生成：

```text
devloom-server-linux-amd64
devloom-server-linux-arm64
devloom-server-windows-amd64.exe
devloom-server-darwin-amd64
devloom-server-darwin-arm64
devloom-frontend-online.tar.gz
devloom-frontend-offline.tar.gz
Windows Electron portable / NSIS
macOS Electron DMG (x64 / arm64)
devloom-source.tar.gz
```

移动端产物保存在 EAS 项目中，不会直接上传到 GitHub Release。`mobile/app.config.js` 会将 Actions 中的 Expo Project ID、Owner、更新地址、Bundle ID、Package 和 Apple 登录开关写入 EAS 配置。

## 部署边界

Actions 只构建本仓库包含的服务。完整 AI 任务仍需要外部 Taskflow、runner/host、preview、开发镜像和宿主机安装包；Release 不会生成 `TASKFLOW_IMAGE` 或 `PREVIEW_IMAGE`。详细边界见 `docs/OPEN_SOURCE_BOUNDARIES.md`。

## 常见失败

- `frozen-lockfile` 或 `npm ci` 失败：依赖声明与对应锁文件不一致，需要更新并提交锁文件。
- `Validate generated diff` 失败：Ent 或 Swagger 源文件变化后未提交生成结果。
- Electron 无产物：检查 `desktop/release/`、图标文件和 electron-builder 配置。
- GHCR 返回权限错误：检查仓库或组织是否限制 `GITHUB_TOKEN` 写入 Packages。
- EAS 提示未认证：检查 `EXPO_TOKEN`、`EXPO_PROJECT_ID`、Owner 和项目访问权限。
- iOS/Android 构建失败：在 EAS 项目中检查证书、Provisioning Profile、Keystore、Bundle ID 和 Package 是否一致。
