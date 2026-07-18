# Open-Source Runtime Boundaries

## Default Usable Surface

With PostgreSQL, Redis, valid server/session configuration, and required runtime services connected, the open-source backend provides password accounts, teams, members, policies, repositories, manual Git access-token identities, projects, models, images, skills, MCP, notifications, audit records, and task-control APIs. Object uploads require `object_storage.enabled=true`.

Commercial billing can remain disabled without affecting those management surfaces. The subscription status endpoint returns the basic plan, while frontend model and concurrency checks bypass commercial plan restrictions when billing is off.

## Default-Off API Extensions

The Go backend does not implement these feature groups:

| Feature | Default flag |
|---|---|
| Wallet, plans, recharge, check-in, invitation, payment | `VITE_ENABLE_COMMERCIAL_BILLING=false` / `EXPO_PUBLIC_ENABLE_COMMERCIAL_BILLING=false` |
| Enterprise license and seat status | `VITE_ENABLE_ENTERPRISE_LICENSE=false` |
| Playground listing and publishing | `VITE_ENABLE_COMMUNITY_PLAYGROUND=false` |
| Git identity OAuth authorization shortcuts | `VITE_ENABLE_GIT_IDENTITY_OAUTH=false` |
| Apple login and account deletion | `EXPO_PUBLIC_ENABLE_APPLE_AUTH=false` |

Do not enable a flag until a compatible backend extension is deployed. Manual Git Token identities remain available without Git OAuth shortcuts.

## External Runtime Components

The repository does not build Taskflow, runner/host binaries, preview service, development images, or host installer bundles. `TASKFLOW_SERVER` is mandatory at backend startup, and `backend/docker-compose.yml` expects externally supplied `TASKFLOW_IMAGE` and `PREVIEW_IMAGE` values. Full AI task creation, terminal, file, and preview flows cannot work without these compatible components.

The whiteboard is based on tldraw. Review its production licensing for the intended distribution and provide `VITE_TLDRAW_LICENSE_KEY` when required. Enterprise licensing, commercial APIs, connected model services, Git providers, EAS credentials, signing certificates, and infrastructure are independently operated or licensed.

## Verification Scope

The white-label audit used full-repository text/link scans, frontend-to-Swagger route comparison, `actionlint`, and `git diff --check`. No local project build or test was run; GitHub Actions is the authoritative compile and test path.
