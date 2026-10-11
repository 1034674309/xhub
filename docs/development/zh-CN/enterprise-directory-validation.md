# 企业目录阶段实施与验证记录

日期：2026-10-11。状态：目录、OIDC、Kodo 适配器及配置页面完成本地分层验证；真实企业平台/七牛账号验收尚未执行。

## 本次补齐内容与执行记录

修改文件分组（不包含工作区原有的其他产品改动）：

- 后台：`internal/iam/enterprise_oidc.go`、`enterprise_schema.sql`、`archive.go`、`archive_schema.sql`、`directory_patch.go`、`db.go`；`internal/gateway/enterprise_oidc.go`、`enterprise_security.go`、`archive.go`、`directory_standard.go`、`directory.go`、`routes.go`、`server.go`、`health/health.go`；`go.mod`、`go.sum`。
- 页面：`DirectorySettings.tsx`、`DirectoryManagement.tsx`、`EnterpriseConfiguration.tsx`、`EnterpriseLogin.tsx`、`LoginPage.tsx`、`lib/http/directory.ts`、`lib/http/enterprise.ts`、`messages/en.ts`、`messages/zh-CN.ts`。
- 测试：上述组件测试、`frontend/src/lib/http/directory.test.ts`、`frontend/e2e/directory.spec.ts`、`internal/iam/directory_patch_test.go`、`internal/gateway/enterprise_test.go`、`enterprise_oidc_test.go`、`archive_sdk_test.go`、`health/health_test.go`。
- 文档：本文件及 `enterprise-identity-log-archive-design.md` 的当前实施状态和接入指南。

实际命令及结果：

1. `XHUB_REGRESSION_STRICT=1 go test -json ./internal/iam ./internal/gateway ./internal/gateway/health -run '^Test(DirectoryPatch|EnterpriseSecurity|ArchiveEncoding|EnterpriseHTTP|EnterpriseOIDCFlow|DirectoryHTTP|DirectoryLifecycle|SCIMPaging|SelectDirectoryTeam|KodoSDKProtocol|ProbeBoundaries)$' -count=1`：11 顶层 + 25 子测试通过，失败 0。报告 `.e2e/enterprise-backend.jsonl`。
2. `XHUB_REGRESSION_STRICT=1 go test -json ./internal/iam ./internal/gateway/health -count=1`：53 顶层 + 31 子测试通过，失败 0。报告 `.e2e/enterprise-iam-full.jsonl`。
3. `npm run test:unit -- src/lib/http/directory.test.ts`：1 测试通过，失败 0。
4. `npm run test:component -- src/components/Settings/AdminSettings/DirectorySettings.test.tsx src/components/Settings/AdminSettings/DirectoryManagement.test.tsx src/components/Settings/AdminSettings/EnterpriseConfiguration.test.tsx src/components/EnterpriseLogin.test.tsx src/app/login/LoginPage.test.tsx`：5 文件、20 测试通过，失败 0。
5. `npm run test:integration -- src/app/login/LoginPage.integration.test.tsx`：1 文件、5 测试通过，失败 0。

后台覆盖真实隔离 PostgreSQL、HTTP 路由、PATCH 回滚、停用连接拒绝登录、单次 state、真实本地 TLS OIDC discovery/JWKS/token、错误 nonce/audience/issuer/signature/过期/未知用户、PKCE、Kodo 官方 SDK multipart 和签名/禁止覆盖/内容重试/探针删除、计费 outbox 幂等/结算版本/原目标版本保持/租约 fencing/失败重试/正文排除。外部测试夹具不会写入真实企业或七牛账号，数据库由 schema 清理。

浏览器新增覆盖组织下拉、同组织团队下拉、公开 SCIM 地址、SSO 回调生成和配置持久化、标准发现与用户 PATCH、Kodo 区域下拉、凭据清空且不回显、停用配置保存加载、失败状态与重试、中英文切换。运行命令使用 `E2E_MASTER_KEY=sk-e2e-enterprise-encryption-key-32-characters E2E_BROWSER_REPORT_DIR=/Users/sunqirui/gitlab/aiagent/xhub/.e2e/enterprise-report bash scripts/e2e.sh directory.spec.ts`，报告 `.e2e/enterprise-report/results.json` 和 `junit.xml`。首次新增配置保存测试因原测试主密钥不足 32 字符被产品正确拒绝，使用合规测试密钥后重跑；不得关闭安全校验来使测试通过。

最终企业配置 E2E：1 流程通过、失败/跳过 0；`go test -json ./internal/gateway/health -count=1`：2 测试通过、失败 0，报告 `.e2e/enterprise-health.jsonl`。

### 截图布局调整

简洁化迭代：SCIM 默认仅展示公开地址与连接卡片，新建连接、接入说明及团队映射按需展开；SSO 与 Kodo 表单采用响应式双列，取消重复说明与弹窗内嵌套边框，归档列表收进明细折叠区。中文、英文说明同步维护，原始输入和秘密清理规则保持不变。

简洁化最终结果：组件测试 10 通过、失败 0；E2E 1 流程通过、失败/跳过 0；production build 与 TypeScript 通过。

本轮运行同下文组件测试命令，3 文件、10 测试通过，失败 0；初次因隐藏日志面板含同名说明导致测试定位不唯一，限定到当前 SCIM 页签后重跑通过。E2E 使用 `E2E_MASTER_KEY=sk-e2e-enterprise-encryption-key-32-characters E2E_BROWSER_REPORT_DIR=/Users/sunqirui/gitlab/aiagent/xhub/.e2e/enterprise-simple-report bash scripts/e2e.sh directory.spec.ts`，新增验证新建表单和团队映射默认隐藏、展开后执行真实操作、中英文入口切换。报告 `.e2e/enterprise-simple-report/results.json`、`junit.xml`。本轮仅改页面呈现，不改后台接口/持久化/鉴权；后台边界沿用前一轮验证，未重新执行独立 Go 回归，真实接口由本轮 E2E 覆盖。

管理员面板采用「单点登录设置 / SCIM / 日志设置」三个页签。SSO 展示目录连接卡片，通过弹窗配置；无连接时展示空态并引导创建目录。SCIM 分为租户地址及凭据两步，支持完整地址复制与失败提示。页签支持方向键及 Home/End，切换保留输入，一次性凭据离开页面后清除。

身份平台下拉提供通用 OIDC、Microsoft Entra ID、Okta、Keycloak、Auth0 配置辅助模板，不声明真实厂商认证。模板不持久化，不猜测租户 URL；Entra 预选 oid、其余预选 sub，管理员仍须与 SCIM externalId 对齐。实际登录参数由后台持久化。

截图里的安全设置、界面设置、Vault、插件不在本次范围，未创建无效页签；日志保留期没有对应后台能力，未添加假保存字段。现有提示词日志开关保留在日志页，Kodo 归档始终排除提示词与响应正文。

本次复验命令及结果：

- frontend 目录：`npm run test:component -- src/components/Settings/AdminSettings/DirectorySettings.test.tsx src/components/Settings/AdminSettings/DirectoryManagement.test.tsx src/components/Settings/AdminSettings/EnterpriseConfiguration.test.tsx`：3 文件、10 测试通过，失败 0。覆盖页签隔离、空态、键盘导航、复制成功/失败、身份模板。
- 根目录：`XHUB_REGRESSION_STRICT=1 go test ./internal/iam ./internal/gateway ./internal/gateway/health -run '^Test(DirectoryPatch|EnterpriseSecurity|ArchiveEncoding|EnterpriseHTTP|EnterpriseOIDCFlow|DirectoryHTTP|DirectoryLifecycle|SCIMPaging|SelectDirectoryTeam|KodoSDKProtocol|ProbeBoundaries)$' -count=1`：3 包通过、失败 0，同上文 JSON 报告的 11 顶层/25 子测试选择。
- 根目录：`E2E_MASTER_KEY=sk-e2e-enterprise-encryption-key-32-characters E2E_BROWSER_REPORT_DIR=/Users/sunqirui/gitlab/aiagent/xhub/.e2e/enterprise-design-report bash scripts/e2e.sh directory.spec.ts`。报告 `.e2e/enterprise-design-report/results.json`、`junit.xml`。覆盖页签切换、SSO 弹窗保存/加载/关闭、身份模板、真实 SCIM 同步与归属、日志存储配置及错误、中英文切换，包含 production build 和 TypeScript。

截图布局最终 E2E：1 流程通过，失败/跳过 0，production build 与 TypeScript 检查通过。

下列原始阶段记录保留用于说明演进，最新状态以本节和末尾边界为准。

## 修改范围

- `internal/iam/directory_schema.sql`、`directory.go`、`directory_assignment.go`、`directory_resources.go`、`directory_groups.go`：连接、身份、组、映射、单团队选择及管理员快照。
- `internal/iam/db.go`：接入幂等目录迁移。
- `internal/gateway/directory.go`、`directory_groups.go`、`routes.go`：管理 API 与基础 SCIM 路由。
- `frontend/src/components/Settings/AdminSettings/DirectorySettings.tsx`、`DirectoryManagement.tsx`、`frontend/src/lib/http/directory.ts`、管理员面板及中英文消息：创建连接、启停、一次性凭据、组映射和人工归属。
- 对应 IAM、Gateway、组件测试及 `frontend/e2e/directory.spec.ts`。

## 实际执行与结果

| 命令 | 结果 |
| --- | --- |
| `XHUB_REGRESSION_STRICT=1 go test ./internal/iam ./internal/gateway -run '^Test(SelectDirectoryTeam|DirectoryLifecycle|SCIMPaging|DirectoryHTTP)$' -count=1` | 4 个顶层测试通过，18 个子测试通过，失败 0；真实隔离数据库 |
| 同上加 `-json` | 再次通过，确认上述计数 |
| `XHUB_REGRESSION_STRICT=1 go test ./internal/iam -count=1` | IAM 全量回归通过，退出码 0 |
| `npm run test:component -- src/components/Settings/AdminSettings/DirectorySettings.test.tsx src/components/Settings/AdminSettings/DirectoryManagement.test.tsx`（frontend 目录） | 2 文件、4 测试通过，失败 0 |
| `E2E_BROWSER_REPORT_DIR=/Users/sunqirui/gitlab/aiagent/xhub/.e2e/directory-report bash scripts/e2e.sh directory.spec.ts` | 最终 1 流程通过，失败/跳过 0；包含 production build 与 TypeScript 检查 |

初次后台测试受沙箱网络限制无法连接本机数据库，授权后成功。第一次扩展浏览器测试在语言切换后未重新加载内存快照而失败；按真实页面重新挂载行为补充加载操作，重跑通过。没有把失败运行当作通过。

E2E 覆盖：管理员登录、创建独立组织和团队、创建目录连接、一次性凭据、未启用拒绝同步、启用连接、SCIM 用户与组同步及读取、页面保存映射并核对真实团队归属、人工固定无团队并核对后台结果、恢复自动分配、用户停用、删除组、隐藏凭据、切换英文并重新加载管理快照。测试 schema 在结束时清理。

报告路径：`.e2e/directory-report/results.json`、`.e2e/directory-report/junit.xml`。

## 未覆盖及未实现边界

- OIDC 与 Kodo 官方适配器已实现，未使用真实 Entra/Okta 租户或七牛 AK/SK；不宣称厂商互操作认证。完整浏览器身份平台重定向未覆盖，OIDC 协议链由后台 TLS 测试验证。
- SCIM PATCH、Schemas、ResourceTypes 已实现。Bulk、任意筛选/排序、嵌套组、完整企业扩展不支持；稳定 externalId 是本产品必要映射条件，需按提供方配置验收。
- 历史补归档、多行批量文件、空间私有性自动校验、真实七牛网络/账号策略/区域匹配未覆盖。空间需管理员设为私有，探针成功不等同私有性验证。
- 默认团队和凭据轮换尚无页面；归属 UI 已使用同组织团队下拉，后台提交稳定团队 ID。
- 本次未运行全项目所有后台与浏览器测试，不宣称全项目验收完成。
