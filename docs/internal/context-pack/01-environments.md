# Environments

Verified environment facts:

- Production container: `new-api-nightly`
- Production current verified P1 image: `new-api:seedance-p1-client-request-rc2`
- Historical pre-P1 production image: `new-api:seedance-usage-response-rc1`
- Production server hosting: BytePlus Hong Kong cloud
- Verified production server architecture: `x86_64` / `linux/amd64`
- Production/preflight deployment images should be built as `linux/amd64`
- When building from Apple Silicon local machines, use `docker buildx build --platform linux/amd64`
- Production DB: MySQL 8.0.43-34
- Production DB name: `lsf_newapi_prod`
- Production DB fingerprint: `1e7c66a33168567c`
- Preflight container: `new-api-preflight`
- Preflight current verified P1 image: `new-api:seedance-p1-client-request-rc2`
- Historical pre-P1 preflight image: `new-api:seedance-usage-response-rc1`
- Preflight port: `3002`
- Preflight DB: MySQL 8.0.43-34
- Preflight DB name: `lsf_newapi_preflight`
- Preflight DB fingerprint: `e2546ab4e0b7a4a5`
- Production and preflight DB targets are different.
- P1 schema was not present before P1 image deployment.
- MySQL is the current P1 production/preflight reality.
- SQLite is legacy/cold-backup/rollback reference only unless explicitly re-verified.
- `/etc/newapi/one-api.db` and `/data/one-api.db` are not current live production database paths.

Do not rely on chat memory for production, preflight, image, or DB facts. Re-verify current state with read-only commands before release decisions.
