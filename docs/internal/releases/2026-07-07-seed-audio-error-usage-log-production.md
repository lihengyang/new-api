# Seed Audio Error Usage Log Production Release Record

Date: 2026-07-07
Status: `PRODUCTION_DEPLOY_PASSED / POST_DEPLOY_SMOKE_PARTIAL_API_PASS / ADMIN_UI_MANUAL_PASS`
Production deployed: yes
Customer documentation changed: no
Production smoke account/model entry: Henry/LSF internal `henrytest`

This is the internal production release record for Seed Audio usage error logs
on the OpenAI-compatible `POST /v1/audio/speech` path. It records the completed
preflight validation, production deployment, schema verification, query-plan
verification, and post-deploy smoke. It does not authorize customer
documentation changes, production config changes, production DB manual DDL, or
additional paid smoke.

## Artifact Identity

- Branch: `codex/seed-audio-parser-hotfix-production-20260704`
- Source commit / runtime revision:
  `6107eb4db3b4cce2291c78c69db4aa12b8422163`
- Production container: `new-api-nightly`
- Production image:
  `new-api:seed-audio-error-logs-prod-candidate-6107eb4d`
- Production image ID:
  `sha256:65718430ecddd230d5905ad803cc8e023a9335128eb5ac58c6d560c186cffe98`
- Platform: `linux/amd64`
- Previous production image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Preserved rollback container:
  `new-api-nightly-before-seed-audio-error-logs-6107eb4d-20260707T164119Z`
- Rollback image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Rollback happened: no

## Feature Summary

- Seed Audio upstream-dispatched failures now write usage error logs.
- Error logs are zero-cost operational diagnostics:
  - `quota = 0`
  - cost/amount remain zero or absent in API responses
  - balance is unchanged
  - usage statistics continue to count consume logs only
- Admin usage logs can show redacted diagnostics for troubleshooting:
  - gateway request ID
  - `client_request_id`
  - upstream request ID when available
  - HTTP status
  - error code
  - retryable flag
  - unpaid marker
- User-facing and token-facing usage logs are sanitized by the backend and do
  not expose Seed Audio error diagnostics.
- Request ID search supports exact matching across gateway request ID,
  `client_request_id`, and upstream request ID.
- `client_request_id` is now a structured logs field and has a leading-column
  index for exact lookup and newest-first scans.

## Schema

Production and preflight use MySQL as the current runtime database baseline.
The release relies on GORM AutoMigrate during application startup for the logs
table change.

Added nullable `logs` columns:

```text
client_request_id varchar(191)
upstream_request_id varchar(191)
error_code varchar(128)
http_status bigint
retryable tinyint(1)
```

Added indexes:

```text
idx_logs_client_request_id_created_at(client_request_id, created_at)
idx_logs_upstream_request_id(upstream_request_id)
```

Intentionally not added:

```text
idx_logs_error_code
idx_logs_http_status
```

The `error_code` and `http_status` fields are retained for admin diagnostics,
but they do not have standalone indexes because there is no active query entry
that requires them.

## Query-Plan Verification

Preflight and production MySQL verification passed:

- Exact `client_request_id` query used
  `idx_logs_client_request_id_created_at`.
- Admin request ID search using the real
  `request_id OR client_request_id OR upstream_request_id` condition used
  MySQL `index_merge` with no obvious full-table scan.
- The query path preserved the existing `idx_logs_request_id` and added
  `idx_logs_upstream_request_id` because the Request ID search box can match
  upstream request IDs.

## Production Runtime Verification

- `/api/status`: HTTP `200`
- Runtime revision:
  `6107eb4db3b4cce2291c78c69db4aa12b8422163`
- Production schema/index verification: passed.
- Production lock/process checks during deploy validation did not show a DDL
  lock incident.

No SQL_DSN, DB credentials, bearer token, AK/SK, ProjectName value, channel
settings, raw upstream response, raw customer input, or customer data was
recorded in this release note.

## Production Smoke

Post-deploy smoke used a synthetic client request ID:

```text
codex-prod-seed-audio-error-smoke-20260707T165446Z-ee22f6
```

API-key smoke results:

- Public `/api/status`: HTTP `200`, success response.
- Seed Audio synthetic failure: HTTP `400`, `invalid_reference_url`.
- Balance before/after: unchanged.
- Token-facing logs found a recent Seed Audio `LogTypeError` row.
- Token-facing log quota: `0`.
- Token-facing log did not expose:
  `client_request_id`, `upstream_request_id`, `error_code`, `http_status`,
  `retryable`, or `other.seed_audio_error`.
- Recent Seed Audio success consume logs remained unaffected by error-log
  sanitization and still showed normal consume-side fields.

Admin UI smoke was manually verified by Henry:

- Admin usage log showed a Seed Audio request failure.
- HTTP status: `400`.
- Error code: `invalid_reference_url`.
- `retryable`: `false`.
- Admin view showed `client_request_id`, upstream request ID, and unpaid
  marker.

The post-deploy API-key smoke could not use `/api/log/` or `/api/log/self`
without a web session. Codex did not recover or guess production web access
tokens from the database. The backend sanitization path was still verified
through the token-facing log API, and Henry manually verified the admin UI.

## Boundaries

- No customer documentation was changed.
- No customer token, customer alias, customer group, customer input, or customer
  balance was used for smoke.
- No production DB manual schema changes were made.
- No production DB DROP/ALTER/CREATE was run manually.
- No raw upstream response, raw customer input, ProjectName value, bearer token,
  AK/SK, SQL_DSN, DB password, channel setting, or customer data was printed or
  stored in this release record.

## Rollback Reference

- Rollback container:
  `new-api-nightly-before-seed-audio-error-logs-6107eb4d-20260707T164119Z`
- Rollback image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`

Code rollback will not automatically remove the new nullable columns or indexes
created by AutoMigrate. Do not drop logs fields or indexes during rollback
unless Henry explicitly approves a separate DB-change plan.

## Follow-up Rules

- Keep Seed Audio error diagnostics admin-only.
- Do not expose Seed Audio error diagnostics through `/api/log/self`,
  `/api/log/token`, or other user-facing log paths.
- Do not count `LogTypeError` rows in consume quota/stat totals.
- Keep `idx_logs_client_request_id_created_at` because exact
  `client_request_id` lookup is a core operational path.
- Add standalone diagnostics indexes only when there is a real query entry and
  measured need.
