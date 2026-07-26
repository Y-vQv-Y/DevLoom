# ADTEC DevLoom 品牌维护手册

## 品牌层级

- 企业品牌：`ADTEC`；
- 产品名称：`DevLoom`；
- 完整展示名：`ADTEC DevLoom`；
- 仓库与技术标识：`DevLoom`、`devloom`；
- Go module：`github.com/Y-vQv-Y/DevLoom/backend`；
- 稳定协议环境变量：`DEVLOOM_*`。

包名、镜像名、目录名和环境变量保持 ASCII 技术标识，用户界面与离线包清单使用完整展示名。

## Web 素材

认证页面使用完整 ADTEC lockup：

```text
frontend/public/logo-brand.png
```

侧栏、favicon 和紧凑界面使用透明紧凑标志：

```text
frontend/public/logo.png
frontend/public/logo-light.png
frontend/public/logo-dark.png
frontend/public/logo-colored.png
```

以下文件是当前产品截图或其他客户端发布素材，发布对应客户端前必须逐项确认仍为批准的
ADTEC 品牌版本：

```text
frontend/public/devloom-1.png
frontend/public/devloom-2.png
frontend/public/devloom-3.png
frontend/public/devloom-mobile.png
frontend/electron/icon.png
desktop/electron/icon.png
mobile/assets/icon.png
mobile/assets/icon-dark.png
mobile/assets/adaptive-icon.png
mobile/assets/favicon.png
mobile/assets/logo-light.png
mobile/assets/logo-dark.png
mobile/assets/splash.png
mobile/assets/splash-dark.png
```

不要修改 `frontend/public/tldraw/` 的第三方素材来冒充产品品牌。社区二维码和未使用的
provider 图片不是认证页依赖；对外发布前应删除不用的旧社区素材或替换为 ADTEC 运营入口。

## 链接配置

Web 链接集中在 `frontend/src/config/brand.ts`。私有部署通过以下构建变量指向企业站点：

```text
VITE_PUBLIC_SITE_URL
VITE_DOCS_URL
VITE_ANNOUNCEMENT_URL
VITE_FORUM_URL
VITE_CONSULTATION_URL
VITE_COMPANY_URL
VITE_COMMUNITY_URL
VITE_SUPPORT_URL
```

未设置时回退到当前 GitHub 仓库、Releases 或 Issues。内网无 GitHub 访问时必须在构建镜像
前设置内网站点，不能在安装后只改容器名。

## 运行时与离线包

`deploy/package/package.env` 使用：

```dotenv
BRAND_NAME="ADTEC DevLoom"
BRAND_SLUG=devloom
IMAGE_PREFIX=devloom.local
```

frontend、backend、ingress、Taskflow、preview、Orchestrator、devbox、Compose、中心安装器、
开发主机安装器和离线清单都由仓库构建。`BRAND_SLUG` 改动会影响镜像、容器和包名，必须
整包重建；`DEVLOOM_*` 是内部协议的一部分，不应只为视觉改名。

## 发布检查

1. 登录页显示完整 ADTEC lockup，侧栏和 favicon 显示紧凑标志；
2. 桌面与窄屏没有裁切、拉伸或文字覆盖；
3. 页面标题、邮件、通知、安装输出和 `manifest.json` 使用 ADTEC DevLoom；
4. 包路径与文本不包含旧产品品牌、专利材料、外部应用下载地址或第三方安装器；
5. 桌面和移动端发布前单独检查图标、启动图、包标识和签名主体；
6. tldraw 和所有第三方资源按实际分发方式配置许可证。
