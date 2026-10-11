# OpenAI video tasks

## Behavior
After creation, GET /v1/videos/{id}; when completed GET /v1/videos/{id}/content. Poll with backoff and the same caller. No universal cancel endpoint is provided.

## Endpoint and authentication

~~~http
POST /v1/videos
~~~

Use an XHub API key: Authorization: Bearer <XHUB_API_KEY>. Do not use upstream provider credentials as gateway keys.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -F "model=YOUR_MODEL_NAME" -F "prompt=A lighthouse at sunrise." -F "seconds=8"
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| model | string · required | Public model name; deployment must support this protocol. |
| prompt | string · required | Video description. |
| seconds / size / input_reference | string / file | Supported duration, resolution and optional reference. |

## Success response

~~~json
{
  "id": "video-example",
  "status": "queued"
}
~~~

| Field | Type |
| --- | --- |
| id | string |
| status | string |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid request. Check JSON, parameters and query. |
| 401 | Missing or invalid XHub credential. |
| 403 | Permission denied. Check model scope or policy. |
| 429 | Limit exceeded. Inspect response details and Retry-After. |
