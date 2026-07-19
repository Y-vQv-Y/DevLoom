/* global afterEach, expect, test */

const buildConfig = require('../../app.config');

const ENV_KEYS = [
  'EXPO_UPDATES_URL',
  'EXPO_PUBLIC_UPDATES_SERVER',
  'EXPO_PUBLIC_ENABLE_APPLE_AUTH',
  'IOS_BUNDLE_ID',
  'ANDROID_PACKAGE',
  'IOS_TEAM_ID',
];

const originalEnv = Object.fromEntries(ENV_KEYS.map((key) => [key, process.env[key]]));

afterEach(() => {
  for (const key of ENV_KEYS) {
    const value = originalEnv[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

test('maps native build and update environment variables into Expo config', () => {
  process.env.EXPO_UPDATES_URL = 'https://updates.example.test/manifest';
  process.env.EXPO_PUBLIC_UPDATES_SERVER = 'https://updates.example.test';
  process.env.EXPO_PUBLIC_ENABLE_APPLE_AUTH = 'true';
  process.env.IOS_BUNDLE_ID = 'test.devloom.ios';
  process.env.ANDROID_PACKAGE = 'test.devloom.android';
  process.env.IOS_TEAM_ID = 'TEAM123';

  const config = buildConfig();

  expect(config.extra.updatesServer).toBe('https://updates.example.test');
  expect(config.updates).toMatchObject({ enabled: true, url: 'https://updates.example.test/manifest' });
  expect(config.ios).toMatchObject({
    bundleIdentifier: 'test.devloom.ios',
    appleTeamId: 'TEAM123',
    usesAppleSignIn: true,
  });
  expect(config.android.package).toBe('test.devloom.android');
});
