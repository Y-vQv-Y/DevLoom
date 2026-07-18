import assert from "node:assert/strict"
import test from "node:test"

import { getSiteRedirectPrompt, getSiteRedirectUrl, languagesContainChinese } from "../src/site-redirect.ts"

test("未配置自有域名时不执行跨站跳转", () => {
  const location = { hostname: "example.com", pathname: "/login", search: "", hash: "" }
  assert.equal(getSiteRedirectUrl(location, ["zh-CN"]), null)
  assert.equal(getSiteRedirectPrompt(location, ["en-US"]), null)
})

test("语言检测保留中英文区域判断能力", () => {
  assert.equal(languagesContainChinese(["zh-CN", "en-US"]), true)
  assert.equal(languagesContainChinese(["en-US", "ja-JP"]), false)
})
