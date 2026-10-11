# Trace a failed request

Keep the timestamp, public model, HTTP status and x-litellm-call-id. Find the matching request log before changing configuration.

## Example

~~~bash
curl -i "$XHUB_BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"company-chat","messages":[{"role":"user","content":"Reply with ready."}]}'
~~~

## Contract

This diagnostic makes a model call and may incur provider charges.

| Symptom | Check |
| --- | --- |
| 401 | XHub key validity; do not send the upstream key |
| 403 / concealed 404 | Current membership, ownership and model scope |
| 429 | Budget, RPM/TPM and provider response |
| No deployment | Enabled status, transport and endpoint capability |
| Stream interrupted | Events and final log; HTTP status may already be sent |
| Missing cost | Actual usage, rate snapshot, PostgreSQL and Redis backlog |

Remove credentials, personal information and signed media URLs before sharing evidence. Do not delete production data to repair a fixture.

## Source and verification

[internal/gateway/spend.go](../../../internal/gateway/spend.go)

~~~bash
go test ./internal/gateway/usage -count=1
~~~

[Test prerequisites](testing.md) · [Next: runtime](runtime.md)
