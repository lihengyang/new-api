# Release Records

Internal release records are the repo source of truth for completed production rollouts.

## Naming

Use this format:

```text
YYYY-MM-DD-short-release-name.md
```

Example:

```text
2026-05-31-seedance-p1-client-request-rc2.md
```

## Structure

Each release record should include:

- release name and date
- runtime code commit
- docs/context commit, when applicable
- image tag
- preflight and production environment summary
- DB type and DB name only when safe to record
- migration/schema status
- smoke test summary
- production validation summary
- rollback assets summary without secrets or sensitive paths
- known non-blockers or log noise
- customer-facing impact
- internal-only notes
- follow-up tasks

Do not include secrets, API credentials, environment files, DB dumps, raw production logs, signed video URLs, or real customer data.

After a successful production release, run Patch R using `docs/internal/templates/post-release-docs-sync-template.md`.
