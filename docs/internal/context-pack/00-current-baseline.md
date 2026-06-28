# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Current production release status as of the 2026-06-28 Seedance Mini Scheme B
billing hotfix, production validation, and quota correction:

- Status: `PRODUCTION_DEPLOY_PASSED`
- Production container: `new-api-nightly`
- Current production image: `new-api:seedance-mini-billing-scheme-b-rc1`
- Current production image ID prefix: `c991ffe082dc`
- Current code / OCI revision: `9ef110f928f40a88bf426432db08807b9edf89cb`
- Previous production image: `new-api:seedance-mini-rc1`
- Previous production revision: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Preserved rollback container:
  `new-api-nightly-before-seedance-mini-billing-scheme-b-20260628T030527Z`

The Scheme B hotfix production deployment used the preflight-validated
artifact. It was not rebuilt from docs-only commits. The rollback artifact must
remain available through the observation window unless Henry explicitly retires
it.

Production domain:

```text
https://ai-api.lightspeedfuture.com
```

Supported customer video endpoints:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`

Supported P1 customer endpoint:

- `GET /v1/billing/balance`

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
for the two verified live token targets. Smoke tasks were not compensated, and
Mini customer access remains unrestored.

Customer Mini documentation remains unpublished until Henry separately approves
publication.

Customer guide archive status: `Light_Speed_Future_API_Integration_Guide_v2.1.4`
is the current customer guide designation, and
`Light_Speed_Future_API_Integration_Guide_v2.1.3` is superseded. Archive and
publication work must remain customer-safe and must not include upstream model
IDs, ProjectName values, internal channel/group IDs, or release smoke evidence.
The local archived customer artifact is the DOCX under `docs/customer/`; PDF
generation remains blocked until the local document-rendering dependency is
repaired or Henry supplies a customer-safe PDF artifact.

Asset Library paths are supported according to the tenant package.

Cancel, delete, and list video tasks are not part of the current default scope unless separately enabled in writing.

Customer docs must not expose internal routing, ProjectName, AK/SK, channel names, group names, upstream model names, `token_id`, DB schema, or billing internals.
