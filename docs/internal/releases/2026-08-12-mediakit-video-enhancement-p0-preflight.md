# MediaKit Video Enhancement P0 Preflight Record

Date: 2026-08-12
Status: `PREFLIGHT_VALIDATED / PRODUCTION_NOT_APPROVED`
Customer documentation changed: no
Production touched: no

This combined internal release, runbook, and finding record closes the
BytePlus AI MediaKit Video Enhancement P0 preflight validation. It does not
authorize a production deployment, an additional paid task, a configuration
change, or a customer-guide update.

## Artifact Identity

- Branch: `codex/mediakit-video-enhancement-p0-20260811`
- Preflight container: `new-api-preflight`
- Preflight port: `3002`
- Database: isolated preflight MySQL
- Source / OCI revision:
  `968670a13306d9ea57497aa48ff022a732c2112e`
- Platform: `linux/amd64`
- Migrations for this parser fix: `0`

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

## Deferred Finding

`LSF_ERROR_DETAIL_LOSS` remains deferred. MediaKit failure responses can still
be collapsed to the generic `video_enhancement_failed` customer error. This P0
parser correction intentionally does not change that behavior.

## Next Gate Runbook

Production remains blocked until Henry explicitly approves production. A future
production gate must re-verify the exact candidate identity, preserve the live
production rollback artifact and runtime shape, confirm MySQL without exposing
credentials, deploy only the approved production container, and run only the
separately approved validation. This record does not grant those permissions.

Do not infer production readiness from preflight history if current runtime or
configuration evidence has changed. Do not reuse this paid-smoke authorization
for another task.
