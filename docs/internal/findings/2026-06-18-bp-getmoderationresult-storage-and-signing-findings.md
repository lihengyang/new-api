# BP GetModerationResult Storage and Signing Findings

Date: 2026-06-18

Scope: exploration and local implementation review only. No production, preflight, external DB, or runtime state was modified or queried. The original review used the local repo plus the Desktop handoff brief. The API contract correction was subsequently verified from the BP official `GetModerationResult` PDF, V1.0.4, dated 2026-06-12, provided by Henry for the 2026-06-19 review.

## Executive Summary

- Platform video task IDs are generated locally as `task_...` and stored in `tasks.task_id`.
- BP/Seedance upstream generation IDs such as `cgt-...` are stored separately in `tasks.private_data.upstream_task_id` after a successful upstream submit.
- For video tasks that reached upstream submit successfully, later failed task polling should preserve enough data to query BP moderation by `Type=task_id` using the stored `cgt-...`.
- Upstream request IDs are not explicitly captured from headers or response metadata. They may only survive opportunistically if the upstream JSON body itself contains a request ID and is stored in `tasks.data`.
- Asset Library calls are currently pass-through relay calls. There is no local material asset table/model and no durable local store for `asset_id`, `request_id`, raw asset responses, or failed asset registration records.
- Existing no-SDK Ark signing for Asset Library can be reused for BP moderation queries, but it is currently controller-private and should be extracted or wrapped before a productized tool.
- Commit `12f0857f` contains the backend/frontend `moderation_diagnose` code. Treat it as implemented branch state, not as proof of production deployment or approval. Its action-name and request-body mismatch was corrected in the 2026-06-19 follow-up described below.
- Commit `9d49047c` contains the rc4 credential-resolution correction. It separates target resolution from credential resolution and no longer parses the task's video channel Bearer key as Asset Admin AK/SK.
- Commit `302f60be` contains the rc5 final preflight correction for manual task ownership, result semantics, library-asset UX, stale-result prevention, and focused rate-limit behavior.

## 2026-06-19 RC4 Preflight Evidence and RC5 Final Correction

Henry confirmed the following rc4 preflight facts for this rc5 task. This is the latest explicit environment evidence and supersedes the older local-only statement that rc4 still required initial MySQL preflight validation:

- Preflight MySQL startup and migrations succeeded.
- Unauthenticated Moderation Diagnose requests returned HTTP 401.
- Exact lookup succeeded for numeric task record ID, LSF `task_...`, and persisted BP `cgt-...`.
- Automatic credential mapping succeeded through Group, Asset Admin ability, ProjectName, and strict AK/SK parsing.
- GetModerationResult signing used the correct Action, Version, service, and fixed internal region.
- A positive moderation result returned real block reasons including Copyright/Safety categories.
- `NotFound.Id` was returned and displayed as a completed upstream query.
- Moderation Diagnose QPM 10 was exercised successfully; the eleventh request returned HTTP 429.
- `library_asset` without ownership mapping did not call BytePlus.

This rc5 task did not connect to a server, deploy preflight, query an external database, or modify production. The preflight facts above come from Henry's latest explicit confirmation, not from new server access during rc5 implementation.

### RC4 issue found after successful preflight

Rc4's backend already attempted automatic ownership resolution for persisted `task_...` and `cgt-...` values in manual mode. However, the rc4 frontend required a credential channel before sending every manual request. As a result, `manual + Type=task_id` could not reach the backend auto-resolution path without an unnecessary manual selection.

Rc4 also left several UX ambiguities:

- a `404 NotFound.Id` attempt retained `success=true` for compatibility but had no separate diagnostic outcome, which could be misread as a found result;
- changing source, ID, Type, or credential channel could leave a previous result visible;
- frontend validation and request failure could leave stale success state;
- `library_asset` invited a query even though automatic asset ownership is not persisted;
- attempted queries, raw response, and raw request/error fields were shown together without a clear primary-versus-advanced hierarchy.

### RC5 final behavior

Manual `task_id`:

- the credential selector is optional in the frontend;
- the backend first checks exact numeric task record ID, exact `task_...`, and exact persisted `cgt-...`;
- a matched task reuses the video-task target and automatic tenant credential resolvers;
- any supplied manual Asset Admin channel is ignored for a matched task;
- an unmatched task ID requires `asset_admin_channel_id` and returns a specific ownership-not-found message when it is absent;
- external task IDs with a selected channel still receive strict backend Asset Admin ability, enabled-status, and AK/SK validation.

Manual `asset_id` and `request_id`:

- require an Asset Admin credential channel;
- continue to validate the selected channel on the backend;
- reject ordinary video Bearer channels.

`library_asset`:

- remains protected by the backend ownership-mapping error;
- creates no attempted query and performs no BytePlus call;
- does not consume Moderation Diagnose QPM;
- the UI explains the storage limitation and provides a `Continue in manual mode` transition that preserves the asset ID, selects `asset_id`, clears prior results, and does not automatically query upstream.

Result semantics:

- `found`: the BytePlus request returned a moderation result;
- `not_found`: BytePlus returned HTTP 404 / `NotFound.Id`;
- `request_failed`: signing, proxy, network, configuration, or other upstream execution failed;
- `validation_error`: local parameters, ownership, or credential validation failed;
- `rate_limited`: the Moderation Diagnose QPM limit rejected the request.

The attempt-level `success` field remains compatible with rc4, including `success=true` for a completed HTTP 404 attempt. The new response-level `result_status=not_found` prevents the UI from presenting that outcome as a found result.

The UI now:

- clears stale results on every relevant input change, query start, validation failure, request failure, reset, and library-to-manual transition;
- shows an empty state until the current input produces a diagnostic result;
- displays result status, resolved query, resolved details, and final redacted BytePlus response first;
- keeps attempted queries, raw request body, and raw error in a collapsed Advanced diagnostics section;
- preserves every fallback attempt in `attempted_queries`;
- never displays AK/SK, ProjectName, Group, channel key, or channel settings.

Rate limiting remains scoped to Moderation Diagnose:

- QPM remains 10;
- the eleventh request returns HTTP 429 with `result_status=rate_limited`;
- rate-limited requests do not enter the diagnose handler or call BytePlus;
- unrelated admin and customer APIs are unaffected.

No asset table, field, index, or migration was added. No customer-facing documentation change is required.

## 2026-06-19 RC4 Corrective Audit

This audit was local-only. No server, preflight environment, production environment, external database, customer data, or deployment state was accessed or modified.

### Authentication boundaries confirmed from code

- Seedance video submit and polling use the selected video channel key as a Bearer API key:
  - `relay/channel/task/doubao/adaptor.go:110-114`
  - `relay/channel/task/doubao/adaptor.go:140-145`
  - `relay/channel/task/doubao/adaptor.go:281-296`
- Asset Library Admin parses the selected channel key as exactly `<AK>|<SK>` and uses Ark HMAC signing:
  - `controller/seedance_asset.go:148-158`
  - `controller/seedance_asset.go:460-465`
  - `controller/seedance_asset.go:494-513`
- Customers still call Asset Library through the normal LSF Bearer-token relay boundary. `router/relay-router.go` applies `TokenAuth` and `Distribute` before the Asset Library routes, then `RelaySeedanceAsset` performs the server-side AK/SK upstream signing:
  - `router/relay-router.go:72-85`
  - `router/relay-router.go:124-144`
  - `controller/seedance_asset.go:439-526`
- AK/SK therefore exists only in the server-side upstream forwarding layer. It is not a customer request field.

### Task persistence facts

`model.Task` persists:

- database record ID: `Task.ID`
- LSF public task ID: `Task.TaskID`
- owning user and token: `Task.UserId`, `Task.TokenId`
- selected billing/routing group: `Task.Group`
- original video channel: `Task.ChannelId`
- origin and upstream model names: `Task.Properties.OriginModelName` and `Task.Properties.UpstreamModelName`
- BP upstream generation ID: `Task.PrivateData.UpstreamTaskID`

Code references:

- `model/task.go:60-84`
- `model/task.go:96-120`
- `model/task.go:191-227`
- `controller/relay.go:658-692`

The BP `cgt-...` ID is stored in the JSON-serialized `private_data` structure under `upstream_task_id`, not in `task_id` and not in a dedicated SQL column. Rc4 adds exact database-specific JSON extraction for SQLite, MySQL, and PostgreSQL, limits the candidate result set, then parses the stored structure and compares the ID again:

- `model/task.go:350-383`

### Current Asset Library tenant and channel selection

Asset Library internal model names are fixed code-level names, not tenant aliases:

- virtual routes: `seedance-virtual-asset-admin`
- real-human routes: `seedance-real-human-asset-admin`

`middleware/distributor.go:272-275` assigns those internal models from the route. Normal channel selection then uses the authenticated request's effective Group plus the internal model through `CacheGetRandomSatisfiedChannel` (`middleware/distributor.go:130-136`). The persistent relation is `abilities(group, model, channel_id)` (`model/ability.go:16-23`).

There is no direct video-channel-to-Asset-Admin-channel foreign key or relation field in `model.Channel` (`model/channel.go:21-54`). `UserId` and `TokenId` identify task ownership, but neither maps directly to a channel.

The existing code/data combination that can prove the same tenant boundary is:

1. the persisted `tasks.group`;
2. an enabled Asset Admin ability with the same `abilities.group`;
3. one of the two fixed Asset Admin internal models;
4. an enabled channel with a strictly valid single `<AK>|<SK>` key;
5. exact `ByteplusProjectName` equality when the original video channel has a configured project.

This mirrors normal Asset Library distribution semantics instead of deriving a relationship from key shape or field names.

### RC4 resolver behavior

Rc4 separates:

- moderation target resolution: `controller/moderation_diagnose.go:224-405`
- moderation credential resolution: `controller/moderation_diagnose.go:406-550`

Target resolution:

- accepts exact task database ID, exact `task_...`, or exact persisted `cgt-...`;
- does not fall back from numeric record lookup to a textual task ID;
- does not accept a client override for `task.channel_id`;
- loads the persisted video channel only for tenant/project validation;
- enforces the 14-day lookup window;
- does not return Group, ProjectName, channel key, channel configuration, or credential channel ID.

Credential resolution:

- queries enabled Asset Admin abilities for the persisted task Group;
- excludes ordinary video-only channels even if their key contains `|`;
- parses only eligible Asset Admin channel keys as strict single `<AK>|<SK>`;
- applies exact ProjectName matching when the video channel has a configured project;
- sorts deterministically by priority descending and channel ID ascending;
- accepts multiple candidates only when both ProjectName and credentials are identical;
- returns `credential mapping ambiguous` instead of guessing when candidates conflict;
- stores only the final credential channel ID in the admin audit metadata.

### Asset ownership storage

The repository still has no durable Asset Library asset-registration model or table. `RelaySeedanceAsset` relays the redacted upstream response directly and does not persist `asset_id`, Asset Library `request_id`, channel, Group, tenant, or registration ownership (`controller/seedance_asset.go:467-526`).

Therefore rc4 `library_asset` automatic mode returns:

```text
asset ownership mapping is not available; use manual mode
```

No migration or temporary asset table was added.

### Failed candidate history

- rc1: used the wrong action/body contract (`GetAIGCModerationResult` and ProjectName in the body). Discarded.
- rc2: corrected the action/body but still depended on channel Region configuration. Failed candidate; not production-approved.
- rc3: used the task's persisted video `channel_id` as the moderation credential channel. Preflight returned `asset admin channel key must use AK|SK format` because the video Bearer key was sent to the Asset Admin parser. Rc3 must not enter production.
- rc4: resolved task ownership first, then selected a separate Asset Admin ability channel within the verified tenant boundary. MySQL preflight and real result/NotFound/QPM checks succeeded, but the manual frontend validation and result-UX issues above make rc4 superseded by rc5. Rc4 must not enter production.
- rc5: preserves the verified rc4 backend mapping and signing behavior while correcting manual task flow, result status semantics, library-asset UX, stale-result handling, and rate-limit response semantics. Rc5 remains a local candidate until its own authorized preflight and explicit production approval.

Production was not changed by this corrective work.

## Source Notes

Required context was read from:

- `docs/internal/context-pack/00-current-baseline.md`
- `docs/internal/context-pack/01-environments.md`
- `docs/internal/context-pack/02-tenant-model.md`
- `docs/internal/context-pack/03-security-boundaries.md`
- `docs/internal/context-pack/04-billing-usage.md`
- `docs/internal/context-pack/05-release-workflow.md`
- `docs/internal/context-pack/06-common-wrong-assumptions.md`
- `docs/internal/seedance-p1-preflight-rollout-runbook.md`
- `docs/internal/releases/2026-05-31-seedance-p1-client-request-rc2.md`
- `docs/customer/Light_Speed_Future_API_Integration_Guide_v2.1.2.docx`
- `/Users/henry-macmini/Desktop/Codex_Handoff_BP_GetModerationResult_Exploration_2026-06-18.md`
- BP official `GetModerationResult` API PDF, V1.0.4, 2026-06-12, provided by Henry for the 2026-06-19 review

## 1. Platform Task ID Generation and Storage

Platform task IDs are generated in `model.GenerateTaskID()` as `task_` plus a random 32-character key (`model/task.go:155`).

Task submit pre-generates the public ID before upstream submit:

- `relay/relay_task.go:192` sets `info.PublicTaskID = model.GenerateTaskID()` when absent.
- `relay/common/relay_info.go:655` documents `PublicTaskID` as the public `task_xxxx` ID used to avoid exposing upstream IDs.

The local task row stores this public ID in `tasks.task_id`:

- `model/task.go:62` defines `Task.TaskID`.
- `model/task.go:205` uses `PublicTaskID` when initializing a task.
- `model/task_reservation.go:56` generates or uses the provided task ID when creating an idempotency reservation.
- `controller/relay.go:658` inserts the normal task row after submit; reservation finalization uses `model.FinalizeTaskReservation()` at `controller/relay.go:669`.

The Doubao/Seedance adapter returns the public ID to customers, not the upstream ID:

- `relay/channel/task/doubao/adaptor.go:262` creates an OpenAI video response.
- `relay/channel/task/doubao/adaptor.go:263` sets both `id` and `task_id` to `info.PublicTaskID`.

## 2. BP Upstream Generation ID Persistence

The Seedance/Doubao submit response is parsed as:

- `relay/channel/task/doubao/adaptor.go:64` defines `responsePayload.ID`.
- `relay/channel/task/doubao/adaptor.go:250` unmarshals the upstream response.
- `relay/channel/task/doubao/adaptor.go:257` requires `dResp.ID` to be non-empty.
- `relay/channel/task/doubao/adaptor.go:268` returns `dResp.ID` as `UpstreamTaskID` and stores the raw submit response as `TaskData`.

After successful submit, the upstream ID is persisted in private task data:

- `controller/relay.go:691` calls `applyTaskPrivateData()`.
- `controller/relay.go:692` stores `result.UpstreamTaskID` in `TaskPrivateData.UpstreamTaskID`.
- `model/task.go:116` defines `TaskPrivateData`.
- `model/task.go:118` defines `upstream_task_id`.

The helper used throughout polling retrieves the upstream ID first:

- `model/task.go:137` defines `GetUpstreamTaskID()`.
- `model/task.go:140` returns `PrivateData.UpstreamTaskID` when present.
- `model/task.go:143` falls back to `TaskID` for older rows.

Polling uses the upstream ID, not the public `task_...`:

- `service/task_polling.go:110` calls `task.GetUpstreamTaskID()`.
- `service/task_polling.go:116` keys the polling map by upstream ID.
- `service/task_polling.go:362` sends `task.GetUpstreamTaskID()` to the provider fetch method.

Conclusion: for successful Seedance submit rows, `cgt-...` should be available in `tasks.private_data.upstream_task_id` even after `tasks.data` is later overwritten by polling.

## 3. Failed Video Task Data Retention

There are two materially different failure phases.

For failures after successful upstream submit:

- The row already has `private_data.upstream_task_id`.
- `service/task_polling.go:395` stores the latest upstream poll response in `tasks.data` after redacting only large base64 video fields.
- `service/task_polling.go:453` handles terminal failure.
- `service/task_polling.go:460` stores the provider error message in `tasks.fail_reason`.
- `service/task_polling.go:475` persists the terminal update with CAS.

This is enough for `GetModerationResult` by `Type=task_id` when the upstream ID is a BP generation ID such as `cgt-...`.

For failures before a successful upstream submit:

- `relay/relay_task.go:249` returns an error immediately on non-200 upstream submit responses.
- If there is no `metadata.client_request_id` reservation, no task row is created.
- If there is a reservation, `controller/relay.go:706` may mark it failed, but `model/task_reservation.go:140` only writes `status`, `progress`, `quota`, `fail_reason`, and `updated_at`; it does not store upstream response data or an upstream ID.

This is not enough to query `GetModerationResult` unless some external operator already captured a BP `request_id`.

Upstream request ID status:

- Search found no explicit capture of BP `ResponseMetadata.RequestId`, response headers, or Ark request IDs for Seedance video tasks.
- `controller/moderation_diagnose.go:268` in commit `12f0857f` tries to infer request IDs from `tasks.data` keys such as `request_id`, `requestId`, and `RequestId`, but that only works if the upstream JSON body contains such a field.
- Generic gateway logs have their own `request_id` field (`model/log.go:38`), but that is the new-api request ID, not BP `ResponseMetadata.RequestId`.

## 4. Admin Task Detail and Exposure

The admin/user task list returns a DTO, not the full model:

- `relay/relay_task.go:682` maps `model.Task` to `dto.TaskDto`.
- `relay/relay_task.go:703` includes `Data`.
- It does not include `PrivateData`, so `upstream_task_id` is intentionally not exposed in the task log API.

Implication: a future diagnose tool should resolve the upstream ID server-side from `private_data`, not rely on the existing task log UI exposing it.

## 5. Asset Library Storage

Asset Library endpoints are registered as pass-through POST paths:

- `router/relay-router.go:124` through `router/relay-router.go:144`.
- `controller/seedance_asset.go:47` maps each gateway path to a BP Ark action.
- `controller/seedance_asset.go:53` maps virtual `assets/create` to `CreateAsset`.
- `controller/seedance_asset.go:64` maps real-human `assets/create` to `CreateAsset`.

The controller prepares, signs, sends, redacts, and copies the upstream response:

- `controller/seedance_asset.go:467` parses the request.
- `controller/seedance_asset.go:472` injects/normalizes backend-managed fields.
- `controller/seedance_asset.go:488` marshals the upstream body.
- `controller/seedance_asset.go:502` signs the upstream request.
- `controller/seedance_asset.go:513` calls upstream.
- `controller/seedance_asset.go:520` reads the upstream response.
- `controller/seedance_asset.go:525` redacts backend project values from the response.
- `controller/seedance_asset.go:526` copies the response back to the caller.

There is no local model/table for material assets in this repo, and the Asset Library controller does not persist success or failure responses. Therefore:

- Successful `asset_id` values are not stored locally unless the caller stores them.
- Upstream `request_id` values are not stored locally unless the caller stores them.
- Failed asset registration/moderation records are not durable in new-api.
- For material asset moderation failures, local new-api can only diagnose by `asset_id` or `request_id` if the operator supplies that value manually.

The diagnose resolver introduced by commit `12f0857f` reflects this limitation:

- `controller/moderation_diagnose.go:288` treats `library_asset` as manually supplied IDs.
- `controller/moderation_diagnose.go:293` requires `channel_id`.
- `controller/moderation_diagnose.go:297` builds an `asset_id` query from the supplied value.
- `controller/moderation_diagnose.go:298` optionally adds a supplied `request_id`.

## 6. BP/Ark Signing Reuse

Existing reusable pieces:

- `controller/seedance_asset.go:148` parses an `AK|SK` channel key.
- `controller/seedance_asset.go:161` builds the Ark regional base URL as `https://ark.<region>.byteplusapi.com`.
- `controller/seedance_asset.go:333` builds an action-style URL with `Action` and `Version=2024-01-01`.
- `controller/seedance_asset.go:350` signs Ark requests using HMAC-SHA256, `X-Date`, `X-Content-Sha256`, service `ark`, and the configured region.

This signer is directly compatible with the handoff’s no-SDK requirement and should be reused rather than installing a host-level SDK.

Current caveats:

- The helper is unexported in `controller`. It can be reused by controller code, but a productized implementation should extract it to a small internal package or dedicated helper to avoid coupling Asset Library and moderation diagnose controllers.
- `relay/channel/jimeng/sign.go:147` hard-codes region `cn-north-1` and service `cv`, so it is only a pattern, not the correct helper for Ark `GetModerationResult`.
- Commit `12f0857f` already calls `buildSeedanceAssetTargetURL()` and `signSeedanceAssetAdminRequest()` (`controller/moderation_diagnose.go:364` and `controller/moderation_diagnose.go:380`).

Official API contract correction:

- BP PDF V1.0.4 dated 2026-06-12 defines the action as `GetModerationResult`.
- The action URL uses `Version=2024-01-01`.
- The JSON request body contains only `Id` and `Type`.
- Valid `Type` values are `task_id`, `asset_id`, and `request_id`.
- `ProjectName` is not a request parameter for this interface.
- The 2026-06-19 follow-up changes the implementation from `GetAIGCModerationResult` to `GetModerationResult`, removes `ProjectName` from the request body, and adds contract and Ark signing tests.
- Channel selection remains server-side: the selected task/channel supplies AK/SK and proxy configuration. For `video_task`, the persisted task `channel_id` remains authoritative and cannot be overridden by the request.

Moderation Region handling:

- `Region` is not a `GetModerationResult` JSON request parameter.
- BP PDF V1.0.4 uses `ap-southeast-1` in the official signing example.
- The current new-api admin surface does not provide a usable BytePlus Region configuration entry for Moderation Diagnose.
- Moderation Diagnose therefore uses the internal constant `ap-southeast-1`; administrators do not need to configure it on the channel.
- The constant is used only to construct the default Ark endpoint and the AK/SK credential-scope signature.
- Region is not included in the request body, response DTO, audit log, or frontend form.
- Asset Library keeps its existing channel-configured Region behavior; this correction is scoped only to Moderation Diagnose.

## 7. Historical Diagnose Commit State — Superseded by RC4 and RC5

Commit `12f0857f` includes diagnose surface area:

- Backend controller: `controller/moderation_diagnose.go`
- Backend tests: `controller/moderation_diagnose_test.go`
- Admin route: `router/api-router.go:209`
- Frontend page: `web/src/pages/ModerationDiagnose/index.jsx`
- Sidebar/render wiring in `web/src/App.jsx`, `web/src/components/layout/SiderBar.jsx`, and related files.

The following behavior describes the original `12f0857f` implementation and is retained only as historical evidence:

- Video task mode resolves by local task row ID or public `task_...`.
- It refuses to use public `task_...` as a BP generation ID unless the extracted ID has `cgt-` prefix.
- It falls back to `private_data.upstream_task_id`.
- Library asset mode does not look up a local asset record because none exists; it requires supplied `asset_id` and `channel_id`.
- Manual mode allows supplied `Id` plus `Type`.
- Raw request/response are returned transiently and audit metadata is logged.

This behavior is superseded by commit `9d49047c`. In particular, rc4 removes automatic-mode `channel_id`, adds exact `cgt-...` lookup, and separates task ownership from Asset Admin credential selection.

## 8. Database Compatibility Review Addendum

Review date: 2026-06-18

Current deployment baseline:

- LSF production and preflight use MySQL.
- Generic new-api code remains compatible with MySQL, PostgreSQL, and SQLite.
- No production, preflight, Docker, or external database was accessed during this review.

Commit `12f0857f` database findings:

- Moderation Diagnose adds no table, field, migration, or schema dependency.
- It uses existing GORM task/channel queries and the existing `RecordLogWithAdminInfo` audit log path.
- It adds no raw SQL and no SQLite-, MySQL-, or PostgreSQL-specific query syntax.
- Sidebar configuration remains JSON text inside the existing `users.setting` TEXT column.
- Audit metadata remains JSON text inside the existing log `other` field.
- Raw moderation request/response bodies, channel credentials, and ProjectName are not written to the audit log.
- Existing tests used in-memory SQLite and did not provide a real MySQL integration test.

Two correctness issues were found:

- `video_task` requests could override the task row's persisted `channel_id`, allowing a task identifier to be queried through a different channel configuration. Local follow-up commit `12ddf656` makes the stored task channel authoritative and adds focused tests for channel mismatch rejection and audit-log sensitive-field exclusion.
- Numeric record lookup ignored non-`record not found` errors from its first GORM query before falling back to `task_id`. Local follow-up commit `2550bbc0` now returns the database error immediately and adds a regression test.

Database conclusion:

- No new schema or migration is required.
- The feature's GORM/log persistence paths are compatible with the current MySQL baseline based on code inspection and local tests.
- Real MySQL execution remains a preflight verification requirement because this repository has no existing MySQL integration-test harness for this feature.

RC4 compatibility addendum, 2026-06-19:

- Rc4 adds exact JSON extraction for `private_data.upstream_task_id` with explicit SQLite, MySQL, and PostgreSQL query branches.
- The loaded JSON structure is parsed and compared again before the task is accepted.
- The query is limited to two rows so duplicates are detected without an unbounded application-level scan.
- Rc4 still adds no table, field, index, or migration.
- Local SQLite tests and compilation passed. This statement was written before Henry confirmed the successful rc4 MySQL preflight described above; rc5 still requires its own authorized preflight before release approval.

## 9. Remaining Productization Gaps

Before building or shipping an internal diagnose feature:

- Add explicit BP request ID capture if request-ID diagnosis is required. Current storage is opportunistic and body-only.
- Add an Asset Library persistence model if asset failure diagnosis must work without manually supplied `asset_id` or `request_id`.
- Consider storing a redacted raw upstream submit error for idempotency reservations. Current reservation failures only keep `fail_reason`.
- Extract Ark signing helpers into a shared internal helper with focused tests for service `ark`, region `ap-southeast-1`, action URL construction, and `Version=2024-01-01`.
- Keep `private_data` backend-only; do not expose upstream IDs in customer responses or ordinary task logs.
- Validate rc5 in an authorized MySQL preflight before any production approval. Rc1, rc2, rc3, and rc4 are superseded or failed candidates and must not be deployed.
- No customer-facing documentation change is required for this internal admin tool correction.

## Recommendation

For automatic diagnosis:

1. Resolve a local task row by exact DB ID, public `task_...`, or persisted `cgt-...`.
2. Read `tasks.private_data.upstream_task_id`.
3. Resolve a separate Asset Admin AK/SK channel through the task Group, Asset Admin ability, and ProjectName consistency.
4. Call BP using the existing Ark signing logic with the verified action name.
5. Show only redacted moderation output and persist only non-sensitive audit metadata.

Asset Library diagnosis should remain manual until new-api persists asset records or a separate verified source of `asset_id` / `request_id` exists.
