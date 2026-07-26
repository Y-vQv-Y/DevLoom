import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import cn from "../src/i18n/resources/cn.ts";
import en from "../src/i18n/resources/en.ts";

const taskList = readFileSync(new URL("../src/pages/console/user/tasks.tsx", import.meta.url), "utf8");
const projectTaskList = readFileSync(
  new URL("../src/pages/console/user/project/overview/tasks-tab.tsx", import.meta.url),
  "utf8",
);

test("finished tasks distinguish completed and cancelled results", () => {
  for (const source of [taskList, projectTaskList]) {
    assert.match(source, /finish_reason === "cancelled"/);
    assert.match(source, /status\.completed/);
    assert.match(source, /status\.stopped/);
  }
});

test("task result labels are available in both languages", () => {
  assert.equal(cn.consoleTasks.status.completed, "已完成");
  assert.equal(en.consoleTasks.status.completed, "Completed");
  assert.equal(cn.projectOverview.tasks.status.completed, "已完成");
  assert.equal(en.projectOverview.tasks.status.completed, "Completed");
});
