# Seedance Moderation Diagnose Production Candidate RC3

Date: 2026-06-20
Status: local production candidate; not deployed
Candidate image: `new-api:seedance-moderation-rc3`
Release branch: `release/seedance-moderation-v1`
Candidate runtime code commit: `d11c6f7f`

## Production Baseline

- Production image record: `new-api:seedance-p1-client-request-rc2`
- Runtime code commit: `d0bc8c18f1198437f91417de3293b5b03256118a`
- Current production remains unchanged.

The release comparison remains fixed to the production runtime commit above.
Feature branches, historical rc5 heads, RC1, and RC2 are not production
baselines.

## RC2 Blocker and RC3 Correction

RC2 correctly bounded the JSON lookup with the indexed `created_at` column,
but parsed the timestamp embedded in BP `cgt-...` identifiers as UTC.
Preflight task record `2658` showed that BP expresses that timestamp in fixed
UTC+8:

```text
cgt timestamp: 2026-06-17 13:32:28 UTC+8
created_at:     2026-06-17 05:32:28 UTC
Unix time:      1781674348
```

The RC2 range was therefore eight hours too late even though its execution
plan used the intended index. RC2 is blocked and must not be deployed.

RC3:

- keeps the strict `cgt-YYYYMMDDHHMMSS-xxxxx` format;
- parses the embedded timestamp in a fixed UTC+8 location;
- converts that instant to the Unix `created_at` range;
- rejects malformed, stale, or excessively future identifiers before querying
  `tasks`;
- keeps the 15-minute tolerance on either side of the parsed instant;
- combines the narrow `created_at` range with the database-specific exact
  `private_data.upstream_task_id` predicate and `LIMIT 2`;
- parses loaded `private_data` and compares the upstream task ID again;
- keeps numeric task-record, LSF `task_...`, and automatic manual task-ID
  resolution behavior unchanged.

For the preflight fixture, the query bounds are exactly:

```text
created_at >= 1781673448
created_at <= 1781675248
```

The fixture's `created_at=1781674348` is centered in that 30-minute range.
A similar identifier with a different suffix does not recover ownership.

## Index and Schema Status

RC3 introduces no schema change.

`model.Task.CreatedAt` already has the repository-standard `gorm:"index"` tag
in the production baseline. Existing migrations and AutoMigrate paths create
or preserve `idx_tasks_created_at`. RC3 does not add a field, migration,
redundant upstream-ID column, JSON functional index, or data backfill.

The SQLite lookup plan remains index-backed:

```text
SEARCH tasks USING INDEX idx_tasks_created_at (created_at>? AND created_at<?)
USE TEMP B-TREE FOR ORDER BY
```

It does not contain an unindexed `SCAN tasks`.

The RC2 MySQL preflight evidence showed:

```text
type=range
key=idx_tasks_created_at
rows=1
```

That evidence confirms the query shape can use the existing index, but RC3
still requires a fresh preflight `EXPLAIN` with the corrected UTC+8-derived
range before production approval.

## Required MySQL Preflight Verification

Before production approval, the isolated MySQL preflight must verify:

```sql
SHOW INDEX FROM tasks WHERE Key_name = 'idx_tasks_created_at';
```

It must run `EXPLAIN` for the parameterized RC3 lookup shape using safe
preflight placeholders:

```sql
SELECT *
FROM tasks
WHERE created_at >= ?
  AND created_at <= ?
  AND JSON_UNQUOTE(JSON_EXTRACT(private_data, '$.upstream_task_id')) = ?
ORDER BY id ASC
LIMIT 2;
```

The plan must select `idx_tasks_created_at` and use the corrected narrow range.
Do not record real task IDs, credentials, or `private_data`.

## Moderation-only Scope

Relative to RC2, RC3 changes only:

- BP cgt timestamp interpretation from UTC to fixed UTC+8;
- exact preflight-fixture and near-match regression tests;
- RC2/RC3 internal release documentation.

No customer-facing API, route, middleware chain, Asset Library behavior,
task creation, task polling, billing settlement, or customer documentation is
changed.

## Billing Exclusions and Blob Closure

The following remain byte-for-byte identical to the production baseline:

- `relay/channel/task/doubao/seedance_billing.go`;
- `relay/channel/task/doubao/seedance_billing_test.go`;
- billing session and task billing;
- task settlement, usage parsing, quota consumption, refund, and balance;
- `ModelRatio`, `GroupRatio`, and price/option data;
- normal/fast model recognition and 1080p behavior;
- `POST /v1/videos` and `GET /v1/videos/{task_id}` core paths.

RC3 does not contain:

- Seedance Mini model recognition or behavior;
- Mini no-video or with-video pricing;
- a Mini resolver or 1080p guard;
- Mini billing tests, configuration, or customer documentation;
- the values `0.0035` or `0.0021` outside this explicit exclusion record.

## Required Local Verification

Before RC3 handoff:

- run the complete Moderation, task model, middleware, and Seedance billing
  test suites;
- repeat the Seedance billing tests 20 times;
- verify the SQLite schema index and `EXPLAIN QUERY PLAN`;
- run the seven Moderation frontend tests and targeted Prettier check;
- complete a clean Dockerfile Bun/Vite production build;
- compare protected billing and customer-path blobs to the production baseline;
- scan the release diff for sensitive information;
- push a clean release HEAD;
- build the linux/amd64 RC3 image with OCI revision, version, and creation
  labels and verify the labels against the pushed release HEAD.

No server connection, preflight deployment, production deployment, or
production modification is authorized by this record.
