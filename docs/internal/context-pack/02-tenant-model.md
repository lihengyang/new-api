# Tenant Model

One tenant normally maps to one tenant package, project isolation setup, group, and token.

Customers receive:

- Base URL
- Bearer API key
- tenant video model aliases
- allowed endpoint paths
- customer-facing docs

Customers must use the model aliases provided by Light Speed Future.

Customers must not use upstream model names directly.

Asset Library requests do not require a model field.

ProjectName is managed server-side and must not be provided by customers.

Customer-facing docs should use placeholders such as `<LSF_API_KEY>` and `<TENANT_MODEL_ALIAS>`.
