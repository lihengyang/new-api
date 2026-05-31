# Light Speed Future API Integration Guide v2.1.2

Tenant Quick Start for Seedance 2.0 Video Generation

Base URL:

```text
https://ai-api.lightspeedfuture.com
```

All requests use Bearer API key authentication:

```http
Authorization: Bearer <LSF_API_KEY>
```

## Check Billing Balance

Use this endpoint to check the balance associated with the API key used in the request.

```http
GET /v1/billing/balance
```

Example:

```bash
curl https://ai-api.lightspeedfuture.com/v1/billing/balance \
  -H "Authorization: Bearer <LSF_API_KEY>"
```

Response:

```json
{
  "object": "billing.balance",
  "currency": "USD",
  "balance": {
    "available": 8.8131,
    "used": 35.3767,
    "unlimited": false
  },
  "updated_at": 1780077090
}
```

For an unlimited API key, `available` is `null`:

```json
{
  "object": "billing.balance",
  "currency": "USD",
  "balance": {
    "available": null,
    "used": 35.3767,
    "unlimited": true
  },
  "updated_at": 1780077090
}
```

## Create a Video Task

```http
POST /v1/videos
```

Use the video model alias provided in your onboarding package.

Example:

```bash
curl https://ai-api.lightspeedfuture.com/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "A cinematic product shot of a futuristic electric car at sunrise"
  }'
```

The response includes the task identifier. Poll the task until it reaches a terminal status:

```http
GET /v1/videos/{task_id}
```

Example:

```bash
curl https://ai-api.lightspeedfuture.com/v1/videos/task_abc123 \
  -H "Authorization: Bearer <LSF_API_KEY>"
```

Recommended workflow:

1. Create the video task with `POST /v1/videos`.
2. Store the returned `task_id`.
3. Poll `GET /v1/videos/{task_id}` every 30 seconds or longer until `status` is `completed` or `failed`.
4. For longer videos or high-load periods, use a 45-60 second polling interval.
5. Stop polling when `status` is `completed` or `failed`.
6. When completed, read the video URL from the response metadata.

## Optional client_request_id

`metadata.client_request_id` is optional for `POST /v1/videos`.

If omitted, video task creation behavior is unchanged.

If provided, `client_request_id` makes task creation idempotent for the same API key:

- The first request creates the video task normally.
- A later request with the same API key and the same `client_request_id` returns the original task.
- A duplicate replay does not create another video task.
- Different API keys may reuse the same `client_request_id` independently.
- The current version does not compare request body consistency. Reusing the same `client_request_id` with different request contents still returns the original task for that API key.

Best practice: use a stable business identifier, such as your order ID, job ID, or workflow run ID. Do not reuse the same `client_request_id` for different video creation requests under the same API key.

Example:

```bash
curl https://ai-api.lightspeedfuture.com/v1/videos \
  -H "Authorization: Bearer <LSF_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<TENANT_MODEL_ALIAS>",
    "prompt": "A cinematic product shot of a futuristic electric car at sunrise",
    "metadata": {
      "client_request_id": "order_20260530_0001"
    }
  }'
```

When the request includes a valid `client_request_id`, the task response includes it in `metadata`:

```json
{
  "id": "task_abc123",
  "task_id": "task_abc123",
  "object": "video",
  "model": "<TENANT_MODEL_ALIAS>",
  "status": "queued",
  "progress": 0,
  "created_at": 1780077090,
  "metadata": {
    "client_request_id": "order_20260530_0001"
  }
}
```

### client_request_id Format

`metadata.client_request_id` must be a string with 1-128 characters.

Allowed characters:

```text
A-Z a-z 0-9 . _ - :
```

Invalid values return HTTP 400 with an OpenAI-style error:

```json
{
  "error": {
    "message": "metadata.client_request_id must be a string with 1-128 characters using letters, numbers, '.', '_', '-', or ':'",
    "type": "invalid_request_error",
    "param": "metadata.client_request_id",
    "code": "invalid_client_request_id"
  }
}
```
