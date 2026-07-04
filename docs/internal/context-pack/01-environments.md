# Environments

Current verified release facts as of the 2026-07-04 Seed Audio P01
reliability/admin UI production closeout:

- Production container: `new-api-nightly`
- Production current image:
  `new-api:seed-audio-p01-reliability-admin-ui-rc1-fa002bfc`
- Production current image ID prefix: `2c55d6f3eff3`
- Production source revision:
  `fa002bfc3e3a99e469f9332e2e1efce48757ba44`
- Previous production image: `new-api:seed-audio-p0-prod-20260702-303d4ea`
- Previous production revision:
  `303d4ea3efa0a3317a22d4232348bf623a9228f8`
- Retained rollback container:
  `new-api-nightly-before-seed-audio-p01-reliability-admin-ui-rc1-20260704T031037Z`
- Retained rollback image tag:
  `new-api:rollback-before-seed-audio-p01-reliability-admin-ui-rc1-20260704T031037Z`
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
  after Seed Audio P01 reliability/admin UI deployment.
- Production container restart count was `0` at closeout.
- Production `seed_audio_idempotencies.error_diagnostics` exists as nullable
  `TEXT`.
- Seed Audio P01 production smoke passed: `3001` local rejection with no
  charge/no upstream dispatch, `3000` not rejected as too long, minimal paid
  success, replay without duplicate charge, and sanitized sensitive/log scan.
- Production `/console/token` manual check passed for page-size `10 -> 20`,
  `20 -> 10`, search-mode page-size changes, no flicker/request storm, and no
  HTTP `429`.
- Seedance Mini Scheme B production gates passed for billing balance, Mini
  high-resolution rejection before task/billing/upstream, Mini no-video exact
  final billing, Mini video-input exact final billing, and Fast 4K rejection
  regression.
- Original affected Mini SUCCESS tasks in the audited window: `4`.
- Total token-level quota credit completed: `364016`.
- Smoke tasks were not compensated.
- Customer Mini usage was manually paused outside system configuration; no
  system Mini access toggle was changed or restored.
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
