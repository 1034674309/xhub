# Provider credential forms

Open Model providers → Add provider. Select the provider type first, then enter the address and authentication in that form. Changing type clears old authentication fields; editing a saved connection keeps its type fixed.

## Example

![English provider connection form](../../assets/provider-guide/provider-form-en.jpg)

## Contract

| Definition | Source |
| --- | --- |
| Form fields | internal/catalog/publicdata/provider_fields.json |
| Public merged definitions | /public/providers/fields |
| Form identity | credential_info.provider_id |
| Connection protocol | custom_llm_provider |

The field snapshot is not an implementation of every upstream adapter. OAuth and native cloud authentication chains are not implied by a form entry. Entries without supported authentication cannot be saved as ordinary key connections. Price reload uses XHUB_PRICE_FEED_URL when set, otherwise catalog.MarketURL.

## Source and verification

[internal/catalog/credential_providers.go](../../../internal/catalog/credential_providers.go)

~~~bash
go test ./internal/catalog -count=1
~~~

[Test prerequisites](testing.md) · [Next: provider-catalog](provider-catalog.md)
