import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

import cn from "../src/i18n/resources/cn.ts"
import en from "../src/i18n/resources/en.ts"

const footerSource = readFileSync(new URL("../src/components/welcome/footer.tsx", import.meta.url), "utf8")
const terminalSource = readFileSync(new URL("../src/components/welcome/terminal-chrome.tsx", import.meta.url), "utf8")

test("欢迎页外壳提供 DevLoom 中英文资源", () => {
  assert.equal(cn.welcomeShell.footer.repository, "DevLoom GitHub 仓库")
  assert.equal(en.welcomeShell.footer.repository, "DevLoom GitHub Repository")
  assert.equal(cn.welcomeShell.footer.support, "问题与支持")
  assert.equal(en.welcomeShell.footer.releases, "Release Announcements")
  assert.match(cn.welcomeShell.footer.copyright, /DevLoom/)
  assert.match(en.welcomeShell.footer.copyright, /DevLoom/)
})

test("欢迎页资源链接来自统一品牌配置", () => {
  assert.match(footerSource, /BRAND\.documentationUrl/)
  assert.match(footerSource, /BRAND\.repositoryUrl/)
  assert.match(terminalSource, /BRAND\.announcementsUrl/)
  assert.match(terminalSource, /Dev<span[^>]*>Loom<\/span>/)
})
