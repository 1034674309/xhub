# Browser E2E

## Free isolated runs

~~~bash
make e2e-offline
make e2e-model-endpoints
bash scripts/e2e.sh provider-forms.spec.ts model-credential-protocol.spec.ts
~~~

The [runner](../../../scripts/e2e.sh) uses a real browser, console, gateway and database with local upstreams. Prefer roles/labels over fragile CSS. Delete created resources in finally; the runner cleans schemas.

| Flow | Cases |
| --- | --- |
| Provider → discovery → deployment save | provider-forms / model-discovery |
| Key → Playground → logs | keys-playground |
| Template → binding → actual routing | product-fixes / route-diagnostic |
| Quota → rejection → recovery | quota-hierarchy / rate-hierarchy |
| Guardrail → block / modify / allow | Guardrail specs |

## Paid and shared-data runs

make e2e and make e2e-all call real providers. make testdata clears shared xhub/public and Redis DB 1; it is not an isolated runner. Obtain explicit confirmation before using a working instance.

## Reports

.e2e/runs/<timestamp>-<pid>/ stores evidence; .e2e/latest points to the latest run. Record commands, filters, pass/fail/skip and cleanup. A screenshot alone does not prove HTTP, database and data-plane behavior.

[Test environment](testing.md) · [Real providers](regression-live.md)
