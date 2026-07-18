# DevLoom Frontend Guide

The web client is a Vite + React control plane for projects, AI tasks, Git identities, models, remote environments, team administration, and notifications. It is not a standalone AI runtime.

## Default Routes

- `/` presents the project and links to deployment documentation.
- `/login` provides password and configured OAuth login.
- `/console` contains user projects, tasks, settings, terminal, and file views.
- `/manager` contains team members, policies, hosts, models, images, skills, MCP, notifications, and audit views.
- `/privacy-policy`, `/user-agreement`, and `/self-hosting` provide operator-facing legal and deployment templates.

The playground routes remain in source but redirect home unless `VITE_ENABLE_COMMUNITY_PLAYGROUND=true` because the open-source backend does not implement their APIs.

## Optional Features

The following UI is disabled by default:

```text
VITE_ENABLE_COMMERCIAL_BILLING=false
VITE_ENABLE_ENTERPRISE_LICENSE=false
VITE_ENABLE_COMMUNITY_PLAYGROUND=false
VITE_ENABLE_GIT_IDENTITY_OAUTH=false
```

Do not enable these values without compatible backend implementations. Commercial billing covers wallet, recharge, subscription purchase, invitation, check-in, and payment behavior. Git OAuth shortcuts require authorization URL endpoints or GitHub Apps; users can add Git access-token identities manually without them.

## Runtime Requirements

The Go backend supplies account, project, team, model, task-control, and configuration APIs. Full task execution additionally needs compatible Taskflow, runner/host, preview, development-image, and installer artifacts. Upload and avatar operations require `object_storage.enabled=true`.

The generated `src/api/Api.ts` includes contracts for optional external extensions as well as the open-source backend. Feature flags must guard calls to routes that are absent from the current deployment.

## Development

```bash
pnpm dev:online
pnpm lint
pnpm test
pnpm build:online
```

Run these commands from `frontend/`. Public links and brand defaults are centralized in `src/config/brand.ts`; feature defaults are in `src/config/features.ts`.
