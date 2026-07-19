# GitHub Actions Build and Release Guide

This repository builds on GitHub-hosted runners. The mobile workflow uses Expo Prebuild only to generate native projects, then calls Gradle and Xcode directly. It does not use Expo cloud builds or an Expo project ID.

## Workflows

- `build.yml` runs tests, static checks, web builds, and Docker build checks.
- `electron-release.yml` runs for `v*` tags or from `Actions > Release > Run workflow`.
- Set `publish=true` for a manual run to publish GHCR images and a GitHub Release. `publish=false` keeps the generated artifacts in Actions only.
- Set `mobile=true` for a manual run, or set the repository variable `ENABLE_MOBILE_RELEASE=true` to include mobile jobs in tag releases. When disabled, both mobile jobs are `skipped` and the rest of the release continues.

## Repository Variables

Keep the existing web variables documented in the workflow. Mobile builds use these optional variables:

Commercial, enterprise-license, community-playground, Git OAuth, and Apple-authentication flags default to `false`. `VITE_ENABLE_AUTO_REVIEW` also defaults to `false`; enable it only after configuring the backend review model, image, runtime host, and repository webhook permissions.

| Variable | Default | Purpose |
|---|---|---|
| `ENABLE_MOBILE_RELEASE` | `false` | Include Android and iOS jobs in releases |
| `DEVLOOM_MOBILE_API_URL` | empty | API URL compiled into the mobile app |
| `DEVLOOM_EXPO_UPDATES_URL` | empty | Self-hosted Expo Updates manifest URL |
| `DEVLOOM_UPDATES_SERVER` | empty | Application update service metadata URL |
| `VITE_DEFAULT_SKILL_IDS` | empty | Comma-separated default Web skill IDs installed in this deployment |
| `EXPO_PUBLIC_DEFAULT_SKILL_IDS` | empty | Comma-separated default mobile skill IDs installed in this deployment |
| `ANDROID_PACKAGE` | `io.github.yvqvy.devloom` | Android application ID |
| `IOS_BUNDLE_ID` | `io.github.yvqvy.devloom` | iOS bundle identifier |
| `IOS_TEAM_ID` | empty | Apple Developer Team ID |
| `IOS_EXPORT_METHOD` | `ad-hoc` | Xcode export method for a device IPA |

`IOS_EXPORT_METHOD=ad-hoc` requires a profile containing the target device UDIDs. Use `app-store-connect` for an App Store/TestFlight distribution profile when appropriate.

## Repository Secrets

Android signing is optional. If any Android signing secret is present, all four are required:

| Secret | Purpose |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | Base64 encoded upload keystore |
| `ANDROID_KEYSTORE_PASSWORD` | Keystore password |
| `ANDROID_KEY_ALIAS` | Upload key alias |
| `ANDROID_KEY_PASSWORD` | Upload key password |

With these secrets, the job publishes a signed APK and AAB. Without them, it publishes only a clearly named Debug APK.

iOS device IPA builds require all of the following:

| Secret | Purpose |
|---|---|
| `IOS_CERTIFICATE_P12_BASE64` | Base64 encoded Apple distribution certificate |
| `IOS_CERTIFICATE_PASSWORD` | P12 export password |
| `IOS_PROVISIONING_PROFILE_BASE64` | Base64 encoded device distribution profile |
| `IOS_TEAM_ID` | Optional fallback if not stored as a repository variable |

The workflow imports these files into a temporary keychain, archives the generated Xcode project, exports an IPA, and deletes the keychain and profile before finishing. Apple Developer signing is mandatory for a physical iPhone; a simulator app cannot be installed on a phone.

If no iOS signing input is configured, the same job builds and publishes `devloom-ios-simulator-<version>.app.zip` instead. This is a valid unsigned iOS Simulator package for local simulator testing, not a phone-installable IPA. Partial signing configuration fails early so a release cannot silently publish a misleading device package.

## Preparing Signing Files

Generate an Android upload key once and keep it backed up:

```bash
keytool -genkeypair -v -keystore devloom-upload.keystore -alias devloom \
  -keyalg RSA -keysize 2048 -validity 10000
base64 -w 0 devloom-upload.keystore
```

Export the Apple distribution certificate as a password-protected `.p12` from Keychain Access. Download the matching Ad Hoc provisioning profile from the Apple Developer portal. Encode both files without changing their contents:

```bash
base64 -w 0 certificate.p12
base64 -w 0 DevLoom.mobileprovision
```

On PowerShell, use `[Convert]::ToBase64String([IO.File]::ReadAllBytes('file'))` instead. Store the resulting strings only as GitHub Actions Secrets.

## CI and Release Artifacts

The `CI` workflow validates and uploads the Linux backend binary, online/offline frontend builds, mobile Web export, and regenerated backend API artifacts. It also builds the frontend, backend, and ingress containers without pushing them.

The `Release` workflow produces these non-mobile assets:

```text
devloom-server-linux-amd64
devloom-server-linux-arm64
devloom-server-windows-amd64.exe
devloom-server-darwin-amd64
devloom-server-darwin-arm64
devloom-frontend-online.tar.gz
devloom-frontend-offline.tar.gz
Windows portable/NSIS packages
macOS x64/arm64 DMG packages
devloom-source.tar.gz
```

When `publish=true` or a valid `v*` tag is pushed, the workflow also publishes `devloom-{frontend,backend,ingress}` images to `ghcr.io/y-vqv-y/`.

Successful mobile jobs upload `mobile-android` and `mobile-ios` artifacts. The release job downloads them and includes them in GitHub Release assets:

```text
devloom-android-<version>.apk
devloom-android-<version>.aab
devloom-android-debug-<version>.apk   # when Android signing is absent
devloom-ios-<version>.ipa
devloom-ios-simulator-<version>.app.zip   # when Apple signing is absent
```

Create a release with:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Do not commit generated `mobile/android` or `mobile/ios` directories, certificates, profiles, keystores, passwords, or service-account files. Review `mobile/app.config.js` and this workflow together when changing bundle IDs, package IDs, permissions, or native plugins.

## Troubleshooting

- `IOS_CERTIFICATE_P12_BASE64` or profile errors mean the corresponding GitHub Secret is missing or invalid.
- A provisioning error usually means the profile bundle ID, Team ID, export method, certificate type, or registered iPhone UDID does not match.
- Android release signing errors usually mean one of the four Android secrets is wrong or the alias is not present in the keystore.
- Any workflow requesting Expo cloud credentials is stale and should be removed.

## Backend Runtime Check

The GitHub build validates source and packages artifacts; it does not provide the runtime service. A deployment must set `TASKFLOW_SERVER` to an absolute internal `http://` or `https://` endpoint before starting the Go backend. If it is missing, startup fails with a clear external-runtime configuration error rather than starting a partially working server. For private deployments, install Taskflow/runner and its host agent on the internal network, then register the host and images from the web or mobile client.
