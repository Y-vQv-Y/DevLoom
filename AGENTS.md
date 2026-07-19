# Repository Guidelines

## Project Structure & Module Organization
`backend/` contains the Go service: business logic in `biz/`, shared infrastructure in `pkg/`, generated Ent models in `db/`, and sample config in `config/server/config.yaml.example`. `frontend/` is the Vite + React web app, with source in `src/`, assets in `public/`, and regression tests in `test/`. `desktop/` packages the frontend with Electron. `mobile/` is the Expo/React Native client, with routes in `app/`, shared logic in `src/`, and assets in `assets/`.

## Build, Test, and Development Commands
- `pnpm --dir frontend dev:online` starts the web UI locally.
- `pnpm --dir frontend build:online` creates the production web build.
- `pnpm --dir frontend lint` runs the frontend ESLint config.
- `pnpm --dir desktop electron:dev` launches Electron against the frontend dev server.
- `npm --prefix mobile start` starts Expo; `npm --prefix mobile test` runs Jest. Release mobile builds run natively on GitHub-hosted Ubuntu (Android) and macOS (iOS device IPA).
- `go test ./...` from `backend/` runs backend tests.
- `make swag` and `make generate` from `backend/` refresh Swagger and Ent-generated code after API or schema changes.

## Coding Style & Naming Conventions
TypeScript is strict in `frontend/` and `mobile/`; use the existing `@/` import alias. Match local formatting: frontend files generally use single quotes and minimal semicolons, mobile files commonly keep semicolons, and Go code must stay `gofmt`-clean. Use PascalCase for React components, `useX` for hooks, and `*.test.ts` or `*.test.tsx` for tests. Do not hand-edit `backend/db/` generated files.

## Testing Guidelines
Add backend tests next to the changed Go package. Keep frontend tests in `frontend/test/` using `node:test`; mobile tests belong in `__tests__` folders or adjacent `*.test.ts(x)` files and run with Jest. No repo-wide coverage gate is visible, so add focused regression tests for new behavior.

## Commit & Pull Request Guidelines
Recent history favors short conventional subjects such as `fix: ...`, `feat: ...`, and scoped forms like `test(lifecycle): ...`. Keep commit messages imperative and specific. Pull requests should summarize user impact, config or migration changes, and UI screenshots when relevant. Link issues when available.

## Security & Configuration Tips
Use `backend/config/server/config.yaml.example` for local config. Never commit `admin_token`, database or Redis passwords, OAuth credentials, Apple certificates, provisioning profiles, Android keystores, or signing passwords. Mobile signing uses GitHub Secrets, not Expo/EAS authentication. Review `.github/workflows/` when changing release packaging or versioning.
