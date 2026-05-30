# Seedance P1 Preflight and Rollout Runbook

Internal use only. Do not expose this document to customers.

This runbook applies to branch `feature/seedance-p1-billing-client-request-v1` and covers safe validation of:

- `GET /v1/billing/balance`
- `metadata.client_request_id` idempotency for `POST /v1/videos`

No production rollout may proceed without explicit approval.

## Preflight Environment

Verified baseline before P1 preflight deployment:

- Production container: `new-api-nightly`
- Production image currently: `new-api:seedance-usage-response-rc1`
- Preflight container: `new-api-preflight`
- Preflight image currently: `new-api:seedance-usage-response-rc1`
- Preflight port: `3002`
- Preflight data directory: `/etc/newapi-preflight`
- Production DB: MySQL 8.0.43-34, database `lsf_newapi_prod`
- Preflight DB: MySQL 8.0.43-34, database `lsf_newapi_preflight`
- Production and preflight DB targets are different
- P1 feature branch has not yet been deployed to preflight
- P1 schema columns/index are not present before deployment
- Production must not be touched during preflight

For this rollout, preflight must use MySQL and must not point to the live production DB. SQLite is legacy/cold-backup/rollback reference only; SQLite-only validation is not sufficient for the current production rollout.

Do not run production migrations during preflight. Do not use production credentials, customer bearer keys, AK/SK values, SQL_DSN values, environment files, DB dumps, or production logs in this runbook.

## Preflight Backup

Back up the MySQL preflight DB before starting validation. Use the approved internal backup procedure for `lsf_newapi_preflight`; do not paste backup contents, credentials, connection strings, or command output into this document.

If testing against copied production data, confirm it is a copy and not the live production DB.

Never overwrite `/etc/newapi/one-api.db`. That path is legacy SQLite/cold-backup/rollback reference only and is not the active production DB for this rollout.

## Schema Verification

Schema verification is MySQL-first and must be performed against the preflight MySQL DB.

Verify the `tasks` table contains:

- `token_id`
- `client_request_id`
- `client_request_hash`
- a unique index equivalent on `token_id` + `client_request_id`

Before P1 preflight deployment, these columns/index may be absent. After starting the P1 preflight image, they must be present.

MySQL checks:

```sql
SHOW COLUMNS FROM tasks LIKE 'token_id';
SHOW COLUMNS FROM tasks LIKE 'client_request_id';
SHOW COLUMNS FROM tasks LIKE 'client_request_hash';
SHOW INDEX FROM tasks WHERE Key_name = 'idx_tasks_token_client_request';
```

Confirm the index is unique and includes both `token_id` and `client_request_id`.

Legacy SQLite reference only:

```sql
PRAGMA table_info(tasks);
PRAGMA index_list(tasks);
PRAGMA index_info('idx_tasks_token_client_request');
```

## API Smoke Tests

Use placeholders only:

- `<BASE_URL>`: preflight URL, for example `http://127.0.0.1:3002`
- `<LSF_API_KEY>`: preflight API key
- `<TENANT_MODEL_ALIAS>`: tenant video model alias for preflight
- `<TASK_ID>`: task id returned by `POST /v1/videos`
- `<CLIENT_REQUEST_ID>`: stable test id for the preflight run

### Billing Balance

```bash
curl -sS <BASE_URL>/v1/billing/balance \
  -H "Authorization: Bearer <LSF_API_KEY>"
```

Expected:

- HTTP 200 for a valid API key
- response object is `billing.balance`
- currency is `USD`
- no internal fields are present

### Video Task Without client_request_id

```bash
curl -sS <BASE_URL>/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "Preflight video task without client_request_id"
  }'
```

Expected:

- request succeeds using the existing behavior
- response includes `id` and `task_id`
- no idempotency metadata is required

### Poll Video Task

```bash
curl -sS <BASE_URL>/v1/videos/<TASK_ID> \
  -H "Authorization: Bearer <LSF_API_KEY>"
```

Expected:

- response returns the requested task
- polling remains the recommended workflow
- task eventually reaches `completed` or `failed`

### Video Task With Valid client_request_id

```bash
curl -sS <BASE_URL>/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "Preflight video task with client_request_id",
    "metadata": {
      "client_request_id": "<CLIENT_REQUEST_ID>"
    }
  }'
```

Expected:

- request succeeds
- response includes `id` and `task_id`
- response metadata includes `client_request_id`

### Duplicate client_request_id Replay

Repeat the same request with the same `<LSF_API_KEY>` and `<CLIENT_REQUEST_ID>`:

```bash
curl -sS <BASE_URL>/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "Preflight video task with client_request_id",
    "metadata": {
      "client_request_id": "<CLIENT_REQUEST_ID>"
    }
  }'
```

Expected:

- response returns the original task
- returned `id` and `task_id` match the first request
- no second video task is created
- duplicate replay does not call upstream again
- duplicate replay does not deduct again

### Invalid client_request_id

```bash
curl -sS -i <BASE_URL>/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "Preflight invalid client_request_id",
    "metadata": {
      "client_request_id": "invalid/value"
    }
  }'
```

Expected:

- HTTP 400
- OpenAI-style error
- error code is `invalid_client_request_id`
- no upstream task is created

## Expected Behavior Summary

- Requests without `metadata.client_request_id` work as before.
- First request with a valid `metadata.client_request_id` creates one task.
- Duplicate same API key + same `client_request_id` returns the original task.
- Duplicate replay does not create a second task.
- Duplicate replay does not call upstream again.
- Duplicate replay does not deduct again.
- Different API keys may use the same `client_request_id` independently.
- The current version does not compare request body consistency.

## DB Verification Queries

Use read-only MySQL queries against the preflight DB. Do not select `data` or `private_data`.

Count tasks by `client_request_id`:

```sql
SELECT COUNT(*) AS task_count
FROM tasks
WHERE client_request_id = '<CLIENT_REQUEST_ID>';
```

Inspect non-sensitive task columns:

```sql
SELECT id, task_id, token_id, client_request_id, user_id, channel_id, platform, action, status, progress, quota, submit_time, updated_at
FROM tasks
WHERE client_request_id = '<CLIENT_REQUEST_ID>'
ORDER BY id ASC;
```

Check for stale `RESERVED` rows:

```sql
SELECT id, task_id, token_id, client_request_id, user_id, channel_id, platform, action, submit_time, updated_at
FROM tasks
WHERE status = 'RESERVED'
AND submit_time < UNIX_TIMESTAMP() - 300
ORDER BY submit_time ASC
LIMIT 100;
```

Do not include customer request bodies, bearer tokens, AK/SK values, SQL_DSN values, environment files, DB dumps, production logs, or real customer data in notes or screenshots.

## Stale RESERVED Handling

P1 has no automatic cleanup for stale `RESERVED` rows.

Manual inspection is required before repair:

- Do not blindly mark rows failed.
- Check whether an upstream task may have been created before any repair.
- Escalate before repair.

## Rollback Notes

- Keep the previous production image/tag available.
- Do not rollback DB blindly.
- Schema additions are forward-compatible, but must still be treated carefully.
- Rollback requires explicit approval.

## Production Rollout Gate

Production rollout requires all of the following:

- local tests pass
- P1 image built
- Docker build succeeds
- `new-api-preflight` updated to the P1 image only
- preflight starts cleanly on port `3002`
- MySQL preflight schema verified after startup
- MySQL duplicate key behavior verified
- SQLite-only validation is not sufficient
- all smoke tests pass
- billing balance verified
- no-client_request_id regression passes
- idempotency duplicate replay verified
- stale `RESERVED` query checked
- explicit user approval received

## Customer Communication Notes

Customer docs should only mention:

- Base URL
- Bearer API key
- tenant model aliases
- supported endpoints
- billing balance
- optional `metadata.client_request_id`

Do not mention reservation, `token_id`, DB schema, ProjectName, channels, groups, upstream model names, billing internals, or production paths.
