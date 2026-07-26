# Open-Source Runtime Boundaries

## Default Usable Surface

With PostgreSQL, Redis, valid server/session configuration, and required runtime services connected, the open-source backend provides password accounts, teams, members, policies, repositories, manual Git access-token identities, projects, Git webhook review, models, images, skills, MCP, notifications, audit records, and task-control APIs. Object uploads require `object_storage.enabled=true`.

Commercial billing can remain disabled without affecting those management surfaces. The subscription status endpoint returns the basic plan, while frontend model and concurrency checks bypass commercial plan restrictions when billing is off.

## Default-Off API Extensions

The Go backend does not implement these feature groups:

| Feature | Default flag |
|---|---|
| Wallet, plans, recharge, check-in, invitation, payment | `VITE_ENABLE_COMMERCIAL_BILLING=false` / `EXPO_PUBLIC_ENABLE_COMMERCIAL_BILLING=false` |
| Enterprise license and seat status | `VITE_ENABLE_ENTERPRISE_LICENSE=false` |
| Playground listing and publishing | `VITE_ENABLE_COMMUNITY_PLAYGROUND=false` |
| Git identity OAuth authorization shortcuts | `VITE_ENABLE_GIT_IDENTITY_OAUTH=false` / `EXPO_PUBLIC_ENABLE_GIT_IDENTITY_OAUTH=false` |
| Apple login and account deletion | `EXPO_PUBLIC_ENABLE_APPLE_AUTH=false` |

Do not enable a flag until a compatible backend extension is deployed. Manual Git Token identities remain available without Git OAuth shortcuts.

Project automatic review is implemented by the open-source backend but remains an explicit UI opt-in with `VITE_ENABLE_AUTO_REVIEW=true`, because enabling it registers a repository webhook and consumes a configured development host. Git webhook review tasks also require `review_agent.model_id` and `review_agent.image`; GitLab merge-request comments can trigger a task with `task.at_keyword` (default `@devloom`).

GitHub pull-request comments use the same configured mention and support `review`, `fix`, `explain`, and `plan` commands. These commands create isolated Git tasks. The bundled Taskflow clones the protected base branch, creates the task-specific work branch, configures the Git identity, and installs a pre-push guard that permits only the work branch (or rejects every push when push mode is disabled). Private clone credentials use a temporary `GIT_ASKPASS` helper and do not appear in process arguments. External Workspace Adapters must enforce the same structured policy. OpenHands Agent Server integration remains an optional external runtime contract documented in `docs/AGENT_INTEGRATION_CN.md`, not the default bundled agent loop.

## Source Runtime Components

The repository builds Taskflow, the remote Orchestrator, preview service, devbox development image, and readable host installer bundles. `backend/docker-compose.yml` connects the backend to the source Taskflow and preview images. The independent offline package exports every application image and only reads this source tree plus explicitly pinned third-party infrastructure artifacts.

For a private deployment, install the center bundle, register at least one source Runner, configure an internal model, and select the bundled devbox image. Runner installation tokens are validated by the backend and exchanged for per-host credentials; VM and task routes survive center and Runner restarts. GitLab certificate verification remains enabled by default. Install the internal CA instead of setting `tls_insecure_skip_verify: true` except for temporary diagnosis.

Default task skills are empty so a private deployment does not depend on an external plugin repository. After installing skills, set comma-separated IDs in `VITE_DEFAULT_SKILL_IDS` and `EXPO_PUBLIC_DEFAULT_SKILL_IDS` if they should be selected automatically.

The whiteboard is based on tldraw. Review its production licensing for the intended distribution and provide `VITE_TLDRAW_LICENSE_KEY` when required. Enterprise licensing, commercial APIs, connected model services, Git providers, Apple Developer signing certificates, Android keystores, and infrastructure are independently operated or licensed. Mobile compilation itself runs on GitHub-hosted runners and does not require Expo/EAS credentials.

WebSocket Origin verification is enabled by default. Only set `DEVLOOM_WS_INSECURE_SKIP_VERIFY=true` when a deliberately cross-origin deployment cannot provide matching Origin headers; keep the frontend and API behind the same trusted origin whenever possible.

## Verification Scope

The release audit uses full-repository text/link scans, frontend-to-Swagger route comparison, Go and frontend tests, production builds, Compose validation, offline manifest verification, and real Web-to-Agent-to-preview E2E testing. Run `actionlint` before publishing workflow changes.
