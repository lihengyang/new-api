# Seedance Mini Billing Scheme B Hotfix and Compensation Record

Date: 2026-06-28
Status: production hotfix deployed and compensation completed
Preflight deployed: yes
Production deployed: yes
Quota credit completed: yes
Customer documentation changed: no
Mini customer access restored: no

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

Final quota formulas under Scheme B:

```text
Mini no-video = floor(tokens * ModelRatio * groupRatio * 1.0)
Mini video    = floor(tokens * ModelRatio * groupRatio * (0.0021 / 0.0035))
```

## Root Cause

The Mini resolver used repo-unit ratios while production/preflight Mini aliases
used the family-base `ModelRatio`. That double-applied the Mini no-video base
when the alias `ModelRatio` was configured as `1.75`.

## Hotfix Traceability

- Release branch pushed: `origin/release/seedance-mini-v1`
- Commit: `9ef110f928f40a88bf426432db08807b9edf89cb`
- Commit subject: `fix: align seedance mini billing ratios with family base`
- Image tag: `new-api:seedance-mini-billing-scheme-b-rc1`
- Image ID prefix: `c991ffe082dc`
- Image revision label: `9ef110f928f40a88bf426432db08807b9edf89cb`

The running production container label may still show the previous Mini RC1
revision prefix `4262bb9a`. The running image label is the authoritative
hotfix revision evidence for this deployment.

## Preflight Validation

Preflight was deployed with `new-api:seedance-mini-billing-scheme-b-rc1`.

Validated:

- Mini no-video exact final billing delta: `0`.
- Mini video-input exact final billing delta: `0`.
- Mini 1080p and 4K requests rejected before task creation, billing, and
  upstream call.
- Fast 4K rejection regression rejected before task creation, billing, and
  upstream call.

## Production Deployment and Validation

Production deployment target:

- Container: `new-api-nightly`
- Image: `new-api:seedance-mini-billing-scheme-b-rc1`
- Image ID prefix: `c991ffe082dc`
- Image revision label: `9ef110f928f40a88bf426432db08807b9edf89cb`
- Restart count after deploy/smoke: `0`
- `/api/status`: HTTP `200`

Production smoke:

- Mini no-video task suffix `0Opc3U6B`: final actual quota `528773`,
  expected `528773`, delta `0`.
- Mini video-input task suffix `FEluwetP`: final actual quota `633649`,
  expected `633649`, delta `0`.
- Mini 1080p rejection: HTTP `400`, no task, no billing, no upstream.
- Mini 4K rejection: HTTP `400`, no task, no billing, no upstream.
- Fast 4K rejection: HTTP `400`, no task, no billing, no upstream.

No additional paid production smoke is authorized by this record.

## Blast Radius

- Audit window: Mini RC1 production deployment through post-deploy audit.
- Production Mini task rows in window: `6`.
- Original affected Mini SUCCESS tasks before the hotfix: `4`.
- Affected task suffixes: `4kamY0Li`, `7ydNwLxX`, `V33AhAoZ`, `YiBgqMZ0`.
- Post-deploy smoke tasks in window: `2`.
- Post-deploy Mini tasks with nonzero billing delta: none found.
- Original correction quota: `364016`.
- USD equivalent at `quota / 500000`: `$0.728032`.

## Missing Gates

- No exact final quota invariant.
- No `ModelRatio * OtherRatio` invariant.
- Production-only alias was not covered by smoke.
- Codex App smoke key handling was not pinned to a single Keychain procedure.

## Local Hotfix Scope

- Update the Seedance billing resolver only; no production config, DB, or
  customer configuration changes.
- Add resolver invariants for Mini no-video, Mini video, and Fast video.
- Add Mini precharge regression coverage using `ModelRatio = 1.75`.
- Add Mini-vs-Fast final quota ordering coverage for the same tokens and group
  ratio.

## Quota Credit

Henry explicitly approved production quota credit for the verified live token
targets only. The correction used token-level quota adjustment, not customer
configuration or Mini access changes.

- Total credited quota: `364016`
- Audit reason: `Seedance Mini Scheme B billing correction, incident 2026-06-28`
- Token rows updated: `2`
- Audit logs inserted: `2`
- Placeholder token hash `689b4389b0ad`: not compensated
- Smoke tasks: not compensated
- Original affected tasks: remain `SUCCESS`

Credit details:

| token_hash | before remain_quota | correction quota | after remain_quota | before used_quota | after used_quota |
|---|---:|---:|---:|---:|---:|
| `8920eb70817c` | `48743701` | `257457` | `49001158` | `16256299` | `15998842` |
| `b0b206db9656` | `10145330` | `106559` | `10251889` | `26949570` | `26843011` |

Post-credit read-only verification confirmed:

- exactly two audit logs for the approved reason;
- both token hashes have the expected final remain and used quota;
- no compensation log for placeholder hash `689b4389b0ad`;
- no compensation log for post-deploy smoke task suffixes `0Opc3U6B` or
  `FEluwetP`;
- affected task suffixes `4kamY0Li`, `7ydNwLxX`, `V33AhAoZ`, and `YiBgqMZ0`
  remain `SUCCESS`.

## Rollback Reference

- Rollback container:
  `new-api-nightly-before-seedance-mini-billing-scheme-b-20260628T030527Z`
- Rollback image/revision:
  `new-api:seedance-mini-rc1` /
  `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`

Keep the rollback artifact available through the observation window unless
Henry explicitly approves retiring it.

## Open Status

- Mini customer access has not been restored.
- Customer documentation was not changed.
- No additional credit, refund, deployment, restart, config change, token
  change, channel change, group change, alias change, or customer-access change
  is authorized by this record.

## Security and Verification Caveats

- Production and DB evidence is recorded only as sanitized runtime status,
  image revision, masked DB identity, approved task suffixes, and masked token
  hashes.
- One earlier parser bug printed generic invalid-resolution rejection JSON
  before rerun sanitization; it contained only safe rejection messages and no
  sensitive data.
- One combined post-credit read-only verification query failed due to the
  verification query itself. The same checks were rerun in smaller read-only
  queries and passed; no mutation occurred in the failed verification query.
- DB identity reports use masked DB hash prefixes. If different hash prefixes
  appear across notes, treat this as hash-method/reporting variance unless
  there is evidence of an actual DB target change. The controlling environment
  remains production MySQL with suffix `api_prod`.
