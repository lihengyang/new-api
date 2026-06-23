# Seedance 2.0 4K RC2 Production Release Record

Date: 2026-06-23
Status: `PRODUCTION_DEPLOY_PASSED`
Production deployed: yes
Customer documentation changed: no

This is the internal production release record for Seedance 2.0 4K RC2. It
records a completed deployment; this documentation update did not perform a new
production operation.

## Artifact Identity

- Release branch: `feature/seedance-4k-resolution-billing`
- Code / OCI revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`
- Production image: `new-api:seedance-4k-rc2`
- Image ID:
  `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Platform: `linux/amd64`

Production used the preflight-validated RC2 artifact. It was not rebuilt from
docs-only HEAD `5f3c8071f066e05169fbd163d0b0c7eeafb42822`.

## Production Runtime

- Production container: `new-api-nightly`
- Previous image: `new-api:seedance-moderation-rc3`
- Previous image ID:
  `sha256:429fbd05318ffafd4a44c5262e8ad7ab29320356d24ea33e9345d40421f29a59`
- Preserved rollback container:
  `new-api-nightly-before-seedance-4k-rc2-redeploy-20260623T110257Z`
- Rollback happened: no

## Gates Passed

- `/api/status`: OK
- Restart count: `0`
- Sanitized panic, fatal, database error, SQLite, and migration-failure pattern
  count: `0`
- MySQL gate: passed
- `tasks` schema gate: passed
- tenant alias to BytePlus ModelArk endpoint route gate: passed
- token `henrytest` gate: passed

The route gate confirmed the production tenant aliases map through new-api model
redirection to the expected endpoint-backed Seedance routes:

- Standard alias -> Standard `ep-*` endpoint
- Fast alias -> Fast `ep-*` endpoint

The release record intentionally does not include the full model mapping JSON,
credentials, provider project names, channel or group internals, database
connection details, or customer data.

## Production Smoke Passed

Standard positive cases from the production smoke evidence:

- Standard 4K no-video: `SUCCESS`, effective upstream price `$4.0/M`
- Standard 4K `video_url` with `role="reference_video"`: `SUCCESS`, effective
  upstream price `$2.4/M`

Fast reject case:

- Fast 4K reject: HTTP 400 `invalid_request_error`
- no task created
- no billing log
- no upstream-call evidence

No repeated paid Standard 4K no-video or `video_url` smoke was run during the
second deployment attempt. The second attempt ran the no-cost Fast 4K reject
smoke only.

## Billing Verification Rule

For successful async video tasks, `tasks.quota` is not authoritative final
billing. It is precharge or reservation evidence.

Final billing verification must use settlement logs, `actual_quota`, and final
net quota, then compare the effective upstream price to the expected price:

- Standard 4K no-video: `$4.0/M`
- Standard 4K with `video_url`: `$2.4/M`

The corrected verifier passed using settlement evidence and did not use
`tasks.quota` as the final successful-task billing value.

## Routing Clarification

`relay/channel/task/doubao` is inherited new-api adapter and package naming. It
must not be used to infer the current LSF upstream model ID.

LSF Seedance routing for this rollout is:

```text
tenant alias
  -> new-api model redirection
  -> BytePlus ModelArk endpoint ep-*
  -> Dreamina Seedance model family
```

`dreamina-seedance-*` describes the endpoint-backed model family, not the
direct new-api channel mapping gate.

## Documentation Boundary

Customer documentation was not changed in this release. Public v2.1.3 wording
or an addendum should be prepared only after Henry separately approves the
customer-facing rollout language.

Customer-facing documents must not expose endpoint identifiers, provider
credentials, provider project names, channel or group internals, database
schema, raw request bodies, provider URLs, provider task identifiers, or billing
implementation details.
