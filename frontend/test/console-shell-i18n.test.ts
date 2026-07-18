import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

import cn from "../src/i18n/resources/cn.ts"
import en from "../src/i18n/resources/en.ts"

const userSidebar = readFileSync(new URL("../src/components/console/nav/user-sidebar.tsx", import.meta.url), "utf8")

test("控制台侧边栏使用统一品牌配置", () => {
  assert.match(userSidebar, /BRAND\.supportUrl/)
  assert.match(userSidebar, /BRAND\.repositoryUrl/)
  assert.match(userSidebar, /t\(brandSubtitleKey\)/)
})

test("控制台外壳提供 DevLoom 中英文资源", () => {
  assert.equal(cn.consoleShell.sidebar.brandSubtitle, "DevLoom 开源平台")
  assert.equal(en.consoleShell.sidebar.brandSubtitle, "DevLoom Open Source Platform")
  assert.equal(cn.consoleShell.sidebar.globalBrandSubtitle, "DevLoom 开源平台")
  assert.equal(en.consoleShell.sidebar.globalBrandSubtitle, "DevLoom Open Source Platform")
  assert.equal(cn.consoleShell.sidebar.currentVersion, "当前版本")
  assert.equal(en.consoleShell.sidebar.settings, "Settings")
})
