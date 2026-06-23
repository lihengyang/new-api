# Current Baseline

Product: Light Speed Future Seedance 2.0 API proxy based on customized new-api nightly.

Current production release status as of the 2026-06-23 Seedance 2.0 4K RC2
production deployment:

- Status: `PRODUCTION_DEPLOY_PASSED`
- Production container: `new-api-nightly`
- Current production image: `new-api:seedance-4k-rc2`
- Image ID:
  `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Code / OCI revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`
- Previous production image: `new-api:seedance-moderation-rc3`
- Previous production image ID:
  `sha256:429fbd05318ffafd4a44c5262e8ad7ab29320356d24ea33e9345d40421f29a59`
- Preserved rollback container:
  `new-api-nightly-before-seedance-4k-rc2-redeploy-20260623T110257Z`

The RC2 production deployment used the preflight-validated artifact. It was
not rebuilt from docs-only commits.

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

Seedance 2.0 Standard supports tenant-enabled 4K requests. Fast 1080p and Fast
4K requests remain rejected before task creation, billing, and upstream calls.

Asset Library paths are supported according to the tenant package.

Cancel, delete, and list video tasks are not part of the current default scope unless separately enabled in writing.

Customer docs must not expose internal routing, ProjectName, AK/SK, channel names, group names, upstream model names, `token_id`, DB schema, or billing internals.
