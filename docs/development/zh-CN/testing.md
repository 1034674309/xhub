# 测试命令与环境

## 按修改选验证

| 修改 | 验证层 |
| --- | --- |
| 解析 / 费率 / 转换 | 包单元测试 |
| HTTP / 鉴权 / 持久化 | 后台 regression |
| 页面到数据面 | 真实浏览器 E2E |
| 文档 | 双语、链接、代码高亮、图片加载 |

~~~bash
go test ./internal/provider/... ./internal/router ./internal/catalog -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -count=1
make e2e-offline
~~~

PostgreSQL 默认 127.0.0.1:5433，也可设置 XHUB_TEST_DATABASE_URL。数据库不可达出现 SKIP，不算通过；strict 模式要求数据库可用。regression 每用例创建独立 schema 并清理；Redis 专项必须使用独立实例。

## 前端 / 网站

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

## 不要误运行

| 命令 | 副作用 |
| --- | --- |
| make regression | 本地协议上游，免费；需要数据库 |
| make e2e-offline / e2e-model-endpoints | 本地上游，免费 |
| make regression-live / e2e / e2e-all | 真实供应商，会收费 |
| make testdata | 清空重建共享 xhub/public 和 Redis DB 1，不调用模型 |

[后台回归](regression.md) · [浏览器回归](e2e-regression.md) · [付费验证](regression-live.md)

报告必须列命令、pass/fail/skip、清理与依赖边界。前端生成类型缺少 Go OpenAPI 来源，不能用 routes.json 伪造响应 schema。
