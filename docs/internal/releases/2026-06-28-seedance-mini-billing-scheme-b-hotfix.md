# Seedance Mini Billing Scheme B Hotfix Finding

Date: 2026-06-28
Status: local hotfix prepared
Production connected: no
Production deployed: no
Customer documentation changed: no

## Summary

Scheme B aligns Seedance Mini billing semantics with Standard and Fast:

```text
ModelRatio = family no-video base / repo unit
OtherRatio = current tier / family no-video base
effective ratio = ModelRatio * OtherRatio
```

Mini keeps the official unit prices:

- no-video: `0.0035 USD / K tokens`
- video: `0.0021 USD / K tokens`

Mini production/preflight aliases should keep `ModelRatio = 0.0035 / 0.0020 = 1.75`.
The resolver should return `OtherRatio = 1.0` for Mini no-video and
`OtherRatio = 0.0021 / 0.0035 = 0.6` for Mini video.

Expected effective ratios:

- Mini no-video: `1.75 * 1.0 = 1.75`
- Mini video: `1.75 * 0.6 = 1.05`
- Fast video remains: `2.80 * (0.0033 / 0.0056) = 1.65`

## Root Cause

The Mini resolver used repo-unit ratios while production/preflight Mini aliases
used the family-base `ModelRatio`. That double-applied the Mini no-video base
when the alias `ModelRatio` was configured as `1.75`.

## Read-Only Audit Blast Radius

- Production Mini tasks in window: `4`
- SUCCESS settled affected: `4`
- Correction quota: `364016`

## Missing Gates

- No exact final quota invariant.
- No `ModelRatio * OtherRatio` invariant.
- Production-only alias was not covered by smoke.

## Local Hotfix Scope

- Update the Seedance billing resolver only; no production config, DB, or
  customer configuration changes.
- Add resolver invariants for Mini no-video, Mini video, and Fast video.
- Add Mini precharge regression coverage using `ModelRatio = 1.75`.
- Add Mini-vs-Fast final quota ordering coverage for the same tokens and group
  ratio.
