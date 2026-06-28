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
- Customer sample evidence could be mistaken for the full affected production
  population without a production task-window audit.
- Compensation target reconciliation initially had to distinguish placeholder
  token hashes from live billing tokens.
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
- Internal docs/audit commit:
  `9dad9b4b00766add641299853b1a5542221d1bc2`.
- Preflight exact billing validated for Mini no-video and Mini video-input.
- Production exact billing validated for Mini no-video and Mini video-input.
- Mini 1080p/4K and Fast 4K rejection paths validated before task creation,
  billing, and upstream calls.
- Customer Mini usage was manually paused outside system configuration; no
  system access toggle was changed or restored.
- Token quota credit completed for the two verified live token targets only:

| token_hash | correction quota | after remain_quota | after used_quota |
|---|---:|---:|---:|
| `8920eb70817c` | `257457` | `49001158` | `15998842` |
| `b0b206db9656` | `106559` | `10251889` | `26843011` |

## Prevention

- Add family-base billing invariant tests for every Seedance family and tier.
- For terminal SUCCESS tasks, require
  `actual_quota = floor(tokens * ModelRatio * GroupRatio * OtherRatio)` from
  settlement evidence, not just settlement existence.
- For every new tier, require
  `ModelRatio * OtherRatio = official tier price / repo unit price`.
- Require preflight/prod alias parity checks before production smoke.
- Include production-only aliases in safe smoke coverage before customer
  availability.
- Require no-video exact billing smoke and video-input exact billing smoke when
  the tier supports video input and an approved safe reference asset exists.
- Require unsupported resolution rejection before task creation, billing
  reservation, and upstream submission.
- Require Fast and Standard regression coverage for unaffected families.
- Require post-deploy blast-radius audit from the production task window; a
  customer sample count is not the affected population count.
- Reconcile compensation targets to live billing token, user, and group before
  credit. Placeholder token hashes and smoke tasks are not compensation targets.
- Require exact final billing invariant checks from `actual_quota`,
  settlement logs, final net quota, or upstream usage token evidence.
- Run the Keychain balance gate before any paid smoke.
- In Codex App, create/read the smoke key from the Codex App macOS Keychain
  context rather than relying on Terminal-exported environment variables.
- Use Keychain account `henrytest` and service `lsf-henrytest-api-key`.
- Use a macOS hidden dialog only for key entry or replacement; never paste the
  key into chat, use hidden PTY prompts, write temporary secret files, or guess
  tokens from the DB.
- Treat a `/v1/billing/balance` failure as a key, client, or tenant
  configuration gate blocker unless independent evidence proves runtime failure.
