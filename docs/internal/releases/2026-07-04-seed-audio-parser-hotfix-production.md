# Seed Audio Parser Hotfix Production Release Record

Date: 2026-07-04
Status: `PRODUCTION_DEPLOY_PASSED`
Production deployed: yes
Customer documentation changed: no
Production smoke account/model entry: Henry/LSF internal `henrytest`

This is the internal production release record for the Seed Audio 1.0 upstream
response parser hotfix. It records a completed production deployment and
internal smoke validation. It does not authorize customer documentation
changes, new raw-content retention, production config changes, or additional
paid smoke.

## Artifact Identity

- Production container: `new-api-nightly`
- Production image: `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Image ID:
  `sha256:dda5be8c702028581a42c5489a0154b8d709f3bce61e741211454c45fed03e96`
- Source / OCI revision:
  `dcda4e551e50051ab536173f342eb21f0ca60e8d`
- Platform: `linux/amd64`
- Previous production image:
  `new-api:seed-audio-p01-reliability-admin-ui-rc1-fa002bfc`
- Preserved rollback container:
  `new-api-nightly-before-seed-audio-parser-hotfix-rc1-20260704T173731Z`
- Rollback happened: no

The hotfix used the preflight-validated RC1 candidate image. It was deployed
only to `new-api-nightly`; `new-api-preflight` and staging were not changed by
the production deployment command.

## Root Cause

LSF misclassified successful BytePlus Seed Audio responses as `invalid_json`.
The adapter path mixed capped or incorrect diagnostic response handling with
business parsing and did not correctly handle the official Seed Audio 1.0
success schema returned as top-level JSON fields:

- `audio`
- `data`
- `url`
- `duration`
- `original_duration`
- `code` / `message`

The documented `status` / `headers` / `body` wrapper is a documentation display
wrapper, not the real HTTP response body schema. `X-Tt-Logid` is read from the
HTTP response header.

## Fix Summary

- Read the full upstream response body for business parsing.
- Derive diagnostics preview and body-size bucket separately from the full body.
- Parse the official top-level Seed Audio success schema, including `audio`,
  fallback `data`, `url`, `duration`, and `original_duration`.
- Use `original_duration` for Seed Audio billing duration.
- Classify valid JSON with missing required success fields as schema error, not
  `invalid_json`.
- Reserve `invalid_json` for genuinely invalid or truncated JSON.
- Persist `X-Tt-Logid` in the top-level `seed_audio_idempotencies.x_tt_logid`
  field where available, including failed-row handling.
- Keep raw audio base64, raw upstream bodies, raw prompts, temporary output URL
  values, provider project values, bearer keys, AK/SK, SQL_DSN, and internal
  channel/group values out of diagnostics and logs.

The hotfix does not add raw base64 retention, raw prompt retention, raw upstream
body retention, temporary URL retention, object storage, artifact retention, or
data-flywheel behavior.

## Production Runtime Verification

- `/api/status`: HTTP `200`
- Runtime state after deploy and smoke: `running=true`
- Restart count after deploy and smoke: `0`
- `OOMKilled`: `false`
- StartedAt: `2026-07-04T17:37:31.964033766Z`
- Production MySQL schema check: passed with
  `seed_audio_idempotencies.x_tt_logid` and nullable
  `seed_audio_idempotencies.error_diagnostics` present.

Verification output was sanitized. No SQL_DSN, credentials, ProjectName,
internal channel/group value, raw prompt, raw upstream body, raw base64, or
temporary URL value was recorded in this release note.

## Production Smoke Passed

All production smoke used the production-approved Henry/LSF internal
`henrytest` token and the internal model alias:

```text
lsf-seed-audio-1.0-henrytest
```

`henrytest` is not a customer account, customer token, customer model alias, or
customer balance target. Do not use customer tokens, customer aliases, customer
groups, or customer balances for internal smoke.

Smoke A:

- Category: short text
- `client_request_id`: `prod-seed-audio-parser-a-20260704T173936Z`
- HTTP result: `200`
- Object: `audio.speech`
- Duration/original duration: `3.28` / `3.28`
- Output URL present: true; value not printed
- Error code: none

Smoke B:

- Category: complex environment/SFX description
- `client_request_id`: `prod-seed-audio-parser-b-20260704T173936Z`
- HTTP result: `200`
- Object: `audio.speech`
- Duration/original duration: `15.6` / `15.6`
- Output URL present: true; value not printed
- Error code: none
- `invalid_json`: not observed
- `seed_audio_upstream_error`: not observed

Smoke C:

- Category: replay of Smoke B with the same `client_request_id`
- HTTP result: `200`
- Replay returned the same response ID: true
- Duplicate billing: false

Post-smoke sanitized verification:

- Idempotency rows for Smoke A/B: `2`
- Completed idempotency rows: `2`
- `x_tt_logid` present: `2/2`
- `invalid_json`: `0`
- Non-empty `error_diagnostics`: `0`
- Consume log rows for Smoke A/B/C: `2`
- Duplicate consume log on replay: no
- Docker log scan: `panic=0`, `fatal=0`, `invalid_json=0`,
  `seed_audio_upstream_error=0`
- Raw base64/raw prompt/raw upstream body/temporary URL value leak: not observed

## Boundaries

- No customer documentation was changed.
- No customer token, customer alias, customer group, or customer balance was
  used for smoke.
- No production DB data was manually modified.
- No raw base64, raw prompt, raw upstream body, temporary URL value, ProjectName
  value, bearer token, AK/SK, SQL_DSN, or internal channel/group value was
  printed or stored in this release record.
- No new retention policy, troubleshooting retention, object storage, or data
  flywheel design was added by this hotfix.

## Rollback Reference

- Rollback container:
  `new-api-nightly-before-seed-audio-parser-hotfix-rc1-20260704T173731Z`
- Rollback image:
  `new-api:seed-audio-p01-reliability-admin-ui-rc1-fa002bfc`

Keep the rollback artifact available through the observation window unless
Henry explicitly approves retiring it.

## Follow-up Rule

Future Seed Audio parser or upstream-adapter changes must keep diagnostics
preview separate from business parsing. Capped diagnostic previews are evidence
for troubleshooting only; they must never replace the full response body used
for JSON schema parsing.
