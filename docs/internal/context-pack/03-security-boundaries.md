# Security Boundaries

Never expose API keys, customer bearer keys, AK/SK, SQL_DSN, DB dumps, env files, production logs, or real customer data.

Do not expose ProjectName, internal channel names, group names, upstream BytePlus model names, `token_id`, `private_data`, or DB schema to customers.

Do not select or export `tasks.data` or `tasks.private_data` in routine runbook queries.

No production modification is allowed without explicit approval.

ProjectName must remain server-side.

Customer docs must stay customer-safe.

Seed Audio customer docs may expose only tenant-facing model aliases, customer
endpoint paths, public HTTPS reference URL requirements, and high-level billing
behavior. They must not expose ProjectName, upstream model names, internal
channel/group/project configuration, quota formulas, group ratios, test
balances, commit/image evidence, raw prompts, raw reference URLs, temporary
output URLs, or production smoke details.

Seed Audio error usage diagnostics are admin-only operational data. Admin usage
logs may show redacted request/error diagnostics for upstream-dispatched
failures, but `/api/log/self`, `/api/log/token`, and other user-facing log
paths must be backend-sanitized. User-facing logs must not expose
`client_request_id`, `upstream_request_id`, `error_code`, `http_status`,
`retryable`, or `other.seed_audio_error` for Seed Audio error diagnostics.
