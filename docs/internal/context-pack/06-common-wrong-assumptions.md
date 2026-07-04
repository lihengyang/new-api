# Common Wrong Assumptions

Wrong: production is still SQLite.

Correct: current P1 production/preflight are MySQL; SQLite is legacy/cold-backup/rollback reference only unless re-verified.

Wrong: SQLite preflight is enough for MySQL production.

Correct: MySQL production requires MySQL preflight validation.

Wrong: generic upstream new-api behavior can be assumed.

Correct: this is a customized new-api nightly-based Seedance platform; verify repo code, logs, DB, and release history.

Wrong: customer docs can mention reservation, `token_id`, or DB schema.

Correct: customer docs must not expose internal implementation.

Wrong: `client_request_id` compares request body in P1.

Correct: P1 returns the original task for the same API key + `client_request_id` and does not compare request body.

Wrong: duplicate replay can bill or call upstream again.

Correct: duplicate replay must not pre-deduct and must not call upstream again.

Wrong: it is safe to wrap BytePlus HTTP calls in DB transactions.

Correct: never put long upstream network I/O inside DB transactions.

Wrong: provider project name can be accepted from customer request body.

Correct: provider project identity is server-side only and must not be customer-controlled.

Wrong: production, preflight, image, or DB facts can be answered from memory.

Correct: verify with read-only commands and current docs.

Wrong: `relay/channel/task/doubao` means the current LSF Seedance upstream model ID is a Doubao model.

Correct: `doubao` is inherited new-api adapter/package naming. Seedance routing is tenant alias -> new-api model redirection -> BytePlus ModelArk endpoint `ep-*` -> Dreamina Seedance model family. Do not infer real upstream model IDs from package paths.

Wrong: `dreamina-seedance-*` must be the direct new-api channel mapping gate.

Correct: for the LSF Seedance preflight route, the direct new-api mapping gate is the tenant alias to endpoint route. `dreamina-seedance-*` describes the endpoint-backed model family, not the direct channel mapping target.

Wrong: preflight task failures can depend on a human copying UI details later.

Correct: preflight scripts must automatically collect sanitized `fail_reason` evidence for failed tasks. Sanitized output may include code, type, parameter, and message; it must not include provider task IDs, raw request bodies, provider URLs, credentials, or customer data.

Wrong: remote runbooks can safely leave `<...>` placeholders or hide multiple checks inside nested SSH and SQL quote chains.

Correct: remote runbooks should be executable as written for the approved environment, avoid unresolved placeholders, avoid fragile nested quoting gates, and report per-gate exit codes with sanitized stderr summaries.

Wrong: a Fast reject must always use a nested OpenAI-style error object to pass preflight.

Correct: the task API may return `invalid_request_error` as a top-level task error code. A Fast reject can pass with either nested or top-level error shape only when HTTP 400, no-task, and no-billing checks also pass.

Wrong: `tasks.quota` is the final billing amount for a successful async video task.

Correct: `tasks.quota` is precharge or reservation evidence. Final billing for
terminal successful async video tasks must be reconciled from settlement logs,
`actual_quota`, and final net quota, then checked against the expected effective
upstream price.

Wrong: a final settlement row proves billing is correct.

Correct: terminal SUCCESS billing must prove the exact quota formula:
`actual_quota = floor(tokens * ModelRatio * GroupRatio * OtherRatio)`.

Wrong: customer sample count equals the affected incident population.

Correct: blast radius must be derived from the audited production task window
and must exclude post-fix smoke tasks unless the smoke itself is affected.

Wrong: a customer sample can be compensated directly from the sample token
reference.

Correct: compensation targets must be reconciled to the live billing token,
user, and group. Placeholder token hashes are audit breadcrumbs, not credit
targets.

Wrong: preflight settlement existence is enough for a billing release gate.

Correct: settlement evidence must match the exact expected final quota and
effective ratio.

Wrong: Mini can use a different `ModelRatio` / `OtherRatio` semantic from
Standard and Fast without an explicit gate.

Correct: Seedance families should share the family-base billing convention
unless a release record explicitly approves and tests a different convention.

Wrong: Codex App inherits Terminal-exported smoke-token environment variables.

Correct: Codex App should create/read its own macOS Keychain item and verify
`/v1/billing/balance` before paid smoke.

Wrong: pasting a smoke key into chat, using a hidden PTY prompt, writing a
temporary secret file, or guessing a token from the DB is an acceptable way to
unblock Codex App smoke.

Correct: Codex App smoke uses the macOS Keychain item with account
`henrytest` and service `lsf-henrytest-api-key`. Entry or replacement uses a
macOS hidden dialog only, and only presence/status plus balance response shape
may be printed.

Wrong: copied container labels are authoritative over the running image label.

Correct: after a Docker container is recreated from an image, inspect the
running image label for revision evidence. Container labels may retain an older
copied revision when the deployment path preserves container configuration.

Wrong: a local production smoke HTTP 403 always means the production API or
tenant token is broken.

Correct: first isolate Keychain value, client transport, and tenant
configuration. During Mini RC1, Henry verified the same key through the public
endpoint while one Codex Python client path still saw HTTP 403; curl with
Authorization supplied through stdin config passed the balance gate.

Wrong: a `/v1/billing/balance` gate failure automatically proves production
runtime failure.

Correct: balance gate failure is first a key, client, or tenant configuration
blocker unless independent runtime evidence proves otherwise. Stop before paid
smoke when the balance gate fails.

Wrong: passing Mini no-video production smoke authorizes additional paid
production video tasks.

Correct: each extra paid production smoke case, including Mini
`reference_video` and live Standard 4K reruns, needs separate Henry approval.

Wrong: a successful release means the rollback artifact can be cleaned up
immediately.

Correct: preserve the recorded rollback artifact through the observation window
unless Henry explicitly approves retiring it.

Wrong: internal release evidence means customer Mini docs are ready to publish.

Correct: keep customer docs unpublished until Henry separately approves
publication, and keep them free of internal routing, credentials, ProjectName,
channel/group identifiers, DB fields, and billing internals.

Wrong: LibreOffice render output is the final authority for customer DOCX/PDF
visual quality.

Correct: LibreOffice is only a quick automated preview and structure smoke.
Final customer DOCX/PDF visual QA should use Microsoft Word / Office and Henry
manual review.

Wrong: a customer delivery file can keep `_FIXED`, `_DRAFT`, `_REJECTED`, or a
wrong version filename once the content is correct.

Correct: final customer delivery filenames must use the approved version and
title. For Seed Audio P0, the final guide is
`Light_Speed_Future_API_Integration_Guide_v2.2.0_Seed_Audio_Production_Edition.docx`.

Wrong: docs-only customer-guide commits should update production candidate
image tags or source traceability.

Correct: docs-only commits do not change the frozen production candidate image
tag, deployed artifact identity, or release-source traceability.

Wrong: v2.0.0 is a valid Seed Audio customer-guide version because a temporary
filename once said so.

Correct: the final Seed Audio Production Edition customer-guide version is
v2.2.0. Treat v2.0.0 as a filename mistake, not a release fact.

Wrong: Seed Audio still has a local `2048` character text limit or validates
only `input`.

Correct: the current official Seed Audio `text_prompt` limit is `3000`
characters, and local validation counts the final combined trimmed
`instructions` + newline + trimmed `input` payload.

Wrong: a failed Seed Audio upstream attempt should be retried with the same
`metadata.client_request_id`.

Correct: failed idempotency records remain tied to the original
`client_request_id`; a new upstream attempt needs a fresh client request ID.

If a fact conflicts with chat memory, prefer the repo context pack and latest read-only verification.

Update this file whenever a repeated AI/human mistake is discovered.
