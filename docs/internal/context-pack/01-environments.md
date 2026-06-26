# Environments

Current verified release facts as of the 2026-06-26 Seedance 2.0 Mini RC1
production deployment and smoke pass:

- Production container: `new-api-nightly`
- Production current image: `new-api:seedance-mini-rc1`
- Production current image ID: `sha256:420a29a4dac01fae8b13ca06c54dcc793e1422c90ee45279c68c24fcd6c6d50a`
- Production source revision: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Previous production image: `new-api:seedance-4k-rc2`
- Previous production image ID: `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Retained rollback container: `new-api-nightly-before-seedance-mini-rc1-20260626T162156Z`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- Production rollout health checks passed locally and through the public
  `/api/status` route after Mini RC1 production smoke.
- Seedance 2.0 Mini RC1 production gates passed for billing balance, Mini
  high-resolution rejection before task/billing/upstream, Mini 480p no-video
  success, retrieve, and final settlement evidence.
- Mini `reference_video` production smoke was not run because it requires
  separate approval for an extra paid production task.
- Live Standard 4K was not rerun during Mini RC1. Henry accepted this as a
  Mini RC1 risk boundary.
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
