# Image edits

## Behavior
Let curl set multipart boundaries. Some native deployments accept JSON references instead; use the matching protocol.

## Endpoint and authentication

~~~http
POST /v1/images/edits
~~~

Use an XHub API key: Authorization: Bearer <XHUB_API_KEY>. Do not use upstream provider credentials as gateway keys.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/v1/images/edits" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -F "model=YOUR_MODEL_NAME" -F "image=@source.png" \
  -F "prompt=Add a small lighthouse."
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| model | string · required | Public model name; deployment must support this protocol. |
| image | file|array · required | Multipart source image; format and size depend on provider. |
| prompt / mask | string / file | Edit instruction and optional mask. |

## Success response

~~~json
{
  "data": [
    {
      "b64_json": "BASE64_IMAGE_DATA"
    }
  ]
}
~~~

| Field | Type |
| --- | --- |
| data | object[] |
| data[].b64_json | string |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid request. Check JSON, parameters and query. |
| 401 | Missing or invalid XHub credential. |
| 403 | Permission denied. Check model scope or policy. |
| 429 | Limit exceeded. Inspect response details and Retry-After. |
