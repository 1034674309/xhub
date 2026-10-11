# Rerank

## Behavior
Map results by original index. Response fields depend on registered rerank protocol.

## Endpoint and authentication

~~~http
POST /v1/rerank
~~~

Use an XHub API key: Authorization: Bearer <XHUB_API_KEY>. Do not use upstream provider credentials as gateway keys.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/v1/rerank" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","query":"What is XHub?","documents":["XHub is a model gateway."],"top_n":1}'
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| model | string · required | Public model name; deployment must support this protocol. |
| query / documents | string / array · required | Search query and candidate documents. |
| top_n | integer | Number of ranked results. |

## Success response

~~~json
{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.95
    }
  ]
}
~~~

| Field | Type |
| --- | --- |
| results | object[] |
| results[].index | number |
| results[].relevance_score | number |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid request. Check JSON, parameters and query. |
| 401 | Missing or invalid XHub credential. |
| 403 | Permission denied. Check model scope or policy. |
| 429 | Limit exceeded. Inspect response details and Retry-After. |
