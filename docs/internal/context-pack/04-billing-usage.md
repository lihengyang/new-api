# Billing and Usage

`GET /v1/billing/balance` returns token-level balance information for the authenticated API key.

Video task usage may appear in `GET /v1/videos/{task_id}` for completed tasks.

P1 `client_request_id` duplicate replay must not pre-deduct again and must not call upstream again.

Billing internals such as pre-deduct, refund, top-up, `modelRatio`, `groupRatio`, and `otherMultiplier` are internal and must not be exposed in customer docs.

For commercial billing conclusions, verify current logs and DB behavior rather than relying on upstream new-api assumptions.
