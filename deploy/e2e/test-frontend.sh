#!/bin/sh
set -eu
cd /repo/frontend
corepack enable
pnpm config set registry "${NPM_REGISTRY:-https://registry.npmjs.org}"
pnpm install --frozen-lockfile --ignore-scripts
pnpm rebuild esbuild msw
pnpm test
pnpm lint
pnpm build:online
