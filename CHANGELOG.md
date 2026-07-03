# Changelog

## 1.0.1

- Added fake-server tests for all 18 documented Google Health v4 REST methods.
- Added request payload, query parameter, response, API error, and malformed-response coverage.
- Added endpoint coverage documentation.
- Added missing client options for `updateMask`, subscriber pagination and force deletion, `dataSourceFamily`, and partial TCX export.
- Fixed CLI rollup request bodies to send documented `range` payloads instead of filter strings.
- Added nutrition data types to the registry.
- Persisted refreshed OAuth tokens so long-running sessions stay logged in.
- Added an opt-in macOS Keychain token storage backend (`GHEALTH_TOKEN_STORAGE=keychain`); the plaintext file remains the default.
- Fixed the OAuth callback server so duplicate callback requests can never block the handler, and gave server shutdown a 5-second timeout.
- Fixed `ghealth auth login` to surface config save failures instead of silently ignoring them.
- Fixed `ghealth help` and `ghealth version` to keep working when the config file is corrupt.
- Fixed civil date parsing to reject out-of-range months and days.
- Added tests for `internal/config` (round-trip, defaults, corrupt file).
- Removed unused `auth.AuthURL`, `healthapi.IsNotLoggedIn`, and `healthapi.IsAPIError` helpers and a dead nil-check on `oauth2.NewClient`.

## 1.0.0

- Initial `ghealth` CLI workbench.
- Added OAuth login, local token handling, and setup checks.
- Added registry for all 31 documented Google Health data types.
- Added registry for all 18 documented v4 REST methods.
- Added data point, rollup, profile, settings, identity, subscriber, and raw API commands.
- Added agent manifest/capabilities/schema commands.
- Added GoReleaser and GitHub Actions CI/CD.
