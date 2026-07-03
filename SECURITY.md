# Security Policy

`ghealth` handles health data and OAuth tokens, so security reports are taken seriously.

## Supported Versions

Only the latest released version is supported.

## Reporting

Please report security issues privately by email to the repository owner. Do not open a public issue with token material, private health data, OAuth credentials, or API responses containing personal data.

## Local Secrets

By default `ghealth` stores the OAuth token as a plaintext JSON file (`token.json`, file mode `0600`) in the user config directory. Anyone with read access to that directory (or to an unencrypted backup of it) can read the token, so enable OS-level disk encryption such as FileVault (macOS), LUKS (Linux), or BitLocker (Windows).

On macOS you can opt in to storing the token in the Keychain instead of a file by setting `GHEALTH_TOKEN_STORAGE=keychain` before running `ghealth auth login`. The token is then kept as a generic password (service `ghealth`) managed by `/usr/bin/security`. If you switch backends after logging in, remove the old copy: `ghealth auth revoke --yes` deletes the token from whichever backend is currently selected.

Do not commit local config or token files.
