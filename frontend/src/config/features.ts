function enabled(name: string) {
  return import.meta.env[name]?.trim().toLowerCase() === "true"
}

// Commercial APIs are not part of the open-source backend. Keep their UI off
// unless a deployment provides compatible billing services explicitly.
export const COMMERCIAL_BILLING_ENABLED = enabled("VITE_ENABLE_COMMERCIAL_BILLING")

// License APIs belong to an external enterprise extension and are not present
// in this repository's Go backend.
export const ENTERPRISE_LICENSE_ENABLED = enabled("VITE_ENABLE_ENTERPRISE_LICENSE")

// The open-source backend has no playground publishing/listing endpoints.
export const COMMUNITY_PLAYGROUND_ENABLED = enabled("VITE_ENABLE_COMMUNITY_PLAYGROUND")

// OAuth shortcuts for Git identities require authorization URL endpoints or
// external GitHub Apps. Manual access-token identities remain available.
export const GIT_IDENTITY_OAUTH_ENABLED = enabled("VITE_ENABLE_GIT_IDENTITY_OAUTH")

// Automatic project review requires an external Git review service. The
// open-source backend exposes no auto-review route.
export const AUTO_REVIEW_ENABLED = enabled("VITE_ENABLE_AUTO_REVIEW")
