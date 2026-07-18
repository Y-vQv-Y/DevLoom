const repositoryUrl = "https://github.com/Y-vQv-Y/DevLoom"
const ownerUrl = "https://github.com/Y-vQv-Y"
const issuesUrl = `${repositoryUrl}/issues`

function publicUrl(name: string, fallback: string) {
  const value = import.meta.env[name]?.trim()
  return value || fallback
}

export const BRAND = {
  name: "DevLoom",
  repositoryUrl,
  ownerUrl,
  documentationUrl: publicUrl("VITE_DOCS_URL", `${repositoryUrl}#readme`),
  websiteUrl: publicUrl("VITE_PUBLIC_SITE_URL", repositoryUrl),
  announcementsUrl: publicUrl("VITE_ANNOUNCEMENT_URL", `${repositoryUrl}/releases`),
  forumUrl: publicUrl("VITE_FORUM_URL", issuesUrl),
  consultationUrl: publicUrl("VITE_CONSULTATION_URL", `${issuesUrl}/new`),
  companyUrl: publicUrl("VITE_COMPANY_URL", ownerUrl),
  communityUrl: publicUrl("VITE_COMMUNITY_URL", issuesUrl),
  supportUrl: publicUrl("VITE_SUPPORT_URL", issuesUrl),
  releasesUrl: `${repositoryUrl}/releases`,
} as const
