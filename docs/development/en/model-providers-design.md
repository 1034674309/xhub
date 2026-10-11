# Unimplemented provider extensions

This is a pending-capability index, not a current integration tutorial. Use the [provider UI and Go examples](provider-catalog.md) for shipped behavior.

| Capability | Current boundary | Define before implementation |
| --- | --- | --- |
| OAuth / subscription login | No general chain | Authorization, refresh, revoke and secret storage |
| New cloud signing | Form entry is not an executor | Signing, session credentials, errors and clock skew |
| Suno music | No native executor | Create, status, result/callback and actual usage |
| Separate provider resource API | Old proposals do not establish shipped endpoints | CRUD, migration, deployment references and authorization |

~~~bash
rg 'RegisterTransport|RegisterSupplier' internal/provider
rg 'provider_id|catalog_id' internal/catalog internal/llm
~~~

Inspect registration and execution before proposing API changes. Keep proposed architecture separate from implemented product documentation.
