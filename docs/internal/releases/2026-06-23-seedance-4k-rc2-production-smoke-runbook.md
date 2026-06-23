# Seedance 2.0 4K RC2 Production Smoke Runbook

Date: 2026-06-23
Status: prepared, not executed
Production deployed: no
Customer documentation changed: no

This is an internal production deployment and smoke-test runbook for the
Seedance 2.0 4K RC2 candidate. It does not approve production deployment by
itself. Do not execute any production step until Henry gives explicit
production approval in the same operational context.

## Artifact Rule

Use the exact preflight-passed RC2 artifact where possible:

- image: `new-api:seedance-4k-rc2`;
- image ID:
  `sha256:25488855505bd222069ebc3ccb63d525f7302b53fa42ceaa403b28dcb9e3273b`;
- OCI revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`;
- source revision: `49a45ebfbe95681c8fd95d253bcb03e92e740b19`.

Do not rebuild from docs-only HEAD `20581712cf40c93f4b7a80be626a72253bb45fcb`.
If production uses a rebuilt image instead of the preflighted RC2 image, that
rebuilt image is a new artifact and requires separate provenance validation.
Prefer exact RC2 tar/image transfer.

## Production Smoke Scope

Henry approved production smoke tests that validate both no-video and
video-input billing, accepting a few dollars of production cost. This cost
approval does not authorize deployment by itself, and the smoke set must remain
minimal.

Recommended production smoke set after deployment approval:

1. Standard 4K no-video.
2. Standard 4K `video_url` with `role="reference_video"`.
3. Fast 4K reject.

The Standard 4K `image_url` smoke is optional because preflight already covered
it. Run it only if Henry explicitly asks for the extra production check.

Do not run repeated 4K positive tasks unless Henry separately approves the
additional production cost.

## Production Smoke Tenant Fixtures

Use only the approved production smoke token and aliases:

- token name: `henrytest`;
- Standard alias: `lsf-seedance-2.0-henrytest`;
- Fast alias: `lsf-seedance-2.0-fast-henrytest`;
- Standard route gate:
  `lsf-seedance-2.0-henrytest -> ep-20260424161808-gzbjt`;
- Fast route gate:
  `lsf-seedance-2.0-fast-henrytest -> ep-20260423114305-946j4`.

Do not print or paste the full API key, channel credentials, provider project
identifiers, internal channel or group values, or the full model_mapping JSON.

## Request Templates

These templates document the required request shape. Before execution, generate
a unique `metadata.client_request_id` for each case using the shown prefix.
Do not paste the final raw request body into the final production report.

### Standard 4K no-video

Request body shape:

```json
{
  "model": "lsf-seedance-2.0-henrytest",
  "prompt": "Production smoke: Seedance 2.0 Standard 4K no-video",
  "metadata": {
    "resolution": "4k",
    "duration": 4,
    "ratio": "16:9",
    "generate_audio": false,
    "client_request_id": "prod-seedance4k-YYYYMMDDTHHMMSSZ-standard-novideo"
  }
}
```

Expected:

- task reaches terminal success;
- effective upstream price is `$4.0/M`;
- billing reconciliation is sanitized.

### Standard 4K image_url, optional

Run this case only if Henry explicitly chooses to include the optional
production image fixture. The content item must include
`role="reference_image"`.

Request body shape:

```json
{
  "model": "lsf-seedance-2.0-henrytest",
  "prompt": "Production smoke: Seedance 2.0 Standard 4K image reference",
  "metadata": {
    "resolution": "4k",
    "duration": 4,
    "ratio": "16:9",
    "generate_audio": false,
    "client_request_id": "prod-seedance4k-YYYYMMDDTHHMMSSZ-standard-image",
    "content": [
      {
        "type": "image_url",
        "image_url": {
          "url": "https://lightspeedfuture.com/media/demo/generated-video-output.png"
        },
        "role": "reference_image"
      }
    ]
  }
}
```

Expected:

- task reaches terminal success;
- request is classified as no-video pricing;
- effective upstream price is `$4.0/M`;
- billing reconciliation is sanitized.

### Standard 4K video_url

The content item must include `role="reference_video"`.

Request body shape:

```json
{
  "model": "lsf-seedance-2.0-henrytest",
  "prompt": "Production smoke: Seedance 2.0 Standard 4K video reference",
  "metadata": {
    "resolution": "4k",
    "duration": 4,
    "ratio": "16:9",
    "generate_audio": false,
    "client_request_id": "prod-seedance4k-YYYYMMDDTHHMMSSZ-standard-video",
    "content": [
      {
        "type": "video_url",
        "video_url": {
          "url": "https://lightspeedfuture.com/media/demo/creative-demo-area.mp4"
        },
        "role": "reference_video"
      }
    ]
  }
}
```

Expected:

- task reaches terminal success;
- request is classified as with-video pricing;
- effective upstream price is `$2.4/M`;
- billing reconciliation is sanitized.

### Fast 4K reject

Request body shape:

```json
{
  "model": "lsf-seedance-2.0-fast-henrytest",
  "prompt": "Production smoke: Seedance 2.0 Fast 4K reject",
  "metadata": {
    "resolution": "4k",
    "generate_audio": false,
    "client_request_id": "prod-seedance4k-YYYYMMDDTHHMMSSZ-fast-4k-reject"
  }
}
```

Expected:

- HTTP 400;
- invalid-request error accepted by the parser;
- no task created;
- no billing log;
- no upstream call evidence.

## Fast Reject Parser Acceptance

The production smoke parser must accept any of these equivalent invalid-request
shapes:

- nested `error.type == "invalid_request_error"`;
- top-level `code == "invalid_request_error"`;
- top-level `type == "invalid_request_error"`.

The parser must still require all of the following:

- HTTP 400;
- no task created for the smoke `metadata.client_request_id`;
- no billing log for the smoke `metadata.client_request_id` or reject window;
- no upstream call evidence.

Do not treat a matching error shape as sufficient if any no-task, no-billing,
or no-upstream-call gate fails.

## Remote Execution Hygiene

Before production execution, convert this runbook into concrete private
commands or scripts for the approved host and container. Executable commands
must not contain unresolved `<...>` placeholders.

Rules:

- use concrete resolved variables before execution;
- no nested SSH and SQL quote chains for schema gates;
- run each schema, alias, token, health, smoke, and billing gate as a separate
  step with its own exit code;
- record each gate exit code and sanitized stderr summary;
- use a secret-safe database client configuration and never print the DSN,
  database host, username, or password;
- do not paste raw request bodies into the final production report;
- if a task fails, automatically collect sanitized `fail_reason` evidence
  instead of relying on UI copy.

Sanitized task failure evidence may include error code, type, parameter, and
message. It must not include provider task identifiers, provider URLs, raw
request bodies, credentials, or customer data.

## Forbidden Output Fields

Production runbook output and final reports must not include:

- full API key;
- SQL_DSN;
- database host, username, or password;
- AK/SK;
- ProjectName;
- channel credentials;
- full model_mapping JSON;
- internal channel or group raw values;
- upstream task ID;
- provider URL;
- raw request body;
- customer data.

## Required Production Report

After a separately approved production execution, the report should include
only sanitized fields:

- deployed image tag, image ID, and OCI revision;
- previous image tag and image ID;
- health status, restart count, and sanitized error count;
- MySQL and schema gate status;
- alias route gate status;
- task references as hashes only;
- client request IDs;
- terminal task statuses;
- billing reconciliation summary;
- effective upstream price checks;
- Fast 4K reject HTTP status and accepted error shape;
- no-task, no-billing, and no-upstream-call evidence for Fast 4K reject;
- rollback status.

Customer documentation must remain unchanged unless Henry separately approves
public rollout wording.
