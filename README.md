# ADTEC DevLoom

<p align="center">
  <img src="./frontend/public/logo-brand.png" alt="ADTEC DevLoom" width="260" />
</p>

ADTEC DevLoom is a self-hosted AI software-delivery platform for teams that need
repository, project, model, task, workspace, terminal, file, diff, and live
preview workflows inside a controlled network. The complete Linux runtime is
built from this repository: web UI, Go API, ingress, Taskflow, preview relay,
remote Orchestrator/Runner, devbox image, center installer, and host installer.

No external application package is read or invoked by the build. PostgreSQL,
Redis, ClickHouse, RustFS, Docker Engine, and Docker Compose are pinned
third-party infrastructure dependencies bundled into the offline archive.

## Platform Surface

- Password accounts, teams, members, groups, permissions, and audit records.
- Git identities, repositories, projects, issues, merge requests, and webhooks.
- OpenAI-compatible and Anthropic-compatible model providers.
- AI development tasks with isolated Docker workspaces and lifecycle control.
- Local center execution and authenticated remote Runner hosts.
- Browser terminal, workspace files, upload/download, repository diff, and ports.
- Live HTTP preview through the bundled relay.
- Skills, MCP servers, notifications, images, environment variables, and quotas.
- React web, Electron desktop, and Expo mobile clients.

Commercial billing, payment, invitation rewards, community publishing, Apple
login, Git OAuth shortcuts, and enterprise-license extensions are disabled by
default because they require separately operated services. They are not needed
for the private development workflow above.

## Repository Layout

| Directory | Purpose |
|---|---|
| `backend/` | Go API, migrations, Taskflow, Orchestrator, preview, and E2E model |
| `frontend/` | Vite + React web console |
| `desktop/` | Electron desktop wrapper |
| `mobile/` | Expo/React Native client and self-hosted update server |
| `devbox/` | Source-built Linux development image |
| `deploy/` | Compose, E2E, VMware, offline build, installer, and verification tools |
| `docs/` | Current deployment, operation, integration, and release manuals |

## One-Click Offline Deployment

Build on Linux amd64 with Docker Buildx. Pin the four infrastructure images and
Docker bundle checksums in the ignored local configuration:

```bash
cp deploy/package/package.env.example deploy/package/package.env
bash deploy/package/build.sh --version v1.0.0
```

The output contains every application image and installation dependency:

```text
deploy/out/devloom-offline-linux-amd64.tgz
deploy/out/devloom-offline-linux-amd64.tgz.sha256
```

Install on an offline Linux amd64 server:

```bash
sha256sum -c devloom-offline-linux-amd64.tgz.sha256
tar -xzf devloom-offline-linux-amd64.tgz
cd devloom-offline-linux-amd64
sudo bash install.sh --host devloom.intra.example --admin-email admin@intra.example
```

The installer verifies the manifest, installs the bundled Docker runtime when
needed, imports all images, creates secrets and TLS material, starts Compose,
and runs deployment checks. Configure at least one model after login. A remote
development host can then use the readable installer served from
`/static/installer/<arch>/`.

See [offline deployment](./docs/DEPLOYMENT_OFFLINE_CN.md) and
[offline package build](./docs/OFFLINE_PACKAGE_BUILD_CN.md) for the complete
contract, upgrades, firewalls, and verification.

## Source Validation

```bash
(cd backend && go test ./...)
pnpm --dir frontend test
pnpm --dir frontend lint
pnpm --dir frontend build:online
bash -n deploy/offline/install.sh deploy/offline/preflight.sh deploy/offline/verify.sh
```

`deploy/e2e/run-linux.sh <VM_IP>` builds and starts the full source stack in a
Linux Docker host. `deploy/vmware/Invoke-DevLoomE2E.ps1` drives the repository's
VMware test environment. The enterprise E2E model in `backend/cmd/e2ellm`
generates, tests, starts, and publishes a persistent full-stack acceptance
project when an external model is unavailable.

## Documentation

- [Chinese README](./readme.cn.md)
- [Deployment guide](./docs/DEPLOYMENT_CN.md)
- [Offline deployment](./docs/DEPLOYMENT_OFFLINE_CN.md)
- [User guide](./docs/USER_GUIDE_CN.md)
- [Agent and workspace integration](./docs/AGENT_INTEGRATION_CN.md)
- [Enterprise end-to-end acceptance](./docs/ENTERPRISE_E2E_ACCEPTANCE_CN.md)
- [Runtime boundaries](./docs/OPEN_SOURCE_BOUNDARIES.md)
- [Branding](./docs/BRANDING.md)
- [GitHub Actions](./docs/GITHUB_ACTIONS.md)

## Security

Never commit model keys, Git tokens, database passwords, Runner credentials,
TLS private keys, signing material, or `deploy/package/package.env`. Use the
generated secrets and internal CA in production, keep infrastructure ports
private, and expose only the web endpoint and required Runner/preview routes.

## License

The source is distributed under [GNU AGPL-3.0](./LICENSE). Review the licenses
of bundled third-party images and frontend components before redistribution.
