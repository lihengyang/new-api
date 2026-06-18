# Release Workflow

Default workflow:

1. local branch
2. local tests
3. Docker build
4. verify built image architecture is `linux/amd64`
5. preflight deployment
6. MySQL schema verification
7. smoke tests
8. production only after explicit approval
9. post-release documentation sync after production validation

## Knowledge-first Workflow

Before LSF/new-api work, read `AGENTS.md`, the complete context pack, `docs/internal/releases/README.md`, the current release record and runbook, and relevant findings. Do not treat temporary chat memory as a source of truth.

When current code, authorized read-only environment evidence, or Henry's latest explicit confirmation conflicts with the knowledge base:

1. report the old information
2. report the latest fact
3. cite the code/environment/user-confirmation evidence
4. identify the files that should be updated

Current evidence and Henry's latest explicit confirmation take precedence over old historical documents. Preserve release history, but mark superseded operational guidance clearly.

Current P1 rollout requires MySQL preflight validation.

SQLite-only validation is not sufficient for the current MySQL production rollout.

Preflight must not point to the live production DB.

Keep the rollback image/tag available.

Do not rollback DB blindly.

Release gate should include:

- local tests
- build
- built image architecture inspected as `linux/amd64` before transfer/load
- preflight startup
- schema verification
- billing balance test
- no-client_request_id regression
- `client_request_id` first request
- duplicate replay
- `GET` task polling
- stale `RESERVED` query
- explicit approval

## Post-release Documentation Sync

After production validation succeeds, run a docs-only Patch R follow-up. Do not rely on chat memory alone; read the context pack and latest release record first.

Documents to review and update:

- `AGENTS.md`
- `docs/internal/context-pack/*.md`
- latest `docs/internal/releases/*.md`
- relevant internal runbooks under `docs/internal/*runbook*.md`
- customer-facing docs under `docs/customer/*` only when customer behavior or guidance changed
- reusable templates under `docs/internal/templates/*` when a new release process pattern is established

Patch R review should be performed by the release owner or operator who validated production. If customer-facing documentation is touched, review it specifically for leakage of internal routing, DB, billing, channel, group, credential, or upstream implementation details.

If no customer-facing behavior changed, do not create a new customer guide. Record that the current customer guide remains valid.

Patch R must not modify source code, tests, migrations, Docker files, deployment scripts, production config, CI config, runtime state, secrets, DB dumps, logs, or environment files.
