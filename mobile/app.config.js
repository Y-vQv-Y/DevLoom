const baseConfig = require('./app.json').expo;

module.exports = () => {
  const updatesUrl = process.env.EXPO_UPDATES_URL?.trim();
  const appleAuthEnabled = process.env.EXPO_PUBLIC_ENABLE_APPLE_AUTH?.trim().toLowerCase() === 'true';

  return {
    ...baseConfig,
    ios: {
      ...baseConfig.ios,
      bundleIdentifier: process.env.IOS_BUNDLE_ID?.trim() || baseConfig.ios.bundleIdentifier,
      ...(process.env.IOS_TEAM_ID?.trim() ? { appleTeamId: process.env.IOS_TEAM_ID.trim() } : {}),
      usesAppleSignIn: appleAuthEnabled,
    },
    android: {
      ...baseConfig.android,
      package: process.env.ANDROID_PACKAGE?.trim() || baseConfig.android.package,
    },
    updates: {
      ...baseConfig.updates,
      ...(updatesUrl ? { enabled: true, url: updatesUrl } : {}),
    },
    extra: {
      ...baseConfig.extra,
      ...(process.env.EXPO_PUBLIC_UPDATES_SERVER?.trim()
        ? { updatesServer: process.env.EXPO_PUBLIC_UPDATES_SERVER.trim() }
        : {}),
    },
  };
};
