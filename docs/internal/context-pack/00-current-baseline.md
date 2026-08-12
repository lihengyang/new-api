# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Current production release status as of the 2026-08-12 MediaKit Video
Enhancement P0 production closeout:

- Status: `MEDIAKIT_P0_PRODUCTION_PASS / P0_COMPLETE`
- Production container: `new-api-nightly`
- Current production image:
  `new-api:mediakit-parser-status-fix-968670a1`
- Current production image ID prefix: `c7521715e0ef`
- Current code / OCI revision:
  `968670a13306d9ea57497aa48ff022a732c2112e`
- Previous production image:
  `new-api:seedance-2.5-p1-candidate-51bdea54`
- Previous production revision:
  `51bdea5468ca7be213c02bbc540e6cde1ce993b4`
- Preserved rollback container:
  `new-api-nightly-before-mediakit-p0-20260812T115829Z`

The MediaKit P0 production deployment used the exact preflight-validated image
without rebuilding or migration. Production Standard 1080p smoke passed with
one POST, terminal actual-media billing reconciliation, two idempotent repeated
GETs, transient output URL handling, and no sensitive-data leakage. The rollback
artifact must remain available through the observation window unless Henry
explicitly retires it.

Production domain:

```text
https://ai-api.lightspeedfuture.com
```

Supported customer video endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`

Supported P1 customer endpoint:

- `GET /v1/billing/balance`

MediaKit Video Enhancement P0 status:

- Status: `PRODUCTION_VALIDATED / STANDARD_1080P_SMOKE_PASSED`
- Internal tenant alias maps to the canonical MediaKit video-enhancement model.
- MediaKit uses an API key in the Bearer authorization header; `ProjectName` is
  not a MediaKit configuration or request requirement.
- Successful production profile: `10s / 30fps / 1080p / standard`.
- Highest-FPS-tier precharge, actual-media settlement, differential refund, and
  repeated-GET billing idempotency passed.
- Output URLs remain transient and are not persisted.
- P1: interactive Details preview, sanitized upstream error detail, and closure
  of all six known unrelated baseline test failures.

Seed Audio P0 customer release status:

- Status: `PRODUCTION_RELEASED / CUSTOMER_DOC_READY / TEXT_AUDIO_IMAGE_VALIDATED / PARSER_HOTFIX_DEPLOYED`
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
