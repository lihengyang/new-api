# Seedance 2.0 Mini RC1 Release Record

Date: 2026-06-26
Status: `BLOCKED_SSH_PASSWORD_SECRET_NOT_CONFIGURED`
Production deployed: no
Preflight deployed: no
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
- Preflight deployment or smoke tests. The preflight runtime is a server-side
  Docker runtime, not Macmini Docker Desktop. Henry supplied the server target
  for the next continuation pass, but non-interactive SSH could not authenticate
  without a safely injected password secret or an available key/agent session.
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

## Preflight Plan

Preflight preparation is approved for this RC1, but production remains closed.
The preflight update is currently blocked before server mutation because the
server target is now known, but the current execution environment does not have
a safe non-interactive authentication path.

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

No preflight container, preflight DB, preflight env, preflight logs, preflight
smoke, staging container, production container, production DB, production env,
or production logs were modified or queried in the continuation passes.

Required next server gate:

- Henry provides a safe non-interactive authentication path without pasting
  secrets into chat, for example a masked `PREFLIGHT_SSH_PASSWORD`/`SSHPASS`
  with `sshpass -e`, or a working SSH key/agent/ControlMaster session;
- Codex runs only non-sensitive server checks first:
  `docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'`;
- `new-api-preflight` must be found before any update;
- `new-api-nightly` production and staging containers may be identified but
  must not be touched.

Required preflight gates:

- sanitized DB proof that preflight uses MySQL and is not production DB;
- linux/amd64 image build with tag `new-api:seedance-mini-rc1`;
- container health and restart count;
- Mini 480p/720p no-video success;
- Mini 480p/720p with-video success using approved safe test asset;
- Mini 1080p rejection before task creation, billing, and upstream call;
- Mini 4K rejection before task creation, billing, and upstream call;
- Fast 4K rejection regression;
- Standard 4K regression if tenant package and cost approval allow;
- final settlement evidence, not just reservation evidence;
- redacted logs and no secret exposure.

## Production Gate

Production remains closed. Do not deploy production until Henry separately
approves production deployment after preflight passes.

`READY_FOR_PRODUCTION_APPROVAL` must not be reported until all required local,
scan, build, preflight, settlement, regression, rollback, and secret-safety
gates pass.

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

- No preflight evidence exists yet for Mini success, Mini rejection no-task
  proof, MySQL runtime behavior, route configuration, token ability, or final
  settlement.
- `web/dist` was generated locally through the Dockerfile Bun builder because
  the host shell does not currently provide `bun`.
- The server preflight target is supplied, but safe non-interactive SSH
  authentication is not configured in the current environment, so server
  container status, MySQL proof, rollback image, image load, and smoke evidence
  could not be collected.
- `HENRYTEST_API_KEY` still must be checked immediately before smoke with
  `test -n "${HENRYTEST_API_KEY:-}"`; no token value may be printed.
- The Mini reference-video duration is not read from remote media at submit
  time. RC1 uses a conservative 15s reference-input ceiling for reservation and
  requires final settlement evidence from upstream usage.
