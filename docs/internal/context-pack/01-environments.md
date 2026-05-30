# Environments

Verified environment facts:

- Production container: `new-api-nightly`
- Production current image before P1 deployment: `new-api:seedance-usage-response-rc1`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight current image before P1 deployment: `new-api:seedance-usage-response-rc1`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- P1 schema was not present before P1 image deployment.
- MySQL is the current P1 production/preflight reality.
- SQLite is legacy/cold-backup/rollback reference only unless explicitly re-verified.

Do not rely on chat memory for production, preflight, image, or DB facts. Re-verify current state with read-only commands before release decisions.
