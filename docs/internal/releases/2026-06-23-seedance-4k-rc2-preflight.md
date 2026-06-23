# Seedance 2.0 4K RC2 Preflight Release Record

Date: 2026-06-23
Status: `RC2_PREFLIGHT_PASSED`
Production follow-up: `PRODUCTION_DEPLOY_PASSED` on 2026-06-23
Customer documentation changed: no

## Candidate Identity

- Release branch: `feature/seedance-4k-resolution-billing`
- Source revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`
- Local image: `new-api:seedance-4k-rc2`
- Image ID:
  `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`
- Platform: `linux/amd64`
- OCI revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`

The RC1 candidate is obsolete for this rollout. RC2 is the only Seedance 2.0
4K candidate covered by this preflight record.

## Preflight Runtime State

At the end of validation, the isolated preflight container was running RC2:

- container: `new-api-preflight`;
- image: `new-api:seedance-4k-rc2`;
- status: running;
- restart count: `0`;
- `/api/status`: OK;
- panic, fatal, database error, SQLite, and migration-failure log pattern
  count: `0`.

The production container was not operated. The nightly container was not
operated. No production database connection, production API key, image push, or
customer documentation change was part of this preflight.

## Completed Gates

The RC2 preflight completed these gates successfully:

- RC2 image provenance and OCI revision;
- MySQL runtime gate;
- `tasks` schema gate for `token_id`, `client_request_id`,
  `client_request_hash`, and the token/client-request unique index;
- preflight tenant alias to BytePlus ModelArk endpoint route gate;
- preflight token quota and ability gate;
- Standard 4K no-video positive task;
- Standard 4K `image_url` positive task;
- Standard 4K `video_url` positive task with
  `role="reference_video"`;
- Fast 1080p reject;
- Fast 4K reject.

## Billing and Validation Summary

Standard 4K positive cases:

- no-video passed with effective upstream price `$4.0/M`;
- `image_url` passed and was classified under the no-video price path, with
  effective upstream price `$4.0/M`;
- `video_url` passed after the reference-video request shape was corrected,
  with effective upstream price `$2.4/M`.

Fast reject cases:

- Fast 1080p returned HTTP 400 `invalid_request_error`;
- Fast 4K returned HTTP 400 `invalid_request_error`;
- both reject cases created no task and produced no billing log.

The task API may serialize the validation error as either a nested OpenAI-style
error object or a top-level task error code. Preflight scripts should accept
either shape only when the HTTP status, no-task check, and no-billing check all
also pass.

## Routing Clarification

`relay/channel/task/doubao` is inherited adapter and package naming from the
new-api codebase. It must not be treated as evidence that the current LSF
Seedance upstream model ID is a Doubao model.

The Seedance tenant route for this rollout is:

```text
tenant-facing alias
  -> new-api model redirection
  -> BytePlus ModelArk endpoint ep-*
  -> Dreamina Seedance model family
```

`dreamina-seedance-*` names describe the endpoint-backed Seedance model family.
They are not the direct new-api channel mapping gate for this preflight.

Customer-facing documents must not expose endpoint identifiers, provider
credentials, provider project names, channel or group internals, database
schema, or billing implementation details.

## Preflight Runbook Lessons

Failure handling:

- task failures must collect sanitized `fail_reason` automatically;
- preflight evidence should not depend on manually copying UI error details;
- sanitized failure output may include error code, type, parameter, and message,
  but not provider task identifiers, raw request bodies, provider URLs, or
  customer data.

Remote execution:

- runnable remote runbooks must not contain unresolved `<...>` placeholders;
- each gate should report its own exit code and sanitized stderr summary;
- avoid nested SSH and SQL quoting gates when a checked-in or staged remote
  script can run the same check with clearer boundaries;
- database checks should use a private MySQL client defaults file or equivalent
  secret-safe mechanism, and must not print the DSN, host, username, or
  password.

## Production Follow-up

This preflight record did not approve production deployment by itself. After
separate Henry approval, RC2 was deployed to production and passed the
production release gates. See
`docs/internal/releases/2026-06-23-seedance-4k-rc2-production.md`.

The production follow-up:

- operated only the approved production container, `new-api-nightly`;
- preserved the previous image and rollback container;
- verified current production image, MySQL gate, schema gate, tenant routing,
  health, restart count, and error count;
- did not enable Mini;
- did not update customer documentation;
- kept output sanitized.
