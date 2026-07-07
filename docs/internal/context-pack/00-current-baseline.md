# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Current production release status as of the 2026-07-07 Seed Audio error usage
log production closeout:

- Status:
  `PRODUCTION_DEPLOY_PASSED / SEED_AUDIO_ERROR_USAGE_LOGS / ADMIN_UI_MANUAL_PASS`
- Production container: `new-api-nightly`
- Current production image:
  `new-api:seed-audio-error-logs-prod-candidate-6107eb4d`
- Current production image ID:
  `sha256:65718430ecddd230d5905ad803cc8e023a9335128eb5ac58c6d560c186cffe98`
- Current code / OCI revision:
  `6107eb4db3b4cce2291c78c69db4aa12b8422163`
- Previous production image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Previous production revision:
  `dcda4e551e50051ab536173f342eb21f0ca60e8d`
- Preserved rollback container:
  `new-api-nightly-before-seed-audio-error-logs-6107eb4d-20260707T164119Z`
- Rollback image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`

The Seed Audio error usage log production deployment used the
preflight-validated artifact built from the recorded source revision. The
rollback artifact must remain available through the observation window unless
Henry explicitly retires it. Code rollback will not automatically remove logs
columns or indexes created by AutoMigrate.

Production domain:

```text
https://ai-api.lightspeedfuture.com
```

Supported customer video endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`

Supported P1 customer endpoint:

- `GET /v1/billing/balance`

Seed Audio P0 customer release status:

- Status:
  `PRODUCTION_RELEASED / CUSTOMER_DOC_READY / TEXT_AUDIO_IMAGE_VALIDATED / PARSER_HOTFIX_DEPLOYED / ERROR_USAGE_LOGS_DEPLOYED`
- Customer audio endpoint: `POST /v1/audio/speech`
- Customer-facing modes: `text_only`, `audio_url`, `image_url`
- Retry safety: `metadata.client_request_id`
- Current Seed Audio `text_prompt` limit: `3000` characters. Older `2048`
  references are stale.
- Seed Audio combines trimmed `instructions` and trimmed `input` into the
  upstream `text_prompt`; both fields count toward the `3000` character limit.
- Final customer guide:
  `Light_Speed_Future_API_Integration_Guide_v2.2.0_Seed_Audio_Production_Edition.docx`
- v2.2.0 supersedes v2.1.4 for active onboarding unless a tenant-specific note
  says otherwise.
- P01 closeout: Seed Audio `3001` character local rejection passed with no
  charge/no upstream dispatch; `3000` characters were not rejected as too long;
  minimal paid success and replay passed without duplicate charge; sanitized
  upstream diagnostics schema is present; production `/console/token`
  page-size changes passed manual verification without flicker, request storm,
  or HTTP `429`.
- Parser hotfix closeout: the production response adapter now parses the
  official top-level Seed Audio 1.0 success fields `audio`, fallback `data`,
  `url`, `duration`, and `original_duration` from the full upstream response
  body. Diagnostics preview remains capped and sanitized, but it is not used as
  the business JSON parse input. Valid JSON schema mismatches are classified as
  schema errors, not `invalid_json`; only genuinely invalid or truncated JSON is
  `invalid_json`. Production internal Smoke A/B/C passed with the Henry/LSF
  `henrytest` testing alias `lsf-seed-audio-1.0-henrytest`; complex SFX
  generation passed, replay did not duplicate billing, and raw data leakage was
  not observed.
- Error usage log closeout: upstream-dispatched Seed Audio failures on
  `POST /v1/audio/speech` now write zero-cost `LogTypeError` usage logs.
  Admin usage logs can show redacted diagnostics including gateway request ID,
  `client_request_id`, upstream request ID, HTTP status, error code, retryable
  flag, and unpaid marker. User-facing and token-facing usage logs are
  backend-sanitized and must not expose Seed Audio diagnostics. Error logs do
  not affect balance or consume quota statistics. Request ID search supports
  exact matching across gateway request ID, `client_request_id`, and upstream
  request ID.

`metadata.client_request_id` is optional and supported for `POST /v1/videos`.

Seedance 2.0 Standard supports tenant-enabled 4K requests. Seedance 2.0 Mini
supports tenant-enabled 480p and 720p requests. Mini 1080p and Mini 4K remain
rejected before task creation, billing, and upstream calls. Fast 1080p and Fast
4K requests remain rejected before task creation, billing, and upstream calls.

Scheme B production smoke passed for billing balance, Mini high-resolution
rejection, Mini no-video exact final billing, Mini video-input exact final
billing, and Fast 4K rejection regression. Post-fix Mini smoke tasks had zero
final-billing delta. No additional paid production smoke is authorized by this
baseline.

The original Mini billing incident affected four Mini SUCCESS tasks in the
audited production window. Token-level quota credit of `364016` was completed
for the two verified live token targets. Smoke tasks were not compensated.
Customer Mini usage was manually paused outside system configuration; no
system Mini access toggle was changed or restored.

Customer Mini documentation remains unpublished until Henry separately approves
publication.

Customer guide archive status:
`Light_Speed_Future_API_Integration_Guide_v2.2.0_Seed_Audio_Production_Edition.docx`
is the active onboarding guide unless a tenant-specific note says otherwise.
`Light_Speed_Future_API_Integration_Guide_v2.1.4` remains historical and is
superseded for active onboarding. Archive and publication work must remain
customer-safe and must not include upstream model IDs, ProjectName values,
internal channel/group IDs, release smoke evidence, test balances, or raw
reference/output URLs. Final customer DOCX/PDF visual QA is a Microsoft Word /
Office / Henry manual review gate; LibreOffice is only a quick automated
preview and structure smoke.

Asset Library paths are supported according to the tenant package.

Cancel, delete, and list video tasks are not part of the current default scope unless separately enabled in writing.

Customer docs must not expose internal routing, ProjectName, AK/SK, channel names, group names, upstream model names, `token_id`, DB schema, or billing internals.
