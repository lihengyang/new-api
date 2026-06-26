# Customer Guide Archive Register

Internal use only. This register tracks customer guide archive status and must
not be copied into customer-facing documentation.

## Current Status

As of the Seedance 2.0 Mini RC1 production closeout:

- Current customer guide:
  `Light_Speed_Future_API_Integration_Guide_v2.1.4`
- Superseded customer guide:
  `Light_Speed_Future_API_Integration_Guide_v2.1.3`
- Archive status: prepared for customer guide archive tracking.

This register records guide status only. It does not publish customer
documentation and does not attach or reproduce the guide artifact.

## Customer-Safe Boundary

Customer guide archive and publication work must not expose:

- upstream model IDs;
- provider project values;
- internal channel, group, endpoint, token, or customer identifiers;
- production smoke evidence;
- rollback artifact names;
- database schema or SQL details;
- credentials, API keys, bearer values, AK/SK values, SQL DSNs, raw env, or
  raw logs;
- internal billing implementation details.

Customer-facing guide content should use only customer-safe placeholders such
as `<LSF_API_KEY>` and `<TENANT_MODEL_ALIAS>`.

## Follow-Up

Publication, distribution, or replacement of any customer-facing guide remains
a separate Henry-approved customer documentation action.
