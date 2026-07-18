import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

import cn from "../src/i18n/resources/cn.ts"
import en from "../src/i18n/resources/en.ts"

const legalPageSource = readFileSync(new URL("../src/pages/legal-page-i18n.tsx", import.meta.url), "utf8")

test("法律页面提供中英文资源", () => {
  assert.equal(cn.legalPages.privacy.title, "隐私政策")
  assert.equal(en.legalPages.privacy.title, "Privacy Policy")
  assert.equal(cn.legalPages.userAgreement.title, "用户协议")
  assert.equal(en.legalPages.userAgreement.title, "User Agreement")
})

test("法律页面仅使用 DevLoom 官方仓库和支持入口", () => {
  assert.match(legalPageSource, /BRAND\.repositoryUrl/)
  assert.match(legalPageSource, /BRAND\.supportUrl/)
  assert.equal(cn.legalPages.privacy.contact.repository, "DevLoom GitHub 仓库")
  assert.equal(cn.legalPages.privacy.contact.support, "DevLoom 问题与支持")
  assert.equal(en.legalPages.userAgreement.contact.repository, "DevLoom GitHub Repository")
  assert.equal(en.legalPages.userAgreement.contact.support, "DevLoom Issues and Support")
})
