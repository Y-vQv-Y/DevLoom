# DevLoom Brand Replacement Guide

## Identity and Links

The product name is **DevLoom** and the canonical repository is `https://github.com/Y-vQv-Y/DevLoom`. Web links are centralized in `frontend/src/config/brand.ts`. Configure a deployment with `VITE_PUBLIC_SITE_URL`, `VITE_DOCS_URL`, `VITE_ANNOUNCEMENT_URL`, `VITE_FORUM_URL`, `VITE_CONSULTATION_URL`, `VITE_COMPANY_URL`, `VITE_COMMUNITY_URL`, and `VITE_SUPPORT_URL`; unset values fall back to the GitHub owner, repository, Releases, or Issues.

The Go module is `github.com/Y-vQv-Y/DevLoom/backend`. Runtime environment variables use the `DEVLOOM_` prefix, user-level files use `.devloom`, Electron uses `DEVLOOM_DESKTOP_URL`, and mobile storage uses `devloom.*`. Mobile identifiers default to `io.github.yvqvy.devloom` and can be overridden by Actions variables.

| Entry | Default | Override |
|---|---|---|
| Website | `https://github.com/Y-vQv-Y/DevLoom` | `VITE_PUBLIC_SITE_URL` |
| Documentation | Repository README | `VITE_DOCS_URL` |
| Forum | GitHub Issues | `VITE_FORUM_URL` |
| GitHub | `https://github.com/Y-vQv-Y/DevLoom` | Fixed canonical repository |
| Consulting | New GitHub Issue | `VITE_CONSULTATION_URL` |
| Company/Maintainer | `https://github.com/Y-vQv-Y` | `VITE_COMPANY_URL` |
| Community | GitHub Issues | `VITE_COMMUNITY_URL` |
| Announcements | GitHub Releases | `VITE_ANNOUNCEMENT_URL` |

## Images to Replace Manually

Keep each filename and replace the file contents so existing web, Electron, Expo, email, README, and release packaging references remain valid.

```text
E:\Code\MonkeyCode\frontend\public\logo.png
E:\Code\MonkeyCode\frontend\public\logo-light.png
E:\Code\MonkeyCode\frontend\public\logo-dark.png
E:\Code\MonkeyCode\frontend\public\logo-colored.png
E:\Code\MonkeyCode\frontend\public\devloom-1.png
E:\Code\MonkeyCode\frontend\public\devloom-2.png
E:\Code\MonkeyCode\frontend\public\devloom-3.png
E:\Code\MonkeyCode\frontend\public\devloom-mobile.png
E:\Code\MonkeyCode\frontend\public\head.jpg
E:\Code\MonkeyCode\frontend\public\task-1.png
E:\Code\MonkeyCode\frontend\public\task-2.png
E:\Code\MonkeyCode\frontend\public\task-3.png
E:\Code\MonkeyCode\frontend\public\qrcode.png
E:\Code\MonkeyCode\frontend\public\wechat.png
E:\Code\MonkeyCode\frontend\public\feishu.png
E:\Code\MonkeyCode\frontend\public\dingtalk.png
E:\Code\MonkeyCode\frontend\electron\icon.png
E:\Code\MonkeyCode\desktop\electron\icon.png
E:\Code\MonkeyCode\mobile\assets\icon.png
E:\Code\MonkeyCode\mobile\assets\icon-dark.png
E:\Code\MonkeyCode\mobile\assets\adaptive-icon.png
E:\Code\MonkeyCode\mobile\assets\favicon.png
E:\Code\MonkeyCode\mobile\assets\logo-light.png
E:\Code\MonkeyCode\mobile\assets\logo-dark.png
E:\Code\MonkeyCode\mobile\assets\splash.png
E:\Code\MonkeyCode\mobile\assets\splash-dark.png
E:\Code\MonkeyCode\mobile\assets\provider-light.png
E:\Code\MonkeyCode\mobile\assets\provider-dark.png

Repository screenshots and unused image assets to review manually:

E:\Code\MonkeyCode\.github\pr-assets\832\provider-model-search-filtered.png
E:\Code\MonkeyCode\.github\pr-assets\832\provider-model-search-list.png
E:\Code\MonkeyCode\frontend\src\assets\react.svg
```

The QR-code images still point to the previous communities until replaced. The `provider-*.png` files are retained only as manual replacement placeholders and are not used by the current login screen. Do not replace third-party assets under `frontend/public/tldraw/` unless separately required by their license or design system.

Visual inspection on 2026-07-18 confirmed that `frontend/public/logo-dark.png`, `desktop/electron/icon.png`, and `mobile/assets/icon.png` still contain the previous mascot artwork. Treat every first-party path above as pending manual replacement even though source text, filenames, package metadata, and links have been renamed.

## External Runtime Dependencies

Runner, Taskflow, preview, and development-image binaries are not built from this repository. Their image names, `DEVLOOM_*` environment variables, `.devloom` paths, and installer bundles must be updated in those repositories before using the renamed Docker Compose stack. Host installer files are expected under the configured `static_files` route instead of an old public download service.

The open-source backend also omits billing, payment, invitation, check-in, recharge, playground, Git identity OAuth authorization-URL, automatic project review, Apple authentication/account-deletion, and enterprise-license routes. Keep the corresponding flags in `docs/GITHUB_ACTIONS.md` set to `false` unless compatible services are supplied. The whiteboard uses tldraw; obtain a license appropriate for your distribution and inject it through `VITE_TLDRAW_LICENSE_KEY` instead of reusing a third-party key.
