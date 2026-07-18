# Repository Guidelines

## Project Structure & Module Organization
`backend/` contains the Go service: business logic in `biz/`, shared infrastructure in `pkg/`, generated Ent models in `db/`, and sample config in `config/server/config.yaml.example`. `frontend/` is the Vite + React web app, with source in `src/`, assets in `public/`, and regression tests in `test/`. `desktop/` is the Electron wrapper that packages the frontend. `mobile/` is the Expo/React Native client, with routes in `app/`, shared logic in `src/`, and assets in `assets/`. Design notes live under `docs/superpowers/`.

## Build, Test, and Development Commands
- `pnpm --dir frontend dev:online` starts the web UI locally.
- `pnpm --dir frontend build:online` creates the production web build.
- `pnpm --dir frontend lint` runs the frontend ESLint config.
- `pnpm --dir desktop electron:dev` launches Electron against the frontend dev server.
- `npm --prefix mobile start` starts Expo; `npm --prefix mobile test` runs Jest.
- `go test ./...` from `backend/` runs backend tests.
- `make swag` and `make generate` from `backend/` refresh Swagger and Ent-generated code after API or schema changes.

## Coding Style & Naming Conventions
TypeScript is strict in `frontend/` and `mobile/`; prefer the existing `@/` import alias for app code. Match the local style of each package: frontend files generally use single quotes and minimal semicolons, mobile files commonly keep semicolons, and Go code must stay `gofmt`-clean. Use PascalCase for React components, `useX` for hooks, and `*.test.ts` or `*.test.tsx` for tests. Do not hand-edit generated files under `backend/db/`.

## Testing Guidelines
Add backend tests next to the Go package being changed. Keep frontend tests in `frontend/test/`; they use `node:test` and focus on source-level regressions and i18n wiring. Mobile tests belong in `__tests__` folders or adjacent `*.test.ts(x)` files and run with Jest. There is no visible repo-wide coverage gate, so add focused tests for new behavior and regressions.

## Commit & Pull Request Guidelines
Recent history favors short conventional subjects such as `fix: ...`, `feat: ...`, and scoped forms like `test(lifecycle): ...`. Keep commit messages imperative and specific. Pull requests should summarize user impact, list touched surfaces (`backend`, `frontend`, `desktop`, `mobile`), mention config or migration changes, and include screenshots for UI work. Link issues when available.

## Security & Configuration Tips
Use `backend/config/server/config.yaml.example` as the starting point for local config, and never commit `admin_token`, database or Redis passwords, OAuth credentials, EAS tokens, or signing material. If you change release packaging or versioning, review the workflows in `.github/workflows/` before merging.
