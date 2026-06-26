# Seedance 2.0 Mini RC1 Release Record

Date: 2026-06-26
Status: `READY_FOR_PRODUCTION_APPROVAL`
Production deployed: no
Preflight deployed: yes
Customer documentation changed: no

This is the internal release-preparation record for Seedance 2.0 Mini RC1.
It records local implementation and preflight-preparation evidence. Henry has
approved entering the preflight preparation phase for this RC1. This record
does not authorize production deployment, push, customer documentation
publication, production smoke, or use of customer data.

## Artifact Identity

- Release branch: `release/seedance-mini-v1`
- Local starting revision: `8736869ac22549588daebfff3a0db3b6971bb7f4`
- Runtime candidate commit: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Image tag: `new-api:seedance-mini-rc1`
- Image built: yes
- Image ID:
  `sha256:420a29a4dac01fae8b13ca06c54dcc793e1422c90ee45279c68c24fcd6c6d50a`
- OCI revision label: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Platform target: `linux/amd64`

The candidate branch was created from the local LSF customized Seedance 4K and
Moderation RC3 lineage, not from remote `main`.

## Scope

Implemented locally:

- Seedance 2.0 Mini family recognition, separate from Standard and Fast.
- Official Mini upstream model registration in the Doubao video adapter model
  list.
- Mini billing rates for 480p and 720p:
  - no-video: `3.5 USD / M tokens`, `0.0035 USD / K tokens`, ratio `1.75`
    against the repository ratio unit `0.002 USD / K tokens`;
  - with-video: `2.1 USD / M tokens`, `0.0021 USD / K tokens`, ratio `1.05`
    against the repository ratio unit `0.002 USD / K tokens`.
- Mini resolution guard:
  - allow: `480p`, `720p`;
  - reject: `1080p`, `4k`.
- Mini conservative reservation estimator using the official token formula.
- Mapped-request validation so Mini high-resolution rejection happens before
  billing reservation and upstream submission.
- Targeted unit/regression tests for Mini pricing, Mini precharge estimation,
  Mini rejection, Standard 4K pricing regression, and Fast rejection regression.

Excluded from this local pass:

- Production deployment.
- Customer guide v2.1.4 publication or addendum publication.
- New public customer endpoints.
- Asset Library scope expansion.
- Standard or Fast pricing changes.
- Database migration.

## External Basis

The Mini external facts used for this RC1 draft were provided in Henry's
release brief for this task:

- official Mini upstream model ID: `dreamina-seedance-2-0-mini-260615`;
- Mini online no-video input price: `3.5 USD / M tokens`;
- Mini online with-video input price: `2.1 USD / M tokens`;
- Mini offline inference: not supported yet;
- Mini supported output resolution: `480p`, `720p`;
- Mini unsupported output resolution: `1080p`, `4k`;
- 4K applies only to Standard Seedance 2.0, not Fast or Mini.

## Configuration Model

Mini does not introduce a new tenant architecture. It inherits the existing LSF
tenant isolation model:

```text
one tenant
= one BytePlus Project
= one new-api Group
= one or more Channels
= one or more customer Tokens
```

Customer requests must continue to use only tenant-facing aliases. The upstream
Mini model is selected by server-side model mapping and channel configuration.
Project identity remains server-side channel configuration and must not be
accepted from the customer request body.

Mini channel access remains governed by the existing `group + model + channel`
ability table/cache. A group without a Mini alias/channel ability should fail
with no available channel or model-not-allowed behavior, not fallback to
Standard or Fast.

Retrieve, settlement, and diagnose paths remain authoritative from stored
`tasks.channel_id` and task private data. External reports and customer
responses must not expose provider project names, internal channel names,
group names, channel IDs, group IDs, endpoint IDs, credentials, raw request
bodies, or customer tokens.

## Upstream Routing

Customer endpoint scope remains unchanged:

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`
- `GET /v1/billing/balance`

Mini upstream routing is server-side only:

```text
tenant-facing Mini alias
  -> new-api model redirection
  -> redacted BytePlus ModelArk route
  -> Dreamina Seedance 2.0 Mini family
```

No customer documentation should include the upstream model ID or endpoint ID.

## Billing Resolver

Mini now has a distinct billing family and does not inherit Standard 1080p/4K
or Fast behavior.

Video-input detection continues to come from request metadata content. A
`video_url` content item is classified as with-video. Image-only, audio-only,
and text-only content remain no-video in the Mini billing tests.

Mini reservation estimation uses:

```text
estimated_tokens =
  (input_video_duration + output_video_duration)
  * output_width
  * output_height
  * output_fps
  / 1024
```

Local estimator assumptions:

- output FPS: `24`;
- 480p conservative dimensions: `854 x 480`;
- 720p conservative dimensions: `1280 x 720`;
- `duration = -1` uses the conservative Mini max duration of `15s`;
- missing or invalid duration also uses `15s`;
- with-video reference duration is not read from the remote asset at submit
  time, so reservation uses a conservative `15s` input-video duration ceiling.

The estimator only raises the submit-time reservation when the formula exceeds
the shared fixed task precharge. It does not add token-estimate multipliers to
`OtherRatios`, so final polling settlement remains based on upstream usage
tokens plus the Mini price-tier ratio.

Terminal settlement remains unchanged:

- SUCCESS is the only terminal status that should become final charge.
- FAILURE, moderation failure, and timeout/expired paths should refund or avoid
  final charge through the existing async task refund path.
- `tasks.quota` is reservation/precharge evidence only.
- Final billing evidence must come from settlement logs, `actual_quota`, final
  net quota, or upstream usage token fields.

If Mini preflight cannot produce sufficient final settlement evidence, the
release must remain `billing_settlement_pending` and must not proceed to
production.

## Regression Boundaries

Standard/Fast behavior intentionally left unchanged:

- Standard 480p/720p remains on existing Standard rates.
- Standard 1080p remains on the existing Standard tier.
- Standard 4K remains Standard-only and tenant-enabled.
- Fast 1080p remains rejected.
- Fast 4K remains rejected.

## Local Tests Run

Passed:

```text
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./relay/channel/task/doubao -run 'Seedance|Mini|Billing|Resolution|VideoInput|ModelList|RequestBody|Precharge' -count=1
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./relay -run 'TestRelayTaskSubmitRejectsMapped(Fast|Mini)HighResolutionBeforeBilling' -count=1
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./controller -run 'Video|Seedance|Mini|Billing|Group|Channel|TaskSubmit' -count=1
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./model -run 'Quota|Billing|Task|Settlement|Reservation|Upstream' -count=1
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./service -run 'Quota|Billing|Task|Settlement|Refund|Recalculate' -count=1
```

Broad package sweep:

```text
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./... -run 'Seedance|Mini' -count=1
```

Result: passed after resolving the local `web/dist` embed blocker.

Frontend build used to resolve the embed blocker:

```text
docker build --target builder -t new-api-web-builder:seedance-mini-rc1 .
docker cp <temporary-builder-container>:/build/dist ./web/dist
```

The local shell had no `bun` executable and the pre-existing `web/node_modules`
tree did not match `web/bun.lock` for Semi UI. The Dockerfile Bun builder path
was used instead; it built Vite successfully from the checked-in `bun.lock`.

Earlier broader relay run:

```text
GOCACHE=/Users/henry-macmini/Dev/new-api/.cache/go-build go test ./relay -run 'Seedance|Mini|Resolution|ClientRequest' -count=1
```

Result: blocked by the sandbox listener restriction in an existing
`httptest.NewServer` client-request test. The focused Seedance resolution test
was rerun separately and passed.

## Tests Not Run

- Full `go test ./...` without a regex: not run.
- MySQL integration tests: not run locally.
- Preflight smoke tests using `henrytest`: not run because the preflight
  container/port is not present locally and `HENRYTEST_API_KEY` is absent.
- Production smoke tests: not run.

## Image Build

Passed:

```text
docker build --platform linux/amd64 -t new-api:seedance-mini-rc1 \
  --label org.opencontainers.image.revision=4262bb9a52a8fd67f339d830b87f9201ac8f7bec \
  --label org.opencontainers.image.version=seedance-mini-rc1 \
  --label org.opencontainers.image.created=2026-06-26T11:05:49Z .
```

Inspection:

- Image tag: `new-api:seedance-mini-rc1`
- Image ID:
  `sha256:420a29a4dac01fae8b13ca06c54dcc793e1422c90ee45279c68c24fcd6c6d50a`
- Platform: `linux/amd64`
- Revision label: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`
- Version label: `seedance-mini-rc1`

The image was built from a clean worktree at the runtime candidate commit.
The continuation pass re-inspected the local Docker image and confirmed the
same tag, image ID, `linux/amd64` platform, OCI revision label, and release
version label.

## Preflight Runtime Deployment

Preflight preparation is approved for this RC1, but production remains closed.
The server preflight runtime has been updated to `new-api:seedance-mini-rc1`.
Henry later confirmed that the preflight UI tenant/group/channel/token/model
mapping and server-side project configuration were completed manually. Smoke
tests are currently blocked before any API request because
`HENRYTEST_API_KEY` is absent in the current execution environment.

Continuation attempt before server target was supplied:

- local hygiene passed on branch `release/seedance-mini-v1` at
  `bc6808be9ec08ea8497bd0138a5a7a642c43a3e3`;
- `git status --short --branch` showed a clean branch before this documentation
  update;
- `git diff --stat` was empty before this documentation update;
- `git diff --check` passed;
- `PREFLIGHT_SSH_TARGET` was absent in the current shell;
- checked release runbook and prior release records did not provide a concrete
  server SSH alias;
- local SSH config was not readable or not present in this environment;
- no server `docker ps`, MySQL, log, env, or container inspection command was
  executed.

Continuation attempt after server target was supplied:

- local hygiene passed again on branch `release/seedance-mini-v1` at
  `3c11582b163ffda8af0bf4be5b6d0077d8524410`;
- `git status --short --branch` showed a clean branch before this documentation
  update;
- `git diff --stat` was empty before this documentation update;
- `git diff --check` passed;
- server target was supplied for `PREFLIGHT_SSH_TARGET`; the raw target is not
  repeated here;
- `PREFLIGHT_SSH_PASSWORD` was absent in the current shell;
- `SSHPASS` was absent in the current shell;
- `sshpass` was not available in the current shell;
- non-interactive `ssh -o BatchMode=yes` reached the server but failed
  authentication;
- no remote `docker ps`, MySQL, log, env, container inspection, image load, or
  container update command executed.

Continuation attempt after temporary SSH key authentication was supplied:

- local hygiene passed again on branch `release/seedance-mini-v1` at
  `38d08da70d9b9eaa271156464e6e36eec97ea995`;
- `git status --short --branch` showed a clean branch before this documentation
  update;
- `git diff --stat` was empty before this documentation update;
- `git diff --check` passed;
- server access used the temporary SSH-key path supplied through secure
  parameters; no root password or password automation path was used;
- initial server container check found `new-api-preflight` plus production and
  staging containers; production and staging were observed only and not
  modified;
- preflight MySQL gate passed with sanitized evidence:
  - `DATABASE=MYSQL`;
  - `SQL_DSN_SHAPE=MYSQL_REDACTED`;
  - active remote `3306` connections were present;
  - app log database initialization signal was present without printing logs;
- the server did not already have `new-api:seedance-mini-rc1`, so the image was
  transferred by temporary tarball, loaded, and the local and server temporary
  tarballs were removed;
- server image confirmation found `new-api:seedance-mini-rc1` with image ID
  prefix `420a29a4dac0`, matching the local Mini RC1 image ID;
- only `new-api-preflight` was updated;
- rollback image: `new-api:seedance-4k-rc2`;
- rollback container retained:
  `new-api-preflight-before-seedance-mini-rc1-20260626T144621Z`;
- updated preflight container:
  - image: `new-api:seedance-mini-rc1`;
  - restart policy: `unless-stopped`;
  - restart count: `0`;
  - port mapping: `3002 -> 3000`;
  - `/api/status`: OK;
  - precise sanitized failure-pattern count: `0`;
- `new-api-nightly` production and `new-api-staging` staging were not modified;
- no preflight smoke was run after the update.

Continuation attempt after Henry completed preflight UI configuration:

- local hygiene passed again on branch `release/seedance-mini-v1` at
  `b8de2717f44026a413203ba9e467a0186e0e25ea`;
- `git status --short --branch` showed a clean branch before this documentation
  update;
- `git diff --stat` was empty before this documentation update;
- `git diff --check` passed;
- `HENRYTEST_API_KEY` was absent in the current execution environment;
- no `/v1/billing/balance` request was sent;
- no Mini, Fast, or Standard smoke request was sent;
- no task, billing, or upstream evidence was produced in this blocked attempt;
- Codex did not modify tenant/group/channel/token/customer configuration.

Earlier local-only discovery is retained as non-authoritative context. Macmini
Docker Desktop is not the preflight runtime and must not be used as the blocker
for server preflight readiness:

- Docker contexts checked: `desktop-linux`, `default`;
- `new-api-preflight` container: not found;
- local port `3002`: not listening;
- local rollback image candidates present:
  - `new-api:seedance-4k-rc2`;
  - `new-api:seedance-moderation-rc3`;
- `HENRYTEST_API_KEY`: absent in the earlier local shell check; no token value
  was printed.

No raw preflight env, raw preflight logs, staging container, production
container, production DB, production env, or production logs were printed or
modified in the continuation passes.

## Preflight Smoke Evidence

Smoke run completed on 2026-06-26 using the server preflight endpoint directly
on port `3002`. No SSH tunnel was required for API traffic.

Post-smoke runtime identity check:

- container: `new-api-preflight`;
- image: `new-api:seedance-mini-rc1`;
- OCI revision label: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`;
- version label: `seedance-mini-rc1`;
- restart count: `0`;
- host port: `3002`.

Token handling:

- `HENRYTEST_API_KEY` was read from macOS Keychain using command substitution
  into a temporary shell variable;
- the token value was not printed, written to disk, committed, or recorded in
  this release record;
- the variable was unset after the smoke scripts completed.

Billing balance:

- `GET /v1/billing/balance`: HTTP `200`;
- response object: `billing.balance`;
- currency: `USD`;
- balance schema observed: `balance.available`, `balance.used`,
  `balance.unlimited`;
- raw balance values were not recorded.

Mini rejection gates were rerun in an isolated no-cost pass to avoid billing-log
time-window overlap with the successful tasks:

```text
mini_1080p_reject:
  http=400
  error_shape=top_level_code
  error_type_or_code=invalid_request_error
  expected_resolution_message=yes
  cid_ref=0a40aa6037b8
  task_count=0
  quota_sum=0
  billing_log_count_window=0
  error_log_count_window=0
  no_upstream_call_evidence=prevalidation_no_task_no_billing

mini_4k_reject:
  http=400
  error_shape=top_level_code
  error_type_or_code=invalid_request_error
  expected_resolution_message=yes
  cid_ref=287a1664a4aa
  task_count=0
  quota_sum=0
  billing_log_count_window=0
  error_log_count_window=0
  no_upstream_call_evidence=prevalidation_no_task_no_billing

fast_4k_reject:
  http=400
  error_shape=top_level_code
  error_type_or_code=invalid_request_error
  expected_resolution_message=yes
  cid_ref=d7dd5c7c274b
  task_count=0
  quota_sum=0
  billing_log_count_window=0
  error_log_count_window=0
  no_upstream_call_evidence=prevalidation_no_task_no_billing
```

Mini no-video success:

```text
case=mini_480p_novideo
submit_http=200
task_ref=41c1188df6b86bf9
metadata_client_request_id_match=yes
poll_terminal_status=completed
db_status=SUCCESS
db_progress=100%
reservation_quota=1531250
completion_tokens=40594
total_tokens=40594
settlement_log_count=1
actual_quota=248638
settlement_delta_sum=1282612
settlement_log_type_set=6
```

Mini `reference_video` success:

```text
case=mini_480p_reference_video
submit_http=200
task_ref=86d1c7e82f257769
metadata_client_request_id_match=yes
poll_terminal_status=completed
db_status=SUCCESS
db_progress=100%
reservation_quota=918750
completion_tokens=90814
total_tokens=90814
settlement_log_count=1
actual_quota=333741
settlement_delta_sum=585009
settlement_log_type_set=6
```

Final settlement evidence passed for both successful Mini cases because each
terminal `SUCCESS` task has upstream usage tokens plus one settlement log with
`actual_quota`. The release is not in `billing_settlement_pending`.

Accepted risk: Standard 4K live preflight regression was not run in this pass.
Henry explicitly accepted this boundary for Mini RC1 readiness. Existing local
regression tests still cover Standard 4K pricing behavior, and the 2026-06-23
RC2 production record remains the current live Standard 4K evidence. This
accepted risk does not authorize production deployment by itself.

Preflight evidence was collected with sanitized DB queries and aggregate Docker
log counts only. No raw env, raw logs, SQL DSN, API key, AK/SK, ProjectName,
channel/group/internal ID, customer token, or raw customer data was printed.

## Production Gate

Production remains closed. Do not deploy production until Henry separately
approves production deployment after this `READY_FOR_PRODUCTION_APPROVAL`
record.

This record means Mini RC1 is ready for Henry's production deployment approval
with the accepted Standard 4K live-preflight risk above. It does not authorize
push, production deployment, production smoke, customer documentation
publication, or customer configuration changes.

Production was not touched during this update. No production container, staging
container, production DB, production env, production log, customer token, group,
channel, or tenant configuration was modified.

## Production Deployment Checklist

Run this checklist only after Henry explicitly approves production deployment:

1. Confirm repo state and artifact identity:
   - branch: `release/seedance-mini-v1`;
   - release record status: `READY_FOR_PRODUCTION_APPROVAL`;
   - runtime/image commit: `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`;
   - image tag: `new-api:seedance-mini-rc1`;
   - preflight smoke status: passed;
   - final Mini billing settlement: passed.
2. Perform read-only production baseline checks:
   - current production container identity;
   - current production image and OCI revision;
   - restart count and health/status endpoint;
   - MySQL production DB identity/fingerprint;
   - confirm production and preflight DB targets are different.
3. Back up the production MySQL DB using the approved internal backup procedure.
   Do not paste backup contents, SQL DSN, credentials, env, or dumps into this
   record.
4. Preserve rollback state before switching production:
   - record the current production image tag, image ID, and OCI revision;
   - retain or create a rollback container/image reference for the current
     production release;
   - confirm the rollback artifact can be started without rebuilding from docs.
5. Deploy only the production container after approval:
   - switch `new-api-nightly` to `new-api:seedance-mini-rc1`;
   - keep restart policy consistent with the existing production setup;
   - do not modify `new-api-staging`;
   - do not modify tenant group/channel/token/customer configuration.
6. Verify production startup:
   - container is running;
   - restart count remains stable;
   - health/status endpoint returns OK;
   - sanitized error-pattern count is acceptable;
   - image label still reports
     `4262bb9a52a8fd67f339d830b87f9201ac8f7bec`.
7. Run the minimal approved production smoke only if Henry separately approves
   production smoke and cost, and only after Henry has completed production UI
   configuration and explicitly replied `生产 UI 配置完成，可以继续 smoke`:
   - billing balance shape;
   - Mini 480p/720p no-video success;
   - Mini 480p/720p `reference_video` success;
   - Mini 1080p rejection before task, billing, and upstream;
   - Mini 4K rejection before task, billing, and upstream;
   - Fast 4K rejection regression;
   - `GET /v1/videos/{task_id}` for successful Mini tasks;
   - final billing settlement evidence, not only `tasks.quota`.
8. Keep all production evidence sanitized:
   - no API keys, AK/SK, SQL DSN, env, raw logs, DB dumps, ProjectName,
     channel/group/internal IDs, customer bearer tokens, raw request bodies, or
     real customer data.

After a successful production runtime deployment, stop before API smoke and
report:

```text
PRODUCTION_RUNTIME_READY_CONFIG_PENDING
```

## Production UI Configuration Gate

Production `henrytest` uses a different API key from preflight. Do not reuse
the preflight smoke key for production smoke.

After the production runtime is healthy, Henry must manually configure the real
production UI before any production API smoke:

- production `henrytest` token group access;
- Mini tenant-facing alias and model mapping;
- group allowed model;
- channel model support;
- channel upstream Mini mapping and endpoint;
- server-side project injection;
- sufficient balance/quota for minimal smoke.

Codex must not modify production group, channel, token, customer, or tenant
configuration through DB, UI, or API. If production smoke later reports
model-not-allowed, channel unavailable, no available channel, or alias missing,
stop and report:

```text
BLOCKED_PRODUCTION_TENANT_CONFIG_PENDING
```

Only describe the configuration class Henry needs to adjust. Do not output
internal channel/group IDs, provider project values, or customer tokens.

## Production Key Handling

Recommended macOS Keychain service for production smoke:

```text
lsf-production-henrytest-api-key
```

Read the production `henrytest` key only into a temporary shell variable:

```text
HENRYTEST_PROD_API_KEY="$(security find-generic-password -w -a henrytest -s lsf-production-henrytest-api-key)"
test -n "${HENRYTEST_PROD_API_KEY:-}" && echo "HENRYTEST_PROD_API_KEY present"
```

Rules:

- do not print or echo the key value;
- do not write it to file, release record, git, logs, shell history, or server;
- use it only in the `Authorization` header for approved production smoke;
- after smoke, run `unset HENRYTEST_PROD_API_KEY`;
- if absent, stop and report:

```text
BLOCKED_PRODUCTION_HENRYTEST_API_KEY_ABSENT
```

## Production Smoke Polling Policy

Async video generation can be slow. During approved production smoke, queued,
running, in-progress, or processing states are not rollback reasons by
themselves.

Required polling behavior:

- keep polling `GET /v1/videos/{task_id}` patiently with bounded waits;
- do not rollback just because a task is not terminal yet;
- classify a smoke case as blocked or failed only after the agreed polling
  window, terminal failure, health regression, or explicit error evidence;
- final billing still requires settlement evidence such as `actual_quota`,
  settlement logs, net quota change, or upstream usage tokens. `tasks.quota`
  remains reservation/precharge evidence only.

## Rollback Plan

Rollback requires explicit Henry approval unless the production switch fails
before serving traffic and immediate revert is the pre-approved deployment
safety action.

1. Roll back container runtime first:
   - stop the Mini RC1 production container if it is unhealthy;
   - restore the previously recorded production image/container reference;
   - do not rebuild rollback from a later docs-only commit.
2. Verify rollback health:
   - production container running;
   - restart count stable;
   - health/status endpoint OK;
   - current image/revision matches the recorded rollback artifact.
3. Database rollback policy:
   - do not rollback DB blindly;
   - Mini RC1 introduces no planned DB migration, so expected rollback is
     container/image only;
   - if unexpected DB mutation is suspected, stop and review the production
     backup plus sanitized read-only evidence before any DB restore.
4. Post-rollback evidence:
   - record sanitized image/revision, health, restart count, and smoke result;
   - record whether any Mini production task was created before rollback;
   - keep secrets, raw env, raw logs, SQL DSN, customer tokens, and internal IDs
     out of the record.
5. Customer-facing action:
   - do not publish Mini customer documentation until production deployment and
     any approved production smoke pass;
   - if rollback happens, leave customer docs unchanged.

## Customer Documentation Impact

Do not publish customer guide v2.1.4 or a Mini addendum yet.

After production pass, customer-safe Mini documentation may include:

- existing LSF base URL and endpoints only;
- tenant-facing Mini alias only;
- supported resolutions: `480p`, `720p`;
- unsupported resolutions: `1080p`, `4k`;
- customer-safe request/response examples with placeholders;
- billing balance endpoint behavior;
- safe invalid-request wording for rejection cases.

Customer documentation must not include the upstream Mini model ID, endpoint
ID, provider project name, credentials, channel/group names or IDs, raw DB
fields, raw upstream responses, internal ratios, or Moderation Diagnose.

## Security Notes

No secrets, API keys, AK/SK, SQL DSNs, env files, DB dumps, production logs,
provider project names, channel IDs, group IDs, customer tokens, or raw
customer data were intentionally added to this release record.

Changed-file sensitive scan completed locally after the root sweep passed.
Matches were limited to:

- test-only tenant alias placeholders containing `henrytest`;
- code header literals such as `Authorization: Bearer` without secret values;
- policy and pricing terminology such as `tokens`, `token`, and `AK/SK`.

No API key, SQL DSN, AK/SK value, customer bearer token, provider ProjectName
value, raw env, channel/group ID, or customer data was printed or committed.

## Known Limitations

- Live Standard 4K preflight regression was not run in this Mini pass. Henry
  accepted this as a Mini RC1 production-approval risk boundary.
- `web/dist` was generated locally through the Dockerfile Bun builder because
  the host shell does not currently provide `bun`.
- The first continuation attempts were blocked because `HENRYTEST_API_KEY` was
  not present in the Codex execution environment. The successful smoke pass used
  the macOS Keychain path and still did not print the token.
- The Mini reference-video duration is not read from remote media at submit
  time. RC1 uses a conservative 15s reference-input ceiling for reservation; the
  successful preflight pass also verified final settlement from upstream usage.
