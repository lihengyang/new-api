# Seedance Moderation Diagnose Production Candidate RC2

Date: 2026-06-20
Status: blocked in preflight; superseded by RC3; not deployed
Candidate image: `new-api:seedance-moderation-rc2`
Release branch: `release/seedance-moderation-v1`
Candidate runtime code commit: `54fa1249`

## RC2 Preflight Blocker

RC2 is not eligible for production.

Preflight task record `2658` established that the timestamp embedded in BP
`cgt-...` identifiers is expressed in fixed UTC+8, while RC2 parsed it as UTC.
The indexed 30-minute `created_at` range was therefore shifted eight hours
forward and could fail to recover an existing task's ownership.

RC3 supersedes RC2 by parsing the embedded timestamp in fixed UTC+8 before
converting it to the Unix `created_at` range. RC2 must remain recorded as a
failed candidate and must not be relabeled or deployed.

## Production Baseline

- Production image record: `new-api:seedance-p1-client-request-rc2`
- Runtime code commit: `d0bc8c18f1198437f91417de3293b5b03256118a`
- Current production remains unchanged.

The release comparison remains fixed to the production runtime commit above.
Feature-branch or historical rc5 heads are not release baselines.

## RC1 Blocker and RC2 Correction

RC1 was blocked because persisted BP `cgt-...` lookup used an exact JSON
predicate with `LIMIT 2`, but no indexed relational predicate. A missing or
rare ID could therefore scan the full `tasks` table.

RC2:

- strictly accepts `cgt-YYYYMMDDHHMMSS-xxxxx`, with a five-character
  alphanumeric suffix;
- parses the embedded timestamp as UTC before any task query;
- rejects malformed IDs, timestamps more than two hours in the future, and
  timestamps outside the BP 14-day lookup window before database access;
- queries only `created_at` within 15 minutes before or after the embedded
  timestamp;
- combines that range with the database-specific exact
  `private_data.upstream_task_id` JSON predicate and `LIMIT 2`;
- parses loaded `private_data` and compares the upstream task ID again before
  accepting a result;
- keeps numeric task-record and exact LSF `task_...` lookup unchanged.

## Index and Schema Status

No new schema change is introduced by RC2.

`model.Task.CreatedAt` already has the repository-standard `gorm:"index"` tag
in the production baseline. Both normal and fast migration paths already run
`AutoMigrate(&Task{})`, which creates or preserves `idx_tasks_created_at`.
RC2 does not add a field, migration, redundant upstream-ID column, JSON
functional index, or data backfill.

The SQLite test schema confirms the existing `created_at` index. The generated
RC2 lookup plan is:

```text
SEARCH tasks USING INDEX idx_tasks_created_at (created_at>? AND created_at<?)
USE TEMP B-TREE FOR ORDER BY
```

The plan does not contain `SCAN tasks`.

## Required MySQL Preflight Verification

Before production approval, the isolated MySQL preflight must verify:

```sql
SHOW INDEX FROM tasks WHERE Key_name = 'idx_tasks_created_at';
```

It must also run `EXPLAIN` for the parameterized RC2 lookup shape and confirm
that the `created_at` index is selected:

```sql
SELECT *
FROM tasks
WHERE created_at >= ?
  AND created_at <= ?
  AND JSON_UNQUOTE(JSON_EXTRACT(private_data, '$.upstream_task_id')) = ?
ORDER BY id ASC
LIMIT 2;
```

Use safe preflight placeholders only. Do not record real task IDs or
`private_data`.

## Moderation-only Scope

RC2 contains the RC1 Moderation Diagnose surface plus only:

- bounded persisted BP task lookup;
- strict pre-query cgt validation;
- cross-database parameterized query tests;
- SQLite index and execution-plan tests;
- this internal release documentation.

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

RC2 does not contain:

- Seedance Mini model recognition or behavior;
- Mini no-video or with-video pricing;
- a Mini resolver or 1080p guard;
- Mini billing tests, configuration, or documentation;
- the values `0.0035` or `0.0021`.

## Local Verification

Passed before this record:

- `go test ./controller ./model ./middleware ./relay/channel/task/doubao`;
- `go test ./controller -run 'Moderation|Diagnose'`;
- `go test ./model -run 'Task|Upstream|Moderation|CGT'`;
- `go test ./middleware -run 'Moderation|Admin|Rate'`;
- `go test ./relay/channel/task/doubao -run 'Seedance|Billing' -count=20`;
- SQLite schema-index and `EXPLAIN QUERY PLAN` assertions;
- `git diff --check`.

Final frontend, clean Dockerfile build, image provenance inspection, and
sensitive-information scan remain required before RC2 handoff.

No server connection, preflight deployment, production deployment, or
production modification is authorized by this record.
