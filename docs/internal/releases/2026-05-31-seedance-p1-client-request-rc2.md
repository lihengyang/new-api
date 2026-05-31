# Seedance P1 Client Request ID Production Release

Date: 2026-05-31
Production image: `new-api:seedance-p1-client-request-rc2`
Runtime code commit: `d0bc8c18`
Latest docs/context commit: `927d1fba`
Production container: `new-api-nightly`
Preflight container: `new-api-preflight`

## Scope

This release adds P1 support for:

- `GET /v1/billing/balance`
- optional `metadata.client_request_id` for `POST /v1/videos`
- same API key + same `client_request_id` duplicate replay behavior
- customer-facing video responses carrying `metadata.client_request_id`
- task reservation/finalization logic for idempotent video task creation

## Production Result

Production rollout completed successfully.

Verified in production:

- `/api/status` returned success
- `GET /v1/billing/balance` returned normal balance response
- invalid `metadata.client_request_id` returned HTTP 400 OpenAI-style error
- production MySQL schema contains:
  - `tasks.token_id`
  - `tasks.client_request_id`
  - `tasks.client_request_hash`
  - `idx_tasks_token_client_request(token_id, client_request_id)`
- real production paid video task completed successfully
- duplicate replay returned the same `task_id`
- DB showed `task_count = 1`
- DB showed `reserved_tasks = 0`
- completed `GET /v1/videos/{task_id}` response included:
  - `metadata.client_request_id`
  - `metadata.url`
  - `usage`

## Rollback Assets

Keep for at least 48 hours after rollout:

- old container: `new-api-nightly-before-p1-20260531153622`
- rollback image: `new-api:seedance-usage-response-rc1`
- DB backup: `lsf_newapi_prod.before-p1-production.20260531153622.sql.gz`
- env snapshot: `new-api-nightly.env.before-p1.20260531153622`
- inspect snapshot: `new-api-nightly.inspect.before-p1.20260531153622.json`

## Customer Document

Final customer-facing document:

- `docs/customer/Light_Speed_Future_API_Integration_Guide_v2.1.2.docx`

This is the customer delivery guide for the P1 production rollout unless superseded by a later version.
