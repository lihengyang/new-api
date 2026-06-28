# Seedance Mini Billing ModelRatio Baseline Mismatch

Date: 2026-06-28
Status: remediated; compensation completed
Customer documentation changed: no

## Finding

Seedance Mini billing semantics were inconsistent with Standard and Fast.
Mini resolver ratios were computed against the repository unit price while the
production/preflight Mini aliases used the family no-video base as
`ModelRatio`.

The corrected Scheme B semantics are:

```text
ModelRatio = family no-video base / repo unit
OtherRatio = current tier / family no-video base
effective ratio = ModelRatio * OtherRatio
```

For Mini:

- no-video effective ratio: `1.75 * 1.0 = 1.75`
- video-input effective ratio: `1.75 * (0.0021 / 0.0035) = 1.05`

## Root Cause

The Mini resolver originally used repo-unit ratios while production/preflight
Mini aliases used family no-video `ModelRatio`. With Mini alias
`ModelRatio = 1.75`, the old resolver double-applied the Mini no-video base.

## Contributing Factors

- No invariant checked that `ModelRatio * OtherRatio == official_price / repo_unit`.
- Final settlement validation checked existence/settlement, not the exact
  expected final quota.
- The production-only Mini alias was not covered by smoke before the incident.
- Codex App key handling confusion delayed smoke because Terminal-exported
  environment variables were treated as if they would carry into Codex App.

## Impact

- Original affected Mini SUCCESS tasks: `4`.
- Affected task suffixes: `4kamY0Li`, `7ydNwLxX`, `V33AhAoZ`, `YiBgqMZ0`.
- Correction quota: `364016`.
- USD equivalent at `quota / 500000`: `$0.728032`.
- Post-deploy smoke tasks were excluded from compensation.

## Remediation

- Scheme B hotfix deployed to production as
  `new-api:seedance-mini-billing-scheme-b-rc1`.
- Hotfix commit:
  `9ef110f928f40a88bf426432db08807b9edf89cb`.
- Preflight exact billing validated for Mini no-video and Mini video-input.
- Production exact billing validated for Mini no-video and Mini video-input.
- Mini 1080p/4K and Fast 4K rejection paths validated before task creation,
  billing, and upstream calls.
- Token quota credit completed for the two verified live token targets only:

| token_hash | correction quota | after remain_quota | after used_quota |
|---|---:|---:|---:|
| `8920eb70817c` | `257457` | `49001158` | `15998842` |
| `b0b206db9656` | `106559` | `10251889` | `26843011` |

## Prevention

- Add family-base billing invariant tests for every Seedance family and tier.
- Require preflight/prod alias parity checks before production smoke.
- Include production-only aliases in safe smoke coverage before customer
  availability.
- Require exact final billing invariant checks from `actual_quota`,
  settlement logs, final net quota, or upstream usage token evidence.
- Run the Keychain balance gate before any paid smoke.
- In Codex App, create/read the smoke key from the Codex App macOS Keychain
  context rather than relying on Terminal-exported environment variables.
