# Environments

Current verified release facts as of the 2026-06-23 Seedance 2.0 4K RC2
production deployment:

- Production container: `new-api-nightly`
- Production current image: `new-api:seedance-4k-rc2`
- Production current image ID: `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Production source revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`
- Previous production image: `new-api:seedance-moderation-rc3`
- Previous production image ID: `sha256:429fbd05318ffafd4a44c5262e8ad7ab29320356d24ea33e9345d40421f29a59`
- Retained rollback container: `new-api-nightly-before-seedance-4k-rc2-redeploy-20260623T110257Z`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- Production rollout health checks passed locally, inside the container, and
  through the public `/api/status` route.
- Seedance 2.0 4K RC2 production gates passed for health, restart count,
  sanitized error pattern count, MySQL, schema, alias-to-endpoint routing,
  token ability, and Fast 4K reject.
- MySQL is the current production/preflight database baseline.
- SQLite is legacy/cold-backup/rollback reference only unless explicitly re-verified.

Historical P1 pre-deployment snapshot:

- production image: `new-api:seedance-usage-response-rc1`;
- preflight image: `new-api:seedance-usage-response-rc1`;
- P1 schema was not present before P1 image deployment.

Do not rely on chat memory for production, preflight, image, or DB facts. Re-verify current state with read-only commands before release decisions.
