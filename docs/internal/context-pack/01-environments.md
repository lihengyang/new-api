# Environments

Current verified release facts as of the 2026-06-28 Seedance Mini Scheme B
billing hotfix production deployment, smoke pass, and quota correction:

- Production container: `new-api-nightly`
- Production current image: `new-api:seedance-mini-billing-scheme-b-rc1`
- Production current image ID prefix: `c991ffe082dc`
- Production source revision: `9ef110f928f40a88bf426432db08807b9edf89cb`
- Production container label revision may still show previous Mini RC1 prefix
  `4262bb9a`; running image label is the authoritative revision evidence.
- Previous production image: `new-api:seedance-mini-rc1`
- Previous production revision: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Retained rollback container: `new-api-nightly-before-seedance-mini-billing-scheme-b-20260628T030527Z`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- Production rollout health checks passed locally and through `/api/status`
  after Scheme B deployment and compensation.
- Seedance Mini Scheme B production gates passed for billing balance, Mini
  high-resolution rejection before task/billing/upstream, Mini no-video exact
  final billing, Mini video-input exact final billing, and Fast 4K rejection
  regression.
- Original affected Mini SUCCESS tasks in the audited window: `4`.
- Total token-level quota credit completed: `364016`.
- Smoke tasks were not compensated.
- Mini customer access remains unrestored until Henry separately approves.
- The rollback artifact must be preserved through the observation window unless
  Henry explicitly approves retiring it.
- Customer Mini documentation remains unpublished until separately approved.
- MySQL is the current production/preflight database baseline.
- SQLite is legacy/cold-backup/rollback reference only unless explicitly re-verified.

Historical P1 pre-deployment snapshot:

- production image: `new-api:seedance-usage-response-rc1`;
- preflight image: `new-api:seedance-usage-response-rc1`;
- P1 schema was not present before P1 image deployment.

Do not rely on chat memory for production, preflight, image, or DB facts. Re-verify current state with read-only commands before release decisions.
