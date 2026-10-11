# Bypass OpenAI image generation

## Native protocol and authentication

Use an XHub public model name and virtual key with the matching Bypass deployment. XHub maps the model and injects deployment credentials, preserves native fields and enforces access, guardrails and billing. Send Authorization: Bearer $XHUB_API_KEY. Model availability depends on configured deployments.

## Overview

Image generation

### Image generation

## Behavior
Image URLs may expire. Preserve results according to upstream retention rules. Billing may use pictures or tokens depending on rates.

## Endpoint and authentication

~~~http
POST /bypass/openai/v1/images/generations
~~~

Use an XHub API key: Authorization: Bearer <XHUB_API_KEY>. Do not use upstream provider credentials as gateway keys.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/bypass/openai/v1/images/generations" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_NAME","prompt":"A small lighthouse at sunrise.","n":1}'
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| model | string · required | Public model name; deployment must support this protocol. |
| prompt | string · required | Image description. |
| n / size / quality | integer / string | Output count, size and quality; limits depend on upstream. |
| response_format | string | url or b64_json where supported. |

## Success response

~~~json
{
  "created": 1677610602,
  "data": [
    {
      "url": "https://example.com/generated.png"
    }
  ]
}
~~~

| Field | Type |
| --- | --- |
| created | number |
| data | object[] |
| data[].url | string |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid request. Check JSON, parameters and query. |
| 401 | Missing or invalid XHub credential. |
| 403 | Permission denied. Check model scope or policy. |
| 429 | Limit exceeded. Inspect response details and Retry-After. |
