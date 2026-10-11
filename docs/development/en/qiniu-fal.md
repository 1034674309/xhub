# Qiniu FAL video integration

Save a model provider connection first. Add a deployment with the actual upstream model, matching FAL protocol, public name and rate. Enter the address and key only in the provider form.

![Provider connection](../../assets/provider-guide/provider-form-en.jpg)

![Model deployment](../../assets/provider-guide/deployment-form-en.jpg)

## Create and query

This creates a paid task. kling-25 must be a saved, permitted deployment.

~~~bash
curl "$XHUB_BASE_URL/queue/fal-ai/kling-video/v2.5-turbo/pro/text-to-video" \
  -H "Authorization: Bearer $XHUB_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"kling-25","prompt":"A red apple on a white table.","duration":"5"}'
~~~

Use the same XHub identity to query rewritten status_url / response_url with request_id. Query paths belong to the shared queue, not the full creation path. model disambiguates public aliases and is stripped before upstream forwarding.

| Series | Transport | Query prefix |
| --- | --- | --- |
| Seedance 2.0 | qiniu_fal_doubao_20 | /queue/bytedance/seedance-2.0/requests/ |
| Dreamina 2.0 | qiniu_fal_dreamina_20 | /queue/byteplus/seedance-2.0/requests/ |
| Kling | qiniu_fal_kling | /queue/fal-ai/kling-video/requests/ |

## Extend it

| Change | Source |
| --- | --- |
| Model / fixed create path | [fal.go](../../../internal/provider/qiniu/fal.go): falEndpoint / registerFal |
| Queue / authentication | RegisterTransport: Actions, TaskID, Auth |
| Measured usage / pricing variants | internal/provider/measurement |
| Terminal settlement / task pinning | internal/dataplane/execution/official.go |

Seedance uses actual completion_tokens; Kling/Veo use final video.duration. Vidu Q2 tiered rates and MiniMax composite usage cannot use an arbitrary flat rate: preserve unpriced when pricing is incomplete. Waiting/failed tasks are not settled; PostgreSQL deduplicates successful repeated queries. No background polling, budget reservation or create-time price locking exists. Do not replay ambiguous creates.

~~~bash
go test ./internal/provider/... ./internal/catalog ./internal/dataplane/... -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run 'Test(FalSettlement|SeedanceSettlement)' -count=1 -v
~~~

[Protocol code](provider-catalog.md) · [Task API](../../api/en/fal-kling.md) · [Test prerequisites](testing.md)
