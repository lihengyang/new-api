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

Wrong: a local production smoke HTTP 403 always means the production API or
tenant token is broken.

Correct: first isolate Keychain value, client transport, and tenant
configuration. During Mini RC1, Henry verified the same key through the public
endpoint while one Codex Python client path still saw HTTP 403; curl with
Authorization supplied through stdin config passed the balance gate.

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

If a fact conflicts with chat memory, prefer the repo context pack and latest read-only verification.

Update this file whenever a repeated AI/human mistake is discovered.
