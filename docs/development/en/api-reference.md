# API documentation and handler source

| Task | Guide |
| --- | --- |
| List public models | [Models](../../api/en/models.md) |
| Conversation | [Chat](../../api/en/chat.md), [Responses](../../api/en/responses.md) |
| Native protocol | [Bypass paths](../../api/en/bypass.md) |
| Video tasks | [Ark](../../api/en/ark.md), [FAL](../../api/en/fal.md) |
| Diagnose errors | [Status codes and logs](../../api/en/errors.md) |

~~~bash
curl "$XHUB_BASE_URL/v1/models" \
  -H "Authorization: Bearer $XHUB_API_KEY"
~~~

Use an XHub virtual key, not the upstream provider key. XHUB_BASE_URL is the gateway root. Returned model IDs are public names.

## Change an endpoint

Find its registration in internal/gateway/routes.go or the module mount.go, then follow the handler to authorization, storage and execution. Update request/response tests and frontend consumers together.

~~~bash
rg 'Register|Handle|GET|POST' internal/gateway/models/mount.go
go test ./internal/gateway/models -count=1
~~~

[Handler source index](handler-index.md) · [Testing](testing.md)
