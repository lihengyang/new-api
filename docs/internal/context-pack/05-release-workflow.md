# Release Workflow

Default workflow:

1. local branch
2. local tests
3. Docker build
4. preflight deployment
5. MySQL schema verification
6. smoke tests
7. production only after explicit approval

Current P1 rollout requires MySQL preflight validation.

SQLite-only validation is not sufficient for the current MySQL production rollout.

Preflight must not point to the live production DB.

Keep the rollback image/tag available.

After a production release passes, keep the rollback artifact available through
the observation window unless Henry explicitly approves retiring it.

Do not rollback DB blindly.

Release gate should include:

- local tests
- build
- preflight startup
- schema verification
- billing balance test
- exact final billing invariant test from settlement evidence
- no-client_request_id regression
- `client_request_id` first request
- duplicate replay
- `GET` task polling
- stale `RESERVED` query
- explicit approval

Seedance billing releases must additionally complete this checklist before
customer availability:

- preflight-production alias parity;
- production-only alias coverage;
- no-video exact billing smoke;
- video-input exact billing smoke when the tier supports video input and an
  approved safe reference asset exists;
- unsupported resolution rejection before task creation, billing reservation,
  and upstream submission;
- Fast and Standard regression coverage for unaffected families;
- post-deploy blast-radius audit from the production task window;
- compensation scope that excludes smoke tasks and placeholder tokens.

Post-release documentation updates should not run additional paid production
smoke unless Henry explicitly approves the extra cost and scope.

For production smoke that uses a Keychain token, prefer a transport that does
not expose the bearer value in process arguments, files, shell history, or logs.
The Mini RC1 production pass used curl with Authorization supplied through
stdin config after a Python client path returned HTTP 403 with the same
Keychain value.

For Mac Codex App smoke, the smoke key procedure is:

- create or read the macOS Keychain item from inside Codex App;
- account: `henrytest`;
- service: `lsf-henrytest-api-key`;
- use a macOS hidden dialog for key entry when the item must be replaced;
- run `/v1/billing/balance` before any paid video smoke;
- print only presence/status and response-shape checks, never the key;
- never paste the key into chat;
- do not rely on Terminal-exported environment variables carrying into Codex
  App;
- do not use hidden PTY prompts;
- do not use temporary secret files;
- do not guess or recover API tokens from the DB.

A production balance/auth 403 from a local smoke client is not by itself a
rollback trigger when runtime health is stable. First separate client transport,
Keychain value, and tenant configuration problems from production runtime
failures.

A balance gate failure is a key, client, or tenant configuration gate blocker
until proven otherwise. Do not classify it as a runtime failure without
independent runtime evidence.

For successful async video tasks, do not mark final billing passed from
`tasks.quota` alone. Require settlement logs, `actual_quota`, final net quota,
or upstream usage token evidence.

For new billing tiers, require an exact final quota invariant:

```text
expected final quota = floor(tokens * ModelRatio * GroupRatio * OtherRatio)
```

The invariant must use the same `ModelRatio` and `OtherRatio` semantics that
production/preflight aliases use. A settlement-exists check is not enough.

Blast-radius and compensation planning must be audited separately from smoke:

- customer sample count is not population count;
- the affected population must come from the production DB audited task window;
- compensation targets must be reconciled to the live billing token, not a
  placeholder token;
- no quota credit, refund, or balance adjustment may happen before token,
  user, and group target reconciliation is complete and approved.
