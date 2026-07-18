// Expo inlines EXPO_PUBLIC_* variables at build time. The open-source default
// avoids calling wallet, recharge, check-in, and invitation APIs that are absent.
export const COMMERCIAL_BILLING_ENABLED =
  process.env.EXPO_PUBLIC_ENABLE_COMMERCIAL_BILLING?.trim().toLowerCase() === 'true';

// The Go backend in this repository does not implement Apple sign-in or
// account deletion. Enable both only with a compatible backend extension.
export const APPLE_AUTH_ENABLED =
  process.env.EXPO_PUBLIC_ENABLE_APPLE_AUTH?.trim().toLowerCase() === 'true';

// OAuth shortcuts need authorization URL endpoints that are not included in
// the open-source backend. Manual access-token identities remain available.
export const GIT_IDENTITY_OAUTH_ENABLED =
  process.env.EXPO_PUBLIC_ENABLE_GIT_IDENTITY_OAUTH?.trim().toLowerCase() === 'true';
