# Security Boundaries

Never expose API keys, customer bearer keys, AK/SK, SQL_DSN, DB dumps, env files, production logs, or real customer data.

Do not expose ProjectName, internal channel names, group names, upstream BytePlus model names, `token_id`, `private_data`, or DB schema to customers.

Do not select or export `tasks.data` or `tasks.private_data` in routine runbook queries.

No production modification is allowed without explicit approval.

ProjectName must remain server-side.

Customer docs must stay customer-safe.
