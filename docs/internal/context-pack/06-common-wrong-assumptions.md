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

Wrong: ProjectName can be accepted from customer request body.

Correct: ProjectName is server-side only and must not be customer-controlled.

Wrong: production, preflight, image, or DB facts can be answered from memory.

Correct: verify with read-only commands and current docs.

Wrong: local Docker build architecture is automatically suitable for production.

Correct: production is `linux/amd64`; Apple Silicon builds must explicitly target `linux/amd64`, and image architecture must be inspected before deployment.

Wrong: shell pipeline `curl | tee | python3 - << heredoc` is a safe JSON parsing pattern.

Correct: heredoc consumes stdin; save the response to a file with `curl -o`, then parse the file.

Wrong: customers should poll video tasks every few seconds.

Correct: customer docs should recommend 30 seconds or longer for Seedance 2.0 polling, and 45-60 seconds for longer videos or high-load periods.

If a fact conflicts with chat memory, prefer the repo context pack and latest read-only verification.

Update this file whenever a repeated AI/human mistake is discovered.

## Customer Guide Source of Truth

Wrong: The latest customer API guide can be reconstructed from memory or old drafts.

Correct: The customer-facing guide for the P1 production rollout is:

`docs/customer/Light_Speed_Future_API_Integration_Guide_v2.1.2.docx`

It includes:

- company website: `https://lightspeedfuture.com`
- API base URL: `https://ai-api.lightspeedfuture.com`
- `POST /v1/videos`
- `GET /v1/videos/{task_id}`
- `GET /v1/billing/balance`
- optional `metadata.client_request_id`
- 30s+ polling guidance
- Common Video Generation Examples
- Asset Library overview and paths
- ProjectName prohibition
- no model field for Asset Library
- LSF contact: Telegram `@HenryBroG`, email `info@lightspeedfuture.com`

## Production Release Source of Truth

Wrong: Production runtime state can be inferred from memory.

Correct: Production state must be confirmed from repo release records and read-only server checks. The 2026-05-31 P1 rollout record is:

`docs/internal/releases/2026-05-31-seedance-p1-client-request-rc2.md`
