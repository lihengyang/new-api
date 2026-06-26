# Customer Guide Archive Register

Internal use only. This register tracks customer guide archive status and must
not be copied into customer-facing documentation.

## Current Status

As of the Seedance 2.0 Mini RC1 production closeout:

- Current customer guide:
  `Light_Speed_Future_API_Integration_Guide_v2.1.4`
- Superseded customer guide:
  `Light_Speed_Future_API_Integration_Guide_v2.1.3`
- Archived DOCX artifact:
  `docs/customer/Light_Speed_Future_API_Integration_Guide_v2.1.4.docx`
- PDF artifact status: not archived in this commit. Local PDF generation/render
  was blocked by the current LibreOffice dependency state, and the externally
  generated PDF was not committed after customer-safety scanning.

This register records guide status and the local archive path. It does not
publish customer documentation externally.

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
