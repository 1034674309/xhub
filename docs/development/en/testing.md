# Test commands and prerequisites

## Choose the verification layer

| Change | Layer |
| --- | --- |
| Parsers / rates / conversion | Package unit tests |
| HTTP / authorization / persistence | Backend regression |
| UI through data plane | Real-browser E2E |
| Docs | Languages, links, highlighting and image loading |

~~~bash
go test ./internal/provider/... ./internal/router ./internal/catalog -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -count=1
make e2e-offline
~~~

PostgreSQL defaults to 127.0.0.1:5433; override with XHUB_TEST_DATABASE_URL. A database SKIP is not a pass; strict mode requires it. Regression creates and cleans an isolated schema per case. Redis tests need a dedicated instance.

## Frontend / website

~~~bash
cd frontend
npm run test:unit -- src/components/route_templates/templateForm.test.ts
npm run test:component -- src/components/route_templates/RouteTemplateSelect.test.tsx
npm run test:types
~~~

~~~bash
cd website
npm test
npm run build
npm run audit:links
npm run test:e2e
~~~

## Side effects

| Command | Boundary |
| --- | --- |
| make regression | Local upstream; free, database required |
| make e2e-offline / e2e-model-endpoints | Local upstream; free |
| make regression-live / e2e / e2e-all | Real providers; incurs charges |
| make testdata | Rebuilds shared xhub/public and Redis DB 1; no inference |

[Backend](regression.md) · [Browser](e2e-regression.md) · [Paid verification](regression-live.md)

Reports must record commands, pass/fail/skip, cleanup and external boundaries. Go OpenAPI sources are not available; routes.json cannot stand in for response schemas.
