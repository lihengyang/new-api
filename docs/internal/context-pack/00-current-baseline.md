# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Production domain:

```text
https://ai-api.lightspeedfuture.com
```

Supported customer video endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`

Supported P1 customer endpoint:

- `GET /v1/billing/balance`

`metadata.client_request_id` is optional and supported for `POST /v1/videos`.

Asset Library paths are supported according to the tenant package.

Cancel, delete, and list video tasks are not part of the current default scope unless separately enabled in writing.

Customer docs must not expose internal routing, ProjectName, AK/SK, channel names, group names, upstream model names, `token_id`, DB schema, or billing internals.

## Current Database Baseline — Confirmed 2026-06-18

- LSF production DB baseline: MySQL.
- LSF preflight DB baseline: MySQL.
- Production and preflight use separate MySQL databases.
- Generic new-api code must remain compatible with MySQL, PostgreSQL, and SQLite.
- SQLite references in LSF documents are legacy, historical release, rollback, or cold-backup context only unless Henry explicitly confirms a new baseline.
- `/etc/newapi/one-api.db` and `/data/one-api.db` are not the current live LSF production DB.

## Current Production Release Baseline — 2026-05-31

Production rollout succeeded for:

- Release: Seedance P1 Client Request ID / Billing Balance
- Production image: `new-api:seedance-p1-client-request-rc2`
- Runtime code commit: `d0bc8c18`
- Latest docs/context commit: `927d1fba`
- Production container: `new-api-nightly`
- Production DB: MySQL `lsf_newapi_prod`
- Preflight image: `new-api:seedance-p1-client-request-rc2`
- Customer guide: `docs/customer/Light_Speed_Future_API_Integration_Guide_v2.1.2.docx`

Verified production behavior:

- `GET /v1/billing/balance`
- `POST /v1/videos` with optional `metadata.client_request_id`
- duplicate replay returns the original task for the same API key + same `client_request_id`
- completed `GET /v1/videos/{task_id}` includes `metadata.client_request_id`, `metadata.url`, and `usage`
- no stuck `RESERVED` rows observed during rollout verification
- real production paid test task completed successfully

Rollback assets retained after rollout:

- old container: `new-api-nightly-before-p1-20260531153622`
- rollback image: `new-api:seedance-usage-response-rc1`
- DB backup: `lsf_newapi_prod.before-p1-production.20260531153622.sql.gz`
- env snapshot: `new-api-nightly.env.before-p1.20260531153622`
- inspect snapshot: `new-api-nightly.inspect.before-p1.20260531153622.json`
