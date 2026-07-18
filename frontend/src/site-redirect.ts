export type SiteRegion = "cn" | "global"

type SiteLocation = Pick<Location, "hostname" | "pathname" | "search" | "hash">

export type SiteRedirectPrompt = {
  currentHost: string
  currentRegion: SiteRegion
  targetHost: string
  targetRegion: SiteRegion
  targetUrl: string
  detectedLanguages: string[]
}

// DevLoom does not assume any public domains. Deployments that need regional
// routing should implement it at the ingress layer with domains they control.
export function getSiteRedirectUrl(
  _location: SiteLocation | null = getCurrentLocation(),
  _languages: readonly string[] = getBrowserLanguages(),
): string | null {
  return null
}

export function getSiteRedirectPrompt(
  _location: SiteLocation | null = getCurrentLocation(),
  _languages: readonly string[] = getBrowserLanguages(),
): SiteRedirectPrompt | null {
  return null
}

export function languagesContainChinese(languages: readonly string[]): boolean {
  return languages
    .map((language) => language.trim())
    .filter(Boolean)
    .slice(0, 2)
    .some((language) => /^zh($|[-_])/i.test(language))
}

function getCurrentLocation(): SiteLocation | null {
  return typeof window === "undefined" ? null : window.location
}

function getBrowserLanguages(): readonly string[] {
  if (typeof navigator === "undefined") return []
  return navigator.languages.length > 0 ? navigator.languages : [navigator.language]
}
