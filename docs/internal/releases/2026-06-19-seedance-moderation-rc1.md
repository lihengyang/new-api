# Seedance Moderation Diagnose Production Candidate RC1

Date: 2026-06-19
Status: local production candidate; not deployed
Candidate image: `new-api:seedance-moderation-rc1`
Release branch: `release/seedance-moderation-v1`
Candidate runtime code commit: `ddc7647919c39948754084c829581f236c007f73`

## Production Baseline

- Production image record: `new-api:seedance-p1-client-request-rc2`
- Runtime code commit: `d0bc8c18f1198437f91417de3293b5b03256118a`
- Source branch: `feature/seedance-p1-billing-client-request-v1`
- Source tag: none for the runtime commit
- Current production remains unchanged.

The baseline was confirmed from the current release record, the context pack, and
Git ancestry before the release branch was created. No server was connected and
no production or preflight state was queried or modified during candidate
assembly.

## Moderation-only Scope

This candidate contains:

- the admin-only Moderation Diagnose backend and
  `POST /api/admin/moderation/diagnose`;
- the safe Asset Admin credential candidate endpoint;
- QPM 10 rate limiting scoped to Moderation Diagnose;
- exact numeric task record, LSF `task_...`, and persisted BP `cgt-...` lookup;
- separate target and credential resolvers;
- deterministic tenant-aware Asset Admin credential resolution;
- MySQL, PostgreSQL, and SQLite upstream-task lookup support;
- manual task automatic ownership resolution;
- the `library_asset` to manual-mode boundary and UI flow;
- `found`, `not_found`, `request_failed`, `validation_error`, and
  `rate_limited` result semantics;
- stale-result clearing and collapsed Advanced diagnostics;
- redacted responses and restricted audit metadata;
- the admin-only page, route, menu, and related backend/frontend tests.

## Explicitly Excluded

This candidate does not contain:

- Seedance Mini model recognition or behavior;
- Mini no-video or with-video pricing;
- a Mini 1080p guard;
- Mini billing tests, configuration, or documentation;
- changes to `relay/channel/task/doubao/seedance_billing.go`;
- changes to `relay/channel/task/doubao/seedance_billing_test.go`.

Production-baseline normal and fast Seedance billing remains unchanged.

## Porting Audit

The original mixed implementation commit was not mechanically cherry-picked.
Its file and hunk boundaries were reviewed first:

- `12f0857f`: only the Moderation controller, tests, admin wiring, route, menu,
  and page hunks were applied; both Mini billing files were excluded.
- `12ddf656` and `2550bbc0`: pure Moderation controller/test fixes.
- `1135c50d` and `342587b0`: only Moderation controller/test hunks were applied;
  historical finding-document hunks were excluded.
- `9d49047c` and `302f60be`: pure Moderation rc4/rc5 fixes.

The resulting Moderation files match the final rc5 feature state while the
Seedance billing implementation and tests match the production baseline.

## RC4 Preflight Evidence

Henry's explicit rc4 preflight evidence recorded on 2026-06-19 confirmed:

- MySQL startup and migrations succeeded;
- unauthenticated Moderation Diagnose requests returned HTTP 401;
- numeric task record IDs, LSF `task_...`, and BP `cgt-...` resolved exactly;
- Group, Asset Admin ability, ProjectName, and strict AK/SK credential mapping
  succeeded;
- GetModerationResult signing used the correct action, version, service, and
  fixed internal region;
- a positive result returned real Copyright and Safety block reasons;
- `NotFound.Id` was returned and displayed;
- QPM 10 was exercised and request 11 returned HTTP 429;
- `library_asset` without ownership mapping did not call BytePlus.

These facts are carried forward as prior authorized preflight evidence. This
candidate assembly did not reconnect to that environment.

## RC5 Corrections Included

The final rc5 corrections included in this candidate:

- allow manual `task_id` requests without a selected credential channel when
  the local task ownership can be resolved;
- ignore a supplied manual channel for an existing local task;
- require and revalidate a credential channel for external task, asset, and
  request IDs;
- make the unavailable `library_asset` ownership mapping an explicit UI
  boundary with a manual-mode transition and no upstream call;
- separate completed `NotFound.Id` requests from found results;
- clear stale results whenever inputs or request state change;
- keep attempted queries and raw diagnostics behind a collapsed Advanced
  diagnostics section;
- return `rate_limited` without entering the handler or affecting unrelated
  APIs.

No asset table, field, index, migration, or schema change was added.

## Local Verification

Passed:

- `go test ./controller ./model ./middleware ./relay/channel/task/doubao`
- `go test ./controller -run 'Moderation|Diagnose'`
- `go test ./middleware -run 'Moderation|Admin|Rate'`
- `go test ./relay/channel/task/doubao -run 'Seedance|Billing'`
- Moderation frontend state tests in the clean Dockerfile Bun builder
- clean Dockerfile Bun dependency install and Vite production build
- targeted Prettier checks for the Moderation page and all related frontend
  files except the shared icon helper
- `git diff --check`

The shared `web/src/helpers/render.jsx` full-file Prettier check still reports
one pre-existing production-baseline formatting difference in
`renderTaskBillingProcess`. The Moderation icon hunk is formatted, and the old
billing-render formatting was intentionally left unchanged to keep this
candidate Moderation-only.

## Release Gates Still Required

- Build and inspect the final `linux/amd64` image.
- Deploy only to the isolated MySQL preflight environment after authorization.
- Re-run authentication, exact lookup, credential mapping, found/not-found,
  QPM, redaction, and no-upstream-call checks against the candidate.
- Confirm preflight is not connected to the production MySQL database.
- Obtain explicit approval before any production action.

No customer-facing documentation was changed. No production deployment is
authorized by this record.
