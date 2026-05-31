# Post-release Documentation Sync Template

Use this template after a successful production release. Keep the patch docs-only.

## Release Identity

- Release name:
- Release date:
- Branch:
- Runtime commit:
- Docs/context commit:
- Image tag:

## Environment Summary

Preflight environment:

- Container:
- Image:
- Port:
- DB type:
- DB name, if safe:

Production environment:

- Container:
- Image:
- Server architecture:
- DB type:
- DB name, if safe:

## Migration Status

- Migration/schema change summary:
- Migration applied to preflight:
- Migration verified in preflight:
- Migration applied to production:
- Migration verified in production:
- Notes:

## Smoke Test Summary

- Local tests:
- Preflight smoke tests:
- Production smoke tests:
- Balance endpoint:
- Video task create:
- Video task polling:
- Idempotency or replay checks:
- Stale task checks:

## Paid Production Validation

- Required for this release:
- Completed:
- Summary:
- Customer-visible output checked:
- Sensitive output avoided:

## Rollback Assets

Summarize rollback assets without secrets. Internal-only backup identifiers may be recorded when needed.

- Previous image/tag:
- Previous container snapshot:
- DB backup identifier, if safe:
- Config/env snapshot identifier, if safe:
- Operator notes:

## Known Non-blockers / Log Noise

- Item:
- Impact:
- Why non-blocking:
- Follow-up owner:

## Customer-facing Impact

- Customer behavior changed:
- Customer docs updated:
- Current customer guide:
- If no customer-facing behavior changed, state that the current customer guide remains valid.
- Leakage review completed:

## Internal-only Notes

- Operational notes:
- Release caveats:
- Monitoring notes:
- Items not suitable for customer docs:

## Follow-up Tasks

- Task:
- Owner:
- Priority:
- Due date:

## Final Checklist

- [ ] Context pack updated from verified facts.
- [ ] Latest release record updated.
- [ ] Internal runbooks updated, if applicable.
- [ ] Customer docs updated only if customer behavior or guidance changed.
- [ ] Customer docs checked for internal information leakage.
- [ ] No source code changed.
- [ ] No tests changed.
- [ ] No migrations changed.
- [ ] No Docker, deployment, production config, or CI files changed.
- [ ] No secrets, credentials, DB dumps, raw logs, signed video URLs, or real customer data added.
- [ ] `git diff --check` passed.
- [ ] Secret/leakage scan completed.
