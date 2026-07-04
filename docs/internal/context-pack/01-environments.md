# Environments

Current verified release facts as of the 2026-07-04 Seed Audio parser hotfix
production closeout:

- Production container: `new-api-nightly`
- Production current image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Production current image ID prefix: `dda5be8c7020`
- Production source revision:
  `dcda4e551e50051ab536173f342eb21f0ca60e8d`
- Previous production image:
  `new-api:seed-audio-p01-reliability-admin-ui-rc1-fa002bfc`
- Previous production revision:
  `fa002bfc3e3a99e469f9332e2e1efce48757ba44`
- Retained rollback container:
  `new-api-nightly-before-seed-audio-parser-hotfix-rc1-20260704T173731Z`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- Production rollout health checks passed through `/api/status` after the Seed
  Audio parser hotfix deployment.
- Production container restart count was `0` at closeout.
- Production `seed_audio_idempotencies.error_diagnostics` exists as nullable
  `TEXT`.
- Production `seed_audio_idempotencies.x_tt_logid` exists and was populated for
  the hotfix Smoke A/B rows.
- Seed Audio P01 production smoke passed: `3001` local rejection with no
  charge/no upstream dispatch, `3000` not rejected as too long, minimal paid
  success, replay without duplicate charge, and sanitized sensitive/log scan.
- Seed Audio parser hotfix production Smoke A/B/C passed with internal alias
  `lsf-seed-audio-1.0-henrytest`: short text passed, complex SFX generation
  passed without `invalid_json` or `seed_audio_upstream_error`, replay returned
  the cached response without duplicate billing, and sanitized diagnostics/log
  scans did not show raw base64, raw prompt, raw upstream body, or temporary URL
  value leakage.
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
