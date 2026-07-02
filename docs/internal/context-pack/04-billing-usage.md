# Billing and Usage

`GET /v1/billing/balance` returns token-level balance information for the authenticated API key.

Video task usage may appear in `GET /v1/videos/{task_id}` for completed tasks.

P1 `client_request_id` duplicate replay must not pre-deduct again and must not call upstream again.

Billing internals such as pre-deduct, refund, top-up, `modelRatio`, `groupRatio`, and `otherMultiplier` are internal and must not be exposed in customer docs.

Seed Audio customer docs should say only that billing is based on successfully
generated audio duration, failed validation requests are not charged, and
customers can check balance with `GET /v1/billing/balance`. Do not expose Seed
Audio quota formulas, group ratios, per-second quota constants, test balances,
or internal settlement fields in customer docs.

For commercial billing conclusions, verify current logs and DB behavior rather than relying on upstream new-api assumptions.

## Billing Release Invariants

`tasks.quota` is reservation/precharge evidence only. It is not final billing
evidence for a terminal SUCCESS async video task.

Terminal SUCCESS billing must be validated from settlement evidence such as
`actual_quota`, settlement logs, final net quota, and upstream usage token
fields. A settlement-exists check is not enough; the final amount must match
the exact quota formula:

```text
actual_quota = floor(tokens * ModelRatio * GroupRatio * OtherRatio)
```

Every new Seedance billing tier must also prove the effective price invariant:

```text
ModelRatio * OtherRatio = official tier price / repo unit price
```

For Seedance Standard, Fast, and Mini, keep the family-base semantics explicit
unless a release record explicitly approves and tests another convention:

```text
ModelRatio = family no-video base / repo unit
OtherRatio = current tier / family no-video base
```

Mini Scheme B final semantics:

```text
Mini no-video    = 1.75 * 1.0 = 1.75
Mini video-input = 1.75 * 0.6 = 1.05
```
