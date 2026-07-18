const baseConfig = require('./app.json').expo;

module.exports = () => {
  const projectId = process.env.EXPO_PROJECT_ID?.trim();
  const owner = process.env.EXPO_OWNER?.trim();
  const updatesUrl = process.env.EXPO_UPDATES_URL?.trim();
  const appleAuthEnabled = process.env.EXPO_PUBLIC_ENABLE_APPLE_AUTH?.trim().toLowerCase() === 'true';

  return {
    ...baseConfig,
    ...(owner ? { owner } : {}),
    ios: {
      ...baseConfig.ios,
      bundleIdentifier: process.env.IOS_BUNDLE_ID?.trim() || baseConfig.ios.bundleIdentifier,
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
      ...(projectId ? { eas: { projectId } } : {}),
    },
  };
};
