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

If a fact conflicts with chat memory, prefer the repo context pack and latest read-only verification.

Update this file whenever a repeated AI/human mistake is discovered.
