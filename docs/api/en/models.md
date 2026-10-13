# List models

## Protocol behavior
GET /models allows anonymous discovery of enabled public model names. Explicit credentials retain permission filtering and invalid keys return 401. GET /v1/models remains authenticated. id is callable; created is always 1677610602 and owned_by is always openai, not the actual provider. Empty data can mean no granted models.

## Endpoint and authentication

~~~http
GET /models
~~~

GET /models publicly lists enabled model names without credentials. Explicit credentials filter access; invalid credentials return 401. GET /v1/models requires authentication.

Use the gateway root Base URL shown in the playground, without appending /v1; send requests to the full endpoint path above.

## Request example

~~~shell
curl "$XHUB_BASE_URL/models"
~~~

## Request parameters

| Field | Type | Description |
| --- | --- | --- |
| scope | string | Empty or expand; other values return 400. expand broadens administrator visibility. |
| team_id | string | Authenticated session team filter; keys keep their bound scope. Ignored for anonymous discovery. |
| return_wildcard_routes | boolean | Keep configured wildcard names and expand catalog matches; inference keys may also include granted wildcard names matching visible models. Default false; ignored for anonymous discovery. |
| only_model_access_groups | boolean | When true, returns an empty list. |

## Success response

~~~json
{
  "object": "list",
  "data": [
    {
      "id": "YOUR_MODEL_NAME",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    }
  ]
}
~~~

| Field | Type |
| --- | --- |
| object | string |
| data | object[] |
| data[].id | string |
| data[].object | string |
| data[].created | number |
| data[].owned_by | string |

## Errors and troubleshooting

| HTTP | Meaning |
| --- | --- |
| 400 | Invalid scope; only empty or expand is accepted. |
| 401 | Missing or invalid XHub credential. |
