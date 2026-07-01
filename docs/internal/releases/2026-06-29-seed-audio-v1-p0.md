# Seed Audio 1.0 P0 Local Implementation Note

Date: 2026-06-29
Scope: local branch `feature/seed-audio-v1`
Status: `PREFLIGHT_FUNCTIONAL_PASSED / PRODUCTION_NOT_DEPLOYED`

## Current Status Summary

- Preflight image: `new-api:seed-audio-p0-rc4`
- P0 functional modes passed in preflight: `text_only`, `image_url`, and
  `audio_url`
- DB-backed idempotency passed
- Usage logs passed
- Production, `new-api-nightly`, and `new-api-staging` were not touched
- Customer documentation was not updated

## Production Gate 0 Read-only Reconnaissance

Henry approved Gate 0 read-only production reconnaissance only. No production
mutation was performed.

- Production container: `new-api-nightly`
- Production current image: `new-api:seedance-mini-billing-scheme-b-rc1`
- Production image revision:
  `9ef110f928f40a88bf426432db08807b9edf89cb`
- Runtime state: `running=true`, `restarting=false`, restart count `0`
- Local `/api/status`: HTTP `200`
- Docker healthcheck field: none configured
- Network mode: `host`
- Existing older rollback container/image observed:
  `new-api-nightly-before-seedance-mini-billing-scheme-b-20260628T030527Z`
  using image `new-api:seedance-mini-rc1`
- Important rollback caveat: before any Seed Audio production deploy, preserve
  the current production image `new-api:seedance-mini-billing-scheme-b-rc1` as
  the new rollback artifact.
- DB connection string: present but redacted
- `DATABASE` flag: did not print a concrete value
- Remote TCP/3306 connections observed from the production app container:
  `2` total, `1` established
- DB shape is consistent with the MySQL baseline, but Gate 1 should perform a
  stronger sanitized MySQL-shape check before any approved AutoMigrate.
- Preflight remains separate: `new-api-preflight` runs
  `new-api:seed-audio-p0-rc4`
- No deploy, restart, DB mutation, smoke, `/v1/audio/speech` request, BytePlus
  call, env dump, log dump, or secret export occurred.

## Gate 1 Production Deploy Runbook

This runbook is documentation only. Seed Audio production deploy is not
approved until Henry explicitly approves the relevant gate.

### Release Identity

- Approved source commit:
  `303d4ea3efa0a3317a22d4232348bf623a9228f8`
- Last code-changing commit:
  `b6979703ca40a4560c9888c3fe90da5dadbb54c4`
- `34d276c` was superseded by docs-only HEAD `303d4ea` for release
  traceability; last code-changing commit remains
  `b6979703ca40a4560c9888c3fe90da5dadbb54c4`.
- Preflight equivalent image: `new-api:seed-audio-p0-rc4`
- Recommended production candidate image tag:
  `new-api:seed-audio-p0-prod-20260702-303d4ea`
- Production image labels must record source revision:
  `303d4ea3efa0a3317a22d4232348bf623a9228f8`

### Gate Split

- Gate 1A-0: build-only if the candidate image is missing; this requires
  explicit Henry approval and is separate from deploy.
- Gate 1A-1: production deploy and MySQL AutoMigrate only.
- Gate 1B: validation-only smoke
- Gate 1C: `text_only` paid smoke
- `audio_url` and `image_url` are second-batch production validation and
  require separate approval.
- If the candidate image is missing, Gate 1A must not proceed unless Henry
  explicitly approves build.

### Gate 1A Approval Requirements

- Henry must explicitly approve production deploy.
- Henry must explicitly approve production MySQL AutoMigrate for:
  - `seed_audio_idempotencies`
  - unique index on `token_id + client_request_id`
- Henry must confirm production UI/config is ready:
  - channel
  - `X-Api-Key`
  - group
  - token
  - tenant-facing Seed Audio alias
- Codex must not read, print, infer, export, or store secrets.

### Stronger Sanitized MySQL-shape Check

Gate 1A must produce PASS/FAIL style output only:

- `SQL_DSN_PRESENT=true`
- `SQL_DSN_MYSQL_TCP_SHAPE=true`
- `MYSQL_3306_ESTABLISHED_COUNT>=1`
- `DATABASE_FLAG_MYSQL_OR_EMPTY_ACCEPTED_WITH_TCP_SHAPE=true`

Do not print host, username, password, database name, SQL_DSN, env dump, DB
dump, or customer data.

### Rollback Preservation

Before replacing `new-api-nightly`, preserve the current production runtime as
the new rollback artifact:

- Current production image: `new-api:seedance-mini-billing-scheme-b-rc1`
- Preserve image ID and revision in sanitized form.
- Preserve container config shape:
  - ports
  - volumes
  - env key presence only
  - labels
  - command
  - entrypoint
  - restart policy
  - network mode
- Do not delete old rollback containers or images.
- Rollback restores runtime image/container only.
- Rollback must not drop `seed_audio_idempotencies` or its unique index.

### Deploy Scope

- Replace only `new-api-nightly`.
- Do not touch `new-api-preflight`, `new-api-staging`, DB containers, nginx, or
  unrelated services.
- Preserve existing runtime settings unless Henry explicitly approves a change.

### Post-deploy Health and Schema Checks

- Container name: `new-api-nightly`
- Image tag and revision match approved candidate.
- `running=true`
- `restarting=false`
- Restart count `0`
- `/api/status` HTTP `200`
- `seed_audio_idempotencies` table exists.
- Unique index exists on `token_id + client_request_id`.
- All verification output must be sanitized.

### Validation-only Smoke Plan

- Not part of Gate 1A.
- Only allowed after separate Gate 1B approval.
- One intentionally invalid `/v1/audio/speech` request may be used only after
  approval.
- Must reject before upstream dispatch and billing.
- Verify no balance change and no usage-log charge.

### Paid text_only Smoke Plan

- Not part of Gate 1A or Gate 1B.
- Only allowed after separate Gate 1C approval.
- One `text_only` paid smoke only.
- Record balance before/after, `original_duration`, expected quota, actual
  delta, replay no duplicate charge, and usage log sanitization.
- Do not run `audio_url` or `image_url` in this gate.

### Fail-stop Conditions

- Candidate image revision mismatch.
- Production DB target shape not confirmed as MySQL.
- Missing AutoMigrate approval.
- Schema verification fails.
- Container not running, restart loop, or restart count increments
  unexpectedly.
- `/api/status` not HTTP `200`.
- Validation-only bills or calls upstream.
- Paid smoke billing delta mismatch.
- Replay duplicates charge, upstream call, or consume log.
- Usage logs expose raw prompt, raw input URL, temporary output URL, upstream
  body, ProjectName, API key, bearer key, SQL_DSN, or customer data.
- Any secret-handling boundary violation.

### Post-pass Docs and Cleanup

- After production pass, update internal release note only.
- Customer docs remain blocked until Henry explicitly approves publication.
- Keep rollback artifact through the rollback window.
- After Henry approves rollback-window closure, clean temporary SSH key
  material and release-only temp files.
- Do not logout or break GitHub auth before required release documentation and
  rollback checks are complete.
- Do not print or save GitHub tokens or production secrets.

## Gate 1A-0 Build-only Result

Gate 1A-0 build-only completed on the Mac mini local Docker Desktop. No
production deploy or runtime mutation was performed.

- Built image tag: `new-api:seed-audio-p0-prod-20260702-303d4ea`
- Image ID:
  `sha256:de16a79e83920990eb351aaaa7e4c86c9e634d3281957ac25a4cd9c3e043cd1d`
- Platform: `linux/amd64`
- Revision label: `303d4ea3efa0a3317a22d4232348bf623a9228f8`
- Source label: `feature/seed-audio-v1`
- Build source commit frozen at:
  `303d4ea3efa0a3317a22d4232348bf623a9228f8`
- Current docs HEAD after build restored to:
  `1e0a0c10366b6450d5bbd60bb554d21780174fdd`
- Working tree: clean
- Candidate image currently exists in Mac mini Docker Desktop only. Gate 1A-1
  must explicitly approve the production server image transfer/load or
  server-side build path before deploy.
- No deploy, production mutation, smoke, BytePlus call, restart, container
  replacement, DB mutation, or `/v1/audio/speech` request occurred.

## Product Boundary

Seed Audio 1.0 is implemented as a separate LSF audio product family on the existing `/v1/audio/speech` endpoint. The Seed Audio path is selected only when the customer model alias starts with `lsf-seed-audio-1.0`. Normal audio speech models continue through the existing relay path.

Customer aliases are preserved in responses. The upstream BytePlus model is fixed internally as `seed-audio-1.0` and is not returned to customers.

## P0 Request Contract

Supported inputs:

- text-only
- `metadata.references[].type=audio_url`
- `metadata.references[].type=image_url`

P0 rejects:

- customer-supplied ProjectName-like routing fields
- `audio_data`, `image_data`, base64-style inline inputs, `asset://`, `file://`, and data URLs
- raw voice/speaker clone fields
- `max_duration_hint`
- non-HTTPS reference URLs, localhost hosts, and private IP literals
- mixed audio and image references
- more than 3 audio URL references or more than 1 image URL reference
- non-`mp3` response formats

Request bodies are capped at 128 KB before upstream dispatch.

## Billing Invariant

Seed Audio bills from upstream `original_duration`, not generated `duration`.

Formula:

- base quota per second: `1250`
- precharge quota: `floor(120 * 1250 * audio_group_ratio)`
- settlement quota: `floor(original_duration * 1250 * audio_group_ratio)`

Validation failures do not charge. Upstream failures, timeouts, missing `original_duration`, missing URL, and invalid customer reference URLs refund the Seed Audio precharge.

Insufficient balance uses HTTP 402 with code `seed_audio_precharge_required`.

## Idempotency

Seed Audio supports `metadata.client_request_id` with DB-backed idempotency.
Redis is not required for Seed Audio P0.

State is stored in the `seed_audio_idempotencies` table with a unique key on
`token_id` + `client_request_id`. The table is migrated through GORM and must
remain compatible with MySQL, PostgreSQL, and SQLite.

Stored values contain sanitized metadata only: request HMAC, status, LSF audio
ID, X-Tt-Logid, temporary URL, URL expiry, durations, actual quota, expiry
timestamps, and sanitized error code/status. The HMAC is computed with the
server-side crypto secret; raw prompts, customer reference URLs, ProjectName,
X-Api-Key, channel keys, bearer keys, and raw upstream JSON are not stored.

When `client_request_id` is present, a DB pending record is created before
billing and upstream dispatch. A duplicate completed request returns the cached
Seed Audio response, a duplicate pending request returns `idempotency_in_progress`,
and a duplicate failed request asks the client to use a new `client_request_id`.
Expired cached results return `idempotency_result_expired` until the tombstone
window expires. If `client_request_id` is omitted, the request proceeds without
idempotency.

RC3 local remediation preserves `metadata.client_request_id` on completed
idempotency replay responses. Replay still returns the cached Seed Audio result
and must not bill again or dispatch upstream again.

## Usage Logs

Seed Audio is synchronous and does not create video tasks. Empty task logs are
therefore expected for `/v1/audio/speech`.

Successful Seed Audio settlement writes one sanitized consume log entry with
the tenant-facing alias, quota, duration/original duration, request path,
reference mode, and seconds-based billing metadata. It must not store raw
prompts, customer reference URLs, temporary output URLs, upstream response
bodies, ProjectName, X-Api-Key, bearer keys, or audio base64 data.

RC3 usage-log visibility remediation confirms Seed Audio writes the same
`logs` table through `model.RecordConsumeLog` and is returned by the existing
admin `/api/log/` and user `/api/log/self` usage-log list APIs. Seed Audio
consume rows are filterable by model alias, token name, group, and request ID;
admin logs also retain the normal channel filter path. The usage-log UI renders
Seed Audio rows as seconds-based billing, showing original-duration usage,
final quota, reference mode, and present/absent output URL status without
printing or storing raw prompts, input URLs, temporary URLs, upstream bodies, or
secrets. Prompt/completion token columns remain zero by design because Seed
Audio is billed by seconds, not tokens.

Henry manually confirmed the RC3 preflight Usage Logs UI on 2026-07-01:

- usage-log visibility: PASS
- UI display: Seed Audio seconds-based billing
- model alias: `lsf-seed-audio-1.0-henrytest`
- group ratio: `2.0000x`
- original duration: `3.28s`
- charge: `$0.016400`
- reference input: `text_only`
- empty task log is expected for synchronous `/v1/audio/speech`
- URL reference smoke tests had not yet been run at the time of this UI check
- production was not touched

Phase 4 URL reference smoke status on 2026-07-01:

- `image_url` smoke on RC3: PASS
- `image_url` original duration: `3.2s`
- `image_url` group ratio: `2.0000x`
- `image_url` charge: `$0.016000`
- `image_url` replay did not duplicate charge
- `image_url` usage-log success count: `1`
- `audio_url` initial smoke on RC3 failed safely with HTTP 503
  `seed_audio_upstream_error`
- `audio_url` balance delta: `$0`
- `audio_url` used one upstream attempt only; no retry was performed
- local audit verified `audio_url` request mapping to upstream
  `references[].audio_url`
- local audit verified `image_url` request mapping remains upstream
  `references[].image_url`
- local audit verified text prompts such as `@Audio1` are preserved and not
  rewritten
- local fix `b6979703ca40a4560c9888c3fe90da5dadbb54c4` classifies
  audio/media resource download or access failures as HTTP 400
  `invalid_reference_url`
- generic upstream or server failures remain `seed_audio_upstream_error`
- no raw reference URLs, raw upstream bodies, prompts, X-Api-Key values, bearer
  keys, ProjectName values, or customer data were printed or stored as part of
  the fix evidence
- `audio_url` smoke remained pending until RC4 redeploy and Phase 4R retry
- no deploy, push, paid smoke, `/v1/audio/speech` request, or BytePlus call was
  performed after the local fix
- production, `new-api-nightly`, and `new-api-staging` were not touched
- customer documentation was not updated

Phase 4R `audio_url` retry status on 2026-07-01:

- preflight image: `new-api:seed-audio-p0-rc4`
- `audio_url` retry using a public HTTPS MP3 reference: PASS
- `audio_url` original duration: `3.83s`
- `audio_url` group ratio: `2.0000x`
- `audio_url` actual quota: `9575`
- `audio_url` charge / balance delta: `$0.019150`
- billing matched formula:
  `floor(original_duration * 1250 * group_ratio)`
- replay idempotency: PASS
- replay returned the same response ID
- replay preserved matching `metadata.client_request_id`
- replay did not duplicate charge
- usage-log visibility: PASS
- `audio_url` usage-log count: `1`
- duplicate replay usage log: no
- raw reference URL and temporary output URL were not printed
- the previous samplelib `audio_url` smoke failure was safe: HTTP 503
  `seed_audio_upstream_error`, zero balance delta, and no retry
- RC4 includes local fix `b6979703ca40a4560c9888c3fe90da5dadbb54c4`,
  which maps audio/media resource download or access failures to HTTP 400
  `invalid_reference_url`
- all P0 reference modes have now been validated in preflight:
  `text_only`, `image_url`, and `audio_url`
- base64 `audio_data` and `image_data` remain unsupported in P0
- production, `new-api-nightly`, and `new-api-staging` were not touched
- no push, deploy, paid smoke, `/v1/audio/speech` request, or BytePlus call was
  performed after this documentation update
- customer documentation was not updated

`GET /v1/billing/balance` is unchanged. It continues to return token-level
wallet balance from token quota fields only. Seed Audio can change the balance
amount through normal precharge and settlement, but it does not change the
balance endpoint route, controller, schema, or channel independence.

## Upstream

Seed Audio sends one POST request to:

`https://voice.ap-southeast-1.bytepluses.com/api/v3/tts/create`

Headers:

- `Content-Type: application/json`
- `Accept: application/json`
- `X-Api-Key: <channel key>`

There is no automatic retry after upstream dispatch. The default timeout is 150 seconds and is clamped to 60-180 seconds via `SEED_AUDIO_UPSTREAM_TIMEOUT_SECONDS`.

## Metrics

Added expvar counters:

- `seed_audio_requests_total`
- `seed_audio_validation_failed_total`
- `seed_audio_precharge_rejected_total`
- `seed_audio_upstream_timeout_total`
- `seed_audio_upstream_error_total`
- `seed_audio_missing_original_duration_total`
- `seed_audio_missing_url_total`
- `seed_audio_invalid_reference_url_total`
- `seed_audio_idempotency_replay_total`
- `seed_audio_idempotency_conflict_total`
- `seed_audio_idempotency_unavailable_total`
- `seed_audio_actual_quota_total`

## Local Verification

Executed locally on 2026-06-29:

- `go test ./relay -run 'TestSeedAudio' -count=1`
- `go test ./controller ./dto ./relay -count=1`

The first broader package run inside the sandbox hit the known listener restriction for existing `httptest.NewServer` tests. The same package test command passed outside the sandbox.

Executed locally on 2026-06-30 for DB-backed Seed Audio idempotency:

- `go test ./model -run 'TestSeedAudio|TestCreateSeedAudio|TestCompleteSeedAudio|TestFailSeedAudio|TestReclaimSeedAudio' -count=1`
- `go test ./relay -run 'TestSeedAudio' -count=1`
- `go test ./model ./controller ./dto ./relay -count=1`
- `git diff --check`

Executed locally on 2026-07-01 for RC3 replay metadata, usage log, and balance
regression remediation:

- `go test ./relay -run 'TestSeedAudio' -count=1`
- `go test ./controller -run 'TestGetBillingBalance' -count=1`
- `go test ./model ./controller ./dto ./relay -count=1`
- `git diff --check`

## Preflight Checklist

Before any preflight or production use:

- Create a dedicated Seed Audio channel per tenant with the BytePlus Seed Speech ProjectName and X-Api-Key configured admin-side only.
- Map only tenant aliases such as `lsf-seed-audio-1.0-<tenant>` to the Seed Audio channel.
- Use a dedicated Seed Audio group/token; do not reuse Seedance video pricing groups.
- Confirm the `seed_audio_idempotencies` table exists with a unique index on
  `token_id` + `client_request_id` before enabling `metadata.client_request_id`.
- Run one unpaid validation-only smoke first.
- Run any paid upstream smoke only after explicit approval and with sensitive parameters supplied via approved secret handling.
- Do not update customer documentation until separately approved.
