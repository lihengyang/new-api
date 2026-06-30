# Seed Audio 1.0 P0 Local Implementation Note

Date: 2026-06-29
Scope: local branch `feature/seed-audio-v1`
Status: local implementation and targeted package tests only; no preflight, production, deploy, or push.

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

Seed Audio supports `metadata.client_request_id` with Redis TTL 2h.

Key shape: `seed_audio:idemp:{token_id}:{client_request_id}`.

Stored values contain sanitized metadata only: request HMAC, status, LSF audio ID, X-Tt-Logid, temporary URL, URL expiry, durations, actual quota, and timestamps. The HMAC is computed with the server-side crypto secret; raw prompts, customer reference URLs, ProjectName, X-Api-Key, channel keys, bearer keys, and raw upstream JSON are not stored.

If Redis is unavailable and `client_request_id` is present, the request fails closed before upstream dispatch. If `client_request_id` is omitted, the request proceeds without idempotency.

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

## Preflight Checklist

Before any preflight or production use:

- Create a dedicated Seed Audio channel per tenant with the BytePlus Seed Speech ProjectName and X-Api-Key configured admin-side only.
- Map only tenant aliases such as `lsf-seed-audio-1.0-<tenant>` to the Seed Audio channel.
- Use a dedicated Seed Audio group/token; do not reuse Seedance video pricing groups.
- Confirm Redis is available before enabling `metadata.client_request_id`.
- Run one unpaid validation-only smoke first.
- Run any paid upstream smoke only after explicit approval and with sensitive parameters supplied via approved secret handling.
- Do not update customer documentation until separately approved.
