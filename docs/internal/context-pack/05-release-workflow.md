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
- no-client_request_id regression
- `client_request_id` first request
- duplicate replay
- `GET` task polling
- stale `RESERVED` query
- explicit approval

Post-release documentation updates should not run additional paid production
smoke unless Henry explicitly approves the extra cost and scope.

For production smoke that uses a Keychain token, prefer a transport that does
not expose the bearer value in process arguments, files, shell history, or logs.
The Mini RC1 production pass used curl with Authorization supplied through
stdin config after a Python client path returned HTTP 403 with the same
Keychain value.

A production balance/auth 403 from a local smoke client is not by itself a
rollback trigger when runtime health is stable. First separate client transport,
Keychain value, and tenant configuration problems from production runtime
failures.

For successful async video tasks, do not mark final billing passed from
`tasks.quota` alone. Require settlement logs, `actual_quota`, final net quota,
or upstream usage token evidence.
