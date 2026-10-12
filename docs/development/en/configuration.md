# Configuration and storage

Trace settings by owner. Model providers and deployments are managed in the console; configuration internals below are for maintainers.

## Example

~~~bash
rg 'OverlayDB|MergePatch' internal/config internal/store
rg 'SelectRouteTemplate|Compile' internal/iam internal/router internal/gateway
~~~

## Contract

| Data | Owner | Application |
| --- | --- | --- |
| Startup database / listener | internal/config | Process initialization |
| Providers / deployments | Store | Model catalog and execution |
| Templates / bindings | IAM | Whole-template resolution per request |
| Identity / members / keys | IAM | Fresh authorization |
| RPM / TPM / cooldown / usage queue | Redis | Shared hot state where configured |
| Response cache | Process | Lost on restart |
| Usage / snapshots | PostgreSQL | Historical identity and price |

OverlayDB replaces top-level values, including null/0/false/arrays. MergePatch recursively merges maps and replaces arrays. iam.SelectRouteTemplate selects one whole document, not a field merge. Failed identity/template reads must not silently grant access.

## Source and verification

[internal/config/config.go](../../../internal/config/config.go)

~~~bash
go test ./internal/router ./internal/store -count=1
~~~

[Test prerequisites](testing.md) · [Next: routing](routing.md)
