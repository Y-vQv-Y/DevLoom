import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), "utf8")
}

const files = {
  brand: source("../src/config/brand.ts"),
  banner: source("../src/components/welcome/banner.tsx"),
  downloads: source("../src/components/welcome/downloads.tsx"),
  terminal: source("../src/components/welcome/terminal-chrome.tsx"),
  selfHosting: source("../src/pages/self-hosting.tsx"),
}

test("品牌链接使用 Y-vQv-Y/DevLoom 作为默认入口", () => {
  assert.match(files.brand, /https:\/\/github\.com\/Y-vQv-Y\/DevLoom/)
  assert.match(files.brand, /VITE_DOCS_URL/)
  assert.match(files.brand, /VITE_PUBLIC_SITE_URL/)
  assert.match(files.brand, /VITE_ANNOUNCEMENT_URL/)
  assert.match(files.brand, /VITE_FORUM_URL/)
  assert.match(files.brand, /VITE_CONSULTATION_URL/)
  assert.match(files.brand, /VITE_COMPANY_URL/)
  assert.match(files.brand, /VITE_COMMUNITY_URL/)
  assert.match(files.brand, /VITE_SUPPORT_URL/)
})

test("重点欢迎页组件不再硬编码品牌外链", () => {
  assert.match(files.banner, /BRAND\.repositoryUrl/)
  assert.match(files.downloads, /BRAND\.releasesUrl/)
  assert.match(files.terminal, /BRAND\.documentationUrl/)
  assert.match(files.terminal, /BRAND\.forumUrl/)
  assert.match(files.terminal, /BRAND\.companyUrl/)
  assert.match(files.terminal, /BRAND\.consultationUrl/)
  assert.match(files.terminal, /BRAND\.communityUrl/)
  assert.match(files.selfHosting, /BRAND\.documentationUrl/)
  assert.match(files.selfHosting, /BRAND\.consultationUrl/)
  assert.match(files.selfHosting, /BRAND\.repositoryUrl/)
  assert.doesNotMatch(files.downloads, /client\.href/)
  assert.match(files.terminal, /BRAND\.websiteUrl/)
  assert.match(files.selfHosting, /BRAND\.releasesUrl/)
})
