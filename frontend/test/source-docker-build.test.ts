import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const dockerfile = readFileSync(new URL("../docker/Dockerfile.source", import.meta.url), "utf8");
const workspace = readFileSync(new URL("../pnpm-workspace.yaml", import.meta.url), "utf8");

test("source image install uses the pinned pnpm build allowlist", () => {
  assert.match(dockerfile, /pnpm install --frozen-lockfile/);
  assert.doesNotMatch(dockerfile, /--ignore-scripts|pnpm rebuild/);
  assert.match(workspace, /allowBuilds:\s+[\s\S]*esbuild: true\s+[\s\S]*msw: true/);
});
