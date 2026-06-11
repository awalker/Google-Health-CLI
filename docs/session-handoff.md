# Session Handoff: Google-Health-CLI Feature Development

## Status

The `dev` branch contains all completed features, is fully tested against the live Fitbit-synced account, and is installed to `~/.local/bin/ghealth` for daily use. Individual feature branches are pushed to the fork (`awalker/Google-Health-CLI`) and are clean, ready for upstream PR submission when you decide the soak period is over.

## What Was Done (Completed Features)

### 1. PR 1: Filterable Flag & Graceful Fallback
- Added `Filterable bool` to `DataType` struct.
- `FilterFromRange` returns `""` for non-filterable types, preventing silent empty results.
- Marked 7 confirmed-working types `Filterable: true`: `weight`, `heart-rate`, `steps`, `distance`, `body-fat`, `active-zone-minutes`, `sedentary-period`.
- `cmd/run.go`: stderr warning when `--from`/`--to` used on a non-filterable type without client-side support.
- Rewrote `UNSUPPORTED_DATA_TYPE_ACTION` error for `total-calories` to suggest `ghealth rollup daily total-calories`.
- Comprehensive registry and client tests added.

### 2. PR 2: Client-Side Date Filtering
- Added `ClientTimePath string` to `DataType` for dot-notation timestamp extraction.
- Created `internal/clientfilter` package: `ParseBound`, `ExtractTime`, `FilterDataPoints` (handles both RFC3339 strings and `{year, month, day}` objects).
- Added `ListAllDataPoints` to `healthapi.Client` for safe, auto-paginated fetching.
- `data list` now automatically fetches all pages and filters in Go for `exercise`, `sleep`, and `daily-resting-heart-rate` when `--from`/`--to` are provided. No warnings, no raw API escape hatch needed.

### 3. PR 3: AZM Rollup Investigation (Documentation Only)
- Discovered the "AZM rollup returns zeros" issue was a misinterpretation of the API response.
- The rollup *does* work and returns a nested zone breakdown object (`sumInFatBurnHeartZone`, `sumInCardioHeartZone`, `sumInPeakHeartZone`), which is actually *more* comprehensive than summing exercise sessions alone (includes passive minutes).
- Updated the `health` skill (`~/.hermes/skills/health/SKILL.md`) to reflect this truth and provide correct `jq` extraction paths. Removed outdated raw API workarounds.

### 4. PR 4: Explicit Date Help & Truncation Warnings
- Updated `--from`/`--to` flag descriptions to explicitly document the 3 date format modes (`YYYY-MM-DD` for rollups, `2006-01-02T15:04:05Z` for `physical_time`, `2006-01-02T15:04:05` without Z for `civil_start_time`).
- Added a `stderr` warning when a response is truncated by `--limit`, alerting users/agents to use `--page-token`, add filters, or increase `--limit`. Prevents silent agent hallucinations from incomplete data without risking runaway token burn.

### 5. PR 5: `ghealth types capabilities` Command
- Added `TypeCapability` struct and `Capabilities()` function to the registry.
- Provides a clean, low-token JSON/table summary of `list`, `rollup`, `serverFilter`, and `clientFilter` support for all 31 data types.
- Replaces the need for agents to parse massive registry dumps to plan queries.

### 6. PR 6: `--units` and `--flatten` Global Flags
- Added `--units imperial` flag: automatically calculates and injects `weightLbs` and `distanceMiles` alongside metric defaults.
- Added `--flatten` flag: collapses deeply nested rollup/list objects into flat, predictable structures (e.g., `{"date": "2026-06-09", "activeZoneMinutesTotal": 547}`).
- Centralized transformation logic in `internal/output/transform.go` to keep `cmd/run.go` clean.

## Live Test Results (2026-06-11)

| Command | Result |
|---|---|
| `ghealth data list exercise --from 2026-05-01 --to 2026-06-01` | ✅ Client-side filtered, no warning, correct sessions |
| `ghealth data list sleep --from 2026-06-01 --to 2026-06-08` | ✅ Client-side filtered, no warning |
| `ghealth data list daily-resting-heart-rate --from 2026-06-01 --to 2026-06-08` | ✅ Client-side filtered, no warning |
| `ghealth data list weight --from 2026-05-01T00:00:00Z --to 2026-06-01T00:00:00Z --units imperial` | ✅ Server-side filtered, returns `weightLbs` |
| `ghealth rollup daily active-zone-minutes --from 2026-06-09 --to 2026-06-10 --flatten` | ✅ Returns `{"date": "2026-06-09", "activeZoneMinutesTotal": 547}` |
| `ghealth data list exercise --limit 2` | ✅ Returns 2 items + `stderr` truncation warning |

## Branch State

- **`upstream/main`**: Original release state.
- **`dev`**: Current integration branch. Contains **all** features listed above. Installed to `~/.local/bin/ghealth` for your daily use.
- **Feature branches** (all pushed to `origin`):
  - `feat/filterable-flag`
  - `feat/client-side-filter` (branched from `feat/filterable-flag`)
  - `feat/azm-rollup-investigation` (documentation/skill updates only)
  - `feat/explicit-date-help`
  - `feat/types-capabilities`
  - `feat/units-and-flatten`

## When Ready to Open Upstream PRs

For each feature branch, rebase onto `upstream/main` and open the PR against the upstream repo:

```bash
cd /home/walke/Projects/Google-Health-CLI

# 1. Update feature branch
git checkout feat/filterable-flag
git fetch upstream
git rebase upstream/main
git push -f origin feat/filterable-flag

# 2. Open PR (NOTE: --repo flag is REQUIRED to target upstream, not your fork)
gh pr create --repo rudrankriyam/Google-Health-CLI \
  --title "feat: add Filterable flag, skip unsupported date filters" \
  --body-file docs/pr-plan-filterable-flag.md
```
*(Repeat the `gh pr create` step for each feature branch, adjusting the title and body as needed.)*

## Key Reminders
- Remote `origin` is configured as SSH (`git@github.com:awalker/Google-Health-CLI.git`). Upstream is HTTPS.
- The `--repo rudrankriyam/Google-Health-CLI` flag in `gh pr create` is mandatory; otherwise, GitHub CLI defaults to opening the PR on your fork.
- If you discover any new API quirks during your daily use of the `dev` build, update the `health` skill and the relevant feature branch before submitting the PR.
