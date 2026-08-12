# Environments

Current verified release facts as of the 2026-08-12 MediaKit Video Enhancement
P0 production closeout:

- Production container: `new-api-nightly`
- Production current image:
  `new-api:mediakit-parser-status-fix-968670a1`
- Production current image ID prefix: `c7521715e0ef`
- Production source revision:
  `968670a13306d9ea57497aa48ff022a732c2112e`
- Previous production image:
  `new-api:seedance-2.5-p1-candidate-51bdea54`
- Previous production revision:
  `51bdea5468ca7be213c02bbc540e6cde1ce993b4`
- Retained rollback container:
  `new-api-nightly-before-mediakit-p0-20260812T115829Z`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- Production rollout health checks passed through `/api/status` after the
  MediaKit P0 deployment.
- Production container restart count was `0` at closeout.
- MediaKit production Standard 1080p smoke passed with one POST, actual profile
  `10s / 30fps / 1080p / standard`, exact billing reconciliation, and two
  repeated GETs with unchanged balance and billing records.
- Production/preflight required MediaKit configuration shapes matched without a
  ProjectName requirement. No Channel, token, group, mapping, environment, or
  database configuration was changed during validation.
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
