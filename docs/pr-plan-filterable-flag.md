# PR Plan: Filterable Flag + Graceful Fallback

## Context

The `ghealth data list` command builds a date-range filter from `DataType.DefaultTimePath` for every data type. The Google Health API silently returns empty results for types whose filter field isn't actually supported. The CLI can't distinguish "no data in range" from "API rejected the filter," forcing users to the raw `ghealth api` escape hatch.

Battle-tested evidence comes from the `health` skill (`~/.hermes/skills/health/SKILL.md`), which documents every workaround discovered through real usage against a live Fitbit-synced account.

## Repository

- **Upstream**: `https://github.com/rudrankriyam/Google-Health-CLI`
- **Fork**: `https://github.com/awalker/Google-Health-CLI` (origin remote)
- **Local**: `/home/walke/Projects/Google-Health-CLI`
- **Remotes**: `origin` = fork, `upstream` = original

## Root Cause

`FilterFromRange()` in `internal/registry/registry.go:110-120` blindly builds a filter string from `DataType.DefaultTimePath` for every type. The registry has no concept of whether the API actually accepts filters for a given type.

## Known Broken Types

| Type | RecordType | DefaultTimePath | Has `list` op | Filter works? | Notes |
|---|---|---|---|---|---|
| `exercise` | Session | `civil_start_time` | yes | **no** — returns empty | Must use raw API, filter client-side |
| `sleep` | Session | `civil_start_time` | yes | **no** — returns empty | Must use raw API, filter client-side |
| `daily-resting-heart-rate` | Daily | `date` | yes | **no** — returns empty | Must use raw API, filter client-side |
| `total-calories` | Interval | `civil_start_time` | **no** | N/A | `UNSUPPORTED_DATA_TYPE_ACTION` — only rollup/dailyRollUp |

## Known Working Types

These accept `--from`/`--to` filters via `data list`:

- `weight` (Sample, `physical_time`)
- `heart-rate` (Sample, `physical_time`)
- `steps` (Interval, `civil_start_time`)
- `active-zone-minutes` (Interval, `civil_start_time`)
- `distance` (Interval, `civil_start_time`)
- `body-fat` (Sample, `physical_time`)
- `sedentary-period` (Interval, `civil_start_time`)

**Needs verification**: The remaining ~20 types. Best approach: test each against the live account before marking `Filterable` in the PR. A safe default is `Filterable: false` — opt-in confirmed types only.

## Additional Issues

### AZM Rollup Returns Zeros

`ghealth rollup daily active-zone-minutes` returns 0 for all days even when data exists. The raw endpoint only has sparse passively-measured minutes. Real AZM data lives in exercise sessions' `metricsSummary.activeZoneMinutes`. Likely an upstream API issue — document in the PR, don't try to fix in this PR.

### Date Format Inconsistency

Three filter modes exist:
1. `physical_time` fields → require `Z` suffix (`2026-05-20T00:00:00Z`)
2. `civil_start_time` fields → reject `Z` suffix (`2026-05-28T00:00:00`)
3. No filter supported → CLI should skip filter entirely

The CLI already handles mode 1 vs 2 correctly via `DefaultTimePath`. The gap is mode 3 — types with no filter support.

## Implementation Plan

### PR 1 — Filterable Flag + Graceful Fallback

**Changes:**

1. **`internal/registry/registry.go`**:
   - Add `Filterable bool` field to `DataType` struct
   - Set `Filterable: true` on confirmed-working types (weight, heart-rate, steps, distance, body-fat, active-zone-minutes, sedentary-period)
   - All others default to `false`
   - Add `FilterFromRange` guard: return `""` if `!dataType.Filterable`

2. **`cmd/run.go`** (`data` handler, ~line 516-551):
   - When `FilterFromRange` returns `""` but `--from`/`--to` were provided, print a stderr warning: `"Note: <type> does not support server-side date filtering. Returning all data."`
   - Catch `UNSUPPORTED_DATA_TYPE_ACTION` error for types without `list` op — suggest `ghealth rollup daily <type>` instead

3. **`internal/registry/registry_test.go`**:
   - Add test: `FilterFromRange` returns `""` for non-filterable types even when from/to are provided
   - Add test: all types with `Filterable: true` have a non-empty `DefaultTimePath`

4. **`internal/healthapi/client_test.go`**:
   - Add case: `data list` for a non-filterable type sends no `filter` query param
   - Add case: `UNSUPPORTED_DATA_TYPE_ACTION` error is surfaced with a helpful message

**Estimated scope**: ~150 lines changed across 4 files.

### PR 2 — Client-Side Date Filtering (follow-up)

- Add `--client-filter` flag or auto-detect empty results + warn
- Fetch all data, parse timestamps, filter in Go
- Eliminates the need for raw API workaround for exercise/sleep/RHR
- Larger scope, better as a separate PR

### PR 3 — AZM Rollup Investigation (follow-up)

- Investigate why rollup returns zeros
- Document findings
- May be an upstream API limitation

## How to Pick This Up

```bash
cd /home/walke/Projects/Google-Health-CLI

# Sync upstream
git fetch upstream
git checkout main && git merge upstream/main

# Create feature branch
git checkout -b feat/filterable-flag

# Implement PR 1 changes
# ...

# Test
go test ./...
go build ./...

# Push to fork
git push -u origin feat/filterable-flag

# Create PR
gh pr create --repo rudrankriyam/Google-Health-CLI \
  --title "feat: add Filterable flag, skip unsupported date filters" \
  --body-file docs/pr-body-filterable-flag.md
```

## Testing Against Live Account

```bash
# Verify auth
ghealth doctor

# Test non-filterable types (should return data without filter)
ghealth data list exercise --json
ghealth data list sleep --json
ghealth data list daily-resting-heart-rate --json

# Test with --from/--to on non-filterable (should warn, still return data)
ghealth data list exercise --from 2026-05-01 --to 2026-06-01 --json

# Test total-calories (should suggest rollup)
ghealth data list total-calories --json

# Test filterable types still work
ghealth data list weight --from 2026-05-01 --to 2026-06-01 --json
ghealth data list steps --from 2026-05-01 --to 2026-06-01 --json
```

## Files to Touch

| File | Change |
|---|---|
| `internal/registry/registry.go` | Add `Filterable` field, set on known-good types, guard `FilterFromRange` |
| `cmd/run.go` | Handle empty filter + unsupported action errors |
| `internal/registry/registry_test.go` | Test Filterable behavior |
| `internal/healthapi/client_test.go` | Test filter omission for non-filterable types |
| `docs/endpoint-coverage.md` | Update coverage table with Filterable column |
