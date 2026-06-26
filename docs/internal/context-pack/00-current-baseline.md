# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Current production release status as of the 2026-06-26 Seedance 2.0 Mini RC1
production deployment and smoke pass:

- Status: `PRODUCTION_DEPLOY_PASSED`
- Production container: `new-api-nightly`
- Current production image: `new-api:seedance-mini-rc1`
- Image ID:
  `sha256:420a29a4dac01fae8b13ca06c54dcc793e1422c90ee45279c68c24fcd6c6d50a`
- Code / OCI revision: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Previous production image: `new-api:seedance-4k-rc2`
- Previous production image ID:
  `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Preserved rollback container:
  `new-api-nightly-before-seedance-mini-rc1-20260626T162156Z`

The Mini RC1 production deployment used the preflight-validated artifact. It
was not rebuilt from docs-only commits. The rollback artifact must remain
available through the observation window unless Henry explicitly retires it.

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

Mini RC1 production smoke passed for billing balance, Mini high-resolution
rejection, Mini 480p no-video success, retrieve, and final settlement evidence.
Mini `reference_video` production smoke was not run because it requires
separate approval for an extra paid production task. Live Standard 4K was not
rerun during Mini RC1 and remains an accepted risk from the release record.

Customer Mini documentation remains unpublished until Henry separately approves
publication.

Asset Library paths are supported according to the tenant package.

Cancel, delete, and list video tasks are not part of the current default scope unless separately enabled in writing.

Customer docs must not expose internal routing, ProjectName, AK/SK, channel names, group names, upstream model names, `token_id`, DB schema, or billing internals.
