# Models and endpoints

A deployment selects a saved provider, upstream model, public model name, transport and price. These are separate contracts.

## Example

![English model deployment form](../../assets/provider-guide/deployment-form-en.jpg)

## Contract

| Field | Role |
| --- | --- |
| model_name | Client model and routing/authorization name |
| litellm_params.model | Actual upstream model ID |
| litellm_credential_name | Saved provider connection |
| model_info.transport | Required registered executor |
| endpoint_types | Standard capabilities derive from transport; Bypass/Fal explicit |

Unknown/missing transports are rejected. Prices do not define capabilities. /public/endpoints is the registry; /model/available returns the current identity's actual executable endpoints.

Model discovery tries /models and /v1/models only after 404, within one 20-second limit. Other errors remain errors; an empty successful list is not retried. Discovery does not change the inference URL.

## Source and verification

[internal/provider/deployment/validate.go](../../../internal/provider/deployment/validate.go)

~~~bash
go test ./internal/provider/... -count=1
~~~

[Test prerequisites](testing.md) · [Next: provider-catalog](provider-catalog.md)
