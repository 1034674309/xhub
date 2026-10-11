# Moderation

## Behavior
Provider moderation is separate from XHub guardrails. Categories are model-specific.

## Endpoint and authentication

~~~http
POST /v1/moderations
~~~

Use an XHub API key: Authorization: Bearer <XHUB_API_KEY>. Do not use upstream provider credentials as gateway keys.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/v1/moderations" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","input":"Hello world."}'
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| model | string · required | Public model name; deployment must support this protocol. |
| input | string or array · required | Content to classify. |

## Success response

~~~json
{
  "results": [
    {
      "flagged": false,
      "categories": {},
      "category_scores": {}
    }
  ]
}
~~~

| Field | Type |
| --- | --- |
| results | object[] |
| results[].flagged | boolean |
| results[].categories | object |
| results[].category_scores | object |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid request. Check JSON, parameters and query. |
| 401 | Missing or invalid XHub credential. |
| 403 | Permission denied. Check model scope or policy. |
| 429 | Limit exceeded. Inspect response details and Retry-After. |
