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
