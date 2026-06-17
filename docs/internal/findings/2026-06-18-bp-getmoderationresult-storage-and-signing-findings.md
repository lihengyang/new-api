# BP GetModerationResult Storage and Signing Findings

Date: 2026-06-18

Scope: exploration only. No production, preflight, Docker, DB, or runtime state was modified or queried. This review used the local repo plus the Desktop handoff brief. The external knowledge card named by the handoff, `LSF_AI_Knowledge_Card_BP_GetModerationResult_API_2026-06-17.md`, was not found under Desktop, Documents, or Dev, so the BP API facts below rely on the handoff text and local code inspection.

## Executive Summary

- Platform video task IDs are generated locally as `task_...` and stored in `tasks.task_id`.
- BP/Seedance upstream generation IDs such as `cgt-...` are stored separately in `tasks.private_data.upstream_task_id` after a successful upstream submit.
- For video tasks that reached upstream submit successfully, later failed task polling should preserve enough data to query BP moderation by `Type=task_id` using the stored `cgt-...`.
- Upstream request IDs are not explicitly captured from headers or response metadata. They may only survive opportunistically if the upstream JSON body itself contains a request ID and is stored in `tasks.data`.
- Asset Library calls are currently pass-through relay calls. There is no local material asset table/model and no durable local store for `asset_id`, `request_id`, raw asset responses, or failed asset registration records.
- Existing no-SDK Ark signing for Asset Library can be reused for BP moderation queries, but it is currently controller-private and should be extracted or wrapped before a productized tool.
- The current working tree already contains uncommitted backend/frontend `moderation_diagnose` code. Treat it as pre-existing branch state, not a completed approved product. It also uses an action name that does not match the handoff.

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
- `controller/moderation_diagnose.go:268` in the current working tree tries to infer request IDs from `tasks.data` keys such as `request_id`, `requestId`, and `RequestId`, but that only works if the upstream JSON body contains such a field.
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

The current working-tree diagnose resolver reflects this limitation:

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
- The current uncommitted `controller/moderation_diagnose.go` already calls `buildSeedanceAssetTargetURL()` and `signSeedanceAssetAdminRequest()` (`controller/moderation_diagnose.go:364` and `controller/moderation_diagnose.go:380`).

Action-name gap:

- The handoff states BP action `GetModerationResult`.
- Current working-tree code uses `seedanceModerationDiagnoseActionName = "GetAIGCModerationResult"` (`controller/moderation_diagnose.go:29`).
- This must be verified against the missing knowledge card or official BP docs before any implementation is productized.

## 7. Existing Diagnose Worktree State

The current working tree already includes uncommitted/modified diagnose surface area:

- Backend controller: `controller/moderation_diagnose.go`
- Backend tests: `controller/moderation_diagnose_test.go`
- Admin route: `router/api-router.go:209`
- Frontend page: `web/src/pages/ModerationDiagnose/index.jsx`
- Sidebar/render wiring in `web/src/App.jsx`, `web/src/components/layout/SiderBar.jsx`, and related files.

Observed behavior from code inspection:

- Video task mode resolves by local task row ID or public `task_...`.
- It refuses to use public `task_...` as a BP generation ID unless the extracted ID has `cgt-` prefix.
- It falls back to `private_data.upstream_task_id`.
- Library asset mode does not look up a local asset record because none exists; it requires supplied `asset_id` and `channel_id`.
- Manual mode allows supplied `Id` plus `Type`.
- Raw request/response are returned transiently and audit metadata is logged.

This is not a recommendation to ship it as-is. It is pre-existing current repo state and still conflicts with the exploration-only boundary until Henry approves a product spec.

## 8. Productization Gaps

Before building or shipping an internal diagnose feature:

- Verify the exact BP action name. Handoff says `GetModerationResult`; current working-tree code says `GetAIGCModerationResult`.
- Decide whether the tool is video-only for v1. Video task failures after upstream submit are already mostly diagnosable by stored `cgt-...`.
- Decide whether manual `Id` + `Type` queries are allowed. They are powerful but bypass local record resolution.
- Add explicit BP request ID capture if request-ID diagnosis is required. Current storage is opportunistic and body-only.
- Add an Asset Library persistence model if asset failure diagnosis must work without manually supplied `asset_id` or `request_id`.
- Consider storing a redacted raw upstream submit error for idempotency reservations. Current reservation failures only keep `fail_reason`.
- Extract Ark signing helpers into a shared internal helper with focused tests for service `ark`, region `ap-southeast-1`, action URL construction, and `Version=2024-01-01`.
- Keep `private_data` backend-only; do not expose upstream IDs in customer responses or ordinary task logs.

## Recommendation

For the first productized tool, start with video task failure diagnosis only:

1. Resolve local task row by DB ID or public `task_...`.
2. Read `tasks.private_data.upstream_task_id`.
3. Require the upstream ID to match expected BP generation ID shape before querying.
4. Call BP using the existing Ark signing logic with the verified action name.
5. Show/store only redacted moderation output according to an explicit internal policy.

Asset Library diagnosis should remain manual until new-api persists asset records or a separate verified source of `asset_id` / `request_id` exists.
