# MediaKit Video Enhancement P0 Release Record

Date: 2026-08-12
Status: `PRODUCTION_VALIDATED / P0_COMPLETE`
Customer documentation changed: no
Production touched: yes

This combined internal release, runbook, and finding record closes the
BytePlus AI MediaKit Video Enhancement P0 preflight and production validation.
It does not authorize an additional paid task, a configuration change, a
customer-guide update, or any P1 implementation.

## Artifact Identity

- Branch: `codex/mediakit-video-enhancement-p0-20260811`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Database: isolated preflight MySQL
- Source / OCI revision:
  `968670a13306d9ea57497aa48ff022a732c2112e`
- Production image: `new-api:mediakit-parser-status-fix-968670a1`
- Production image ID/digest:
  `sha256:c7521715e0ef7aaf882e128d98080c9ac3511dc830b432cb53c6005093e254ce`
- Platform: `linux/amd64`
- Migrations for this parser fix: `0`
- Previous production image: `new-api:seedance-2.5-p1-candidate-51bdea54`
- Previous production revision:
  `51bdea5468ca7be213c02bbc540e6cde1ce993b4`
- Preserved rollback container:
  `new-api-nightly-before-mediakit-p0-20260812T115829Z`

The preflight runtime was healthy with restart count `0`. Existing environment,
network, port, restart policy, and MySQL data were preserved. Production was
not modified.

## Fixes Included

### Client request ID forwarding

Revision:
`9622dde5e985a7864a17509c71130416865aa17c`

- Extract `metadata.client_request_id` from the LSF request.
- Forward it as the BytePlus create-request top-level `client_token`.
- Keep `duration`, `client_request_id`, and the original `metadata` object out
  of the upstream request.
- Continue rejecting customer-supplied `client_token` and invalid client token
  values before precharge or upstream dispatch.

### Response-envelope status parsing

Revision:
`968670a13306d9ea57497aa48ff022a732c2112e`

- Parse task status from the BytePlus response top level.
- Continue reading the output URL and actual media fields from `result`.
- Do not restore an old LSF status merely because `result.status` is absent.
- Retain the existing safe fallback only when the entire upstream response has
  no status.

Root cause: `PARSER_STATUS_SCOPE_MISMATCH`. The upstream response placed
`status` on the response envelope, but the adapter entered `result` and looked
for `result.status`, then retained the stored `IN_PROGRESS/30%` state.

## Preflight Evidence

The successful Standard task used the approved Henry/LSF internal `henrytest`
token and matching internal alias. `henrytest` is a token name, not a username.

- Tool version: `standard`
- Actual duration: `10s`
- Actual FPS: `30`
- Actual resolution: `1080p`
- Precharge: `$0.550932`
- Final net charge: `$0.137732`
- Differential refund: `$0.413200`
- Refund records for the task: `1`
- Terminal synchronization: `PASS`
- Repeated public GET checks: `2`
- Balance before and after both repeated GETs: `$15.004706`
- Additional settlement, refund, or post-consume records after repeated GETs:
  `0`
- Output URL present in both repeated GET responses: true; value not recorded
- Output URL persisted: false

The existing task was first synchronized automatically by the preflight
background poller after the parser deployment. Its terminal transition and
actual-media settlement occurred once. Two later public task GETs returned the
same terminal status and did not change billing or add billing records.

## Production Deployment and Smoke Evidence

Henry approved deployment of the exact preflight-validated image followed by
one Standard 1080p paid production smoke. The image was not rebuilt.

Deployment evidence:

- Production container: `new-api-nightly`
- Runtime revision:
  `968670a13306d9ea57497aa48ff022a732c2112e`
- `/api/status`: PASS
- Restart count: `0`
- Existing environment, host network, no-mount shape, and `unless-stopped`
  restart policy: preserved
- MySQL connection: PASS
- Schema fingerprint before/after: unchanged
- Channel, ability, and pricing configuration fingerprint before/after:
  unchanged
- Existing Seedance ability fingerprint: unchanged
- Migrations: `0`

MediaKit configuration correction:

- `ProjectName` is not a MediaKit requirement. The adapter authenticates with
  the MediaKit API key in the Bearer authorization header.
- MediaKit rejects customer-supplied `ProjectName` and does not forward it.
- No `ProjectName` or `byteplus_project_name` was added to the Channel,
  environment, request, or database.
- Production and successful preflight matched on the required configuration
  shape: enabled Channel, API key presence, default Base URL behavior, token
  owner linkage, effective Group/ability, exact tenant alias, and canonical
  mapping.

Input gate:

- DNS and HTTPS: PASS
- Content-Type: `video/mp4`
- Range media read: PASS
- Input profile: `h264 / 1280x720 / 30fps / 10s / mp4`

Paid smoke:

- `client_request_id`: `mediakit-p0-production-20260812-01`
- LSF POST attempts: `1`
- Confirmed upstream tasks: `1`
- POST HTTP status: `200`
- Final status: `SUCCESS/100%`
- Actual profile: `10s / 30fps / 1080p / standard`
- Balance before create: `$38.192072`
- Balance after highest-FPS-tier precharge: `$37.641140`
- Balance after terminal reconciliation: `$38.054340`
- Balance after repeated GET 1: `$38.054340`
- Balance after repeated GET 2: `$38.054340`
- Precharge: `$0.550932`
- Final net charge: `$0.137732`
- Differential refund: `$0.413200`
- GroupRatio application: exactly once
- Additional billing records after two repeated GETs: `0`
- Output URL present and small-range readable: true; value not recorded
- Output URL persisted: false
- Sensitive-data scan: PASS

The task reached terminal status through normal LSF polling. No retry, second
POST, direct database write, manual refund, configuration change, migration,
container restart, or production rollback occurred.

## Test and Safety Evidence

- MediaKit adapter, realtime fetch, polling, controller, model, task-billing,
  and related regression tests: passed.
- Full repository test run reproduced six known unrelated baseline failures:
  two old moderation timestamp cases, three Claude file-conversion cases, and
  one stream-ticker case.
- Formatting, build, diff check, migration scan, and sensitive-data scan:
  passed.
- No complete signed output URL, API key, bearer token, ProjectName, SQL_DSN,
  or raw upstream response was persisted in this record or task data.
- No new task POST was used for repeated-GET validation.
- No direct upstream GET was used for repeated-GET validation.

## P1 Scope

P1 is recorded but was not implemented in this P0 release:

1. Make successful-task `Details` stop displaying `None` by reusing the
   Seedance interactive video-preview behavior.
2. Fix `LSF_ERROR_DETAIL_LOSS` so customers receive specific, sanitized
   upstream error detail instead of only `video_enhancement_failed`.
3. Reproduce, attribute, and handle each of the six known unrelated full-suite
   baseline failures. P1 must end with no unexplained test failure.

The six current baseline failures comprise two old moderation timestamp cases,
three Claude file-conversion cases, and one stream-ticker case. They were not
changed or reclassified by P0.

## Observation and Rollback Rule

Keep `new-api-nightly-before-mediakit-p0-20260812T115829Z` through the
observation window unless Henry explicitly approves retiring it. Any future
production mutation, paid task, rollback, P1 implementation, or customer-guide
change remains a separate authorization gate. Do not reuse this smoke approval
for another task.
