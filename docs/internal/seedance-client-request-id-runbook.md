# Seedance Client Request ID Idempotency Runbook

This note covers operational checks for `metadata.client_request_id` task reservations.

## Stale RESERVED Rows

`RESERVED` rows are created before billing and upstream submit for idempotent `POST /v1/videos` requests. A row can remain `RESERVED` if the upstream submit succeeds but local finalization fails before the success response is written.

Use a narrow query that avoids `data` and `private_data`:

```sql
SELECT id, task_id, token_id, client_request_id, user_id, channel_id, platform, action, submit_time, updated_at
FROM tasks
WHERE status = 'RESERVED'
AND submit_time < UNIX_TIMESTAMP() - 300
ORDER BY submit_time ASC
LIMIT 100;
```

Do not select or export `data`, `private_data`, request bodies, API keys, bearer tokens, AK/SK values, SQL_DSN values, environment files, customer keys, DB dumps, or production logs.

Manual inspection is required before any repair. P1 intentionally does not include automatic cleanup for stale `RESERVED` rows.
