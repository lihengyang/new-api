# Tenant Model

One tenant normally maps to one tenant package, project isolation setup, group, and token.

Customers receive:

- Base URL
- Bearer API key
- tenant video model aliases
- tenant Seed Audio model aliases when enabled
- allowed endpoint paths
- customer-facing docs

Customers must use the model aliases provided by Light Speed Future.

Customers must not use upstream model names directly.

Seed Audio P0 customer-facing modes are `text_only`, `audio_url`, `image_url`,
and `metadata.client_request_id` idempotent replay on `POST /v1/audio/speech`.
Reference URLs must be public HTTPS URLs and provider-accessible; a URL that
works in a browser may still fail provider-side download or processing.

Seed Audio P0 customer requests must not include `voice`, `audio_data`,
`image_data`, base64 input, customer project routing fields, customer-supplied
upstream project fields, or direct upstream model names.

Asset Library requests do not require a model field.

ProjectName is managed server-side and must not be provided by customers.

Customer-facing docs should use placeholders such as `<LSF_API_KEY>` and `<TENANT_MODEL_ALIAS>`.
