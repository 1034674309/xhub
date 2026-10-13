# Provider integration boundaries

See [provider UI and Go examples](provider-catalog.md) for integration. Credential catalogs, public endpoints and registered transports jointly determine support. A selectable provider form does not establish execution support for every protocol.

| Capability | Current behavior |
| --- | --- |
| OAuth / subscription login | No general provider authorization, refresh or revoke execution chain |
| Cloud signing | Only registered signing and credential methods execute |
| Suno music | No native executor; speech and video protocols do not implement music |
| Provider connection management | Registered credential routes manage named connections referenced by deployments |

~~~bash
rg 'RegisterTransport|RegisterSupplier' internal/provider
rg 'provider_id|catalog_id' internal/catalog internal/llm
~~~
