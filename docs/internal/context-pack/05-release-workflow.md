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
