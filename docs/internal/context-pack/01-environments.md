# Environments

Current verified release facts as of 2026-06-20:

- Production container: `new-api-nightly`
- Production current image: `new-api:seedance-moderation-rc3`
- Production current image ID: `sha256:429fbd05318ffafd4a44c5262e8ad7ab29320356d24ea33e9345d40421f29a59`
- Production source revision: `24c844439fdfd7b7975ed8120a43907e2895cdac`
- Previous production image: `new-api:seedance-p1-client-request-rc2`
- Previous production image ID: `sha256:a90396cab71ce41f62e729868edfb64d8f010cb3ced27d696778f271563ba680`
- Retained rollback container: `new-api-nightly-before-seedance-moderation-rc3-20260620115426`
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
- MySQL is the current production/preflight database baseline.
- SQLite is legacy/cold-backup/rollback reference only unless explicitly re-verified.

Historical P1 pre-deployment snapshot:

- production image: `new-api:seedance-usage-response-rc1`;
- preflight image: `new-api:seedance-usage-response-rc1`;
- P1 schema was not present before P1 image deployment.

Do not rely on chat memory for production, preflight, image, or DB facts. Re-verify current state with read-only commands before release decisions.
