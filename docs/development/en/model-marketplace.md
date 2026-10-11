# Marketplace and local catalog

Marketplace discovery does not create a deployment. Use Add this model, select a provider and protocol, and save before making calls.

## Example

~~~http
GET /price/catalog
POST /price/model/listing
Content-Type: application/json

{"id":"catalog-model","delisted":true}
~~~

## Contract

This is a management request fragment; it requires an authorized management session.

| Record | Persistence |
| --- | --- |
| Full feed snapshot | price_snapshot/catalog |
| Manual override | price_model/<model ID> |
| Listing state | price_listing/<model ID> |

Default feed: https://api.modelink.ai/v1/market/models; XHUB_PRICE_FEED_URL selects a mirror. Reload failure keeps the saved snapshot. Delisting hides the marketplace record without disabling an existing deployment. Withdrawn models retain local history. Restoring an unbased custom price returns 409 instead of erasing its only price.

## Source and verification

[internal/gateway/models/schedule.go](../../../internal/gateway/models/schedule.go)

~~~bash
go test ./internal/catalog ./internal/gateway/models -count=1
~~~

[Test prerequisites](testing.md) · [Next: pricing](pricing.md)
