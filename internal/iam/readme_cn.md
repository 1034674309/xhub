# 身份资源、授权资源、额度与账务

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

进程用 Open 建立一次身份库，Open 已自动调用 Migrate；显式重复 Migrate 用于幂等重新应用 schema。之后共享 DB 并在关闭时 Close。生产读取之前由 authz 授权；事务账务与审计接口只接收可信归属。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Open(ctx, dsn) | 返回连接或错误 |
| DB.Migrate(ctx) | 创建/升级 schema，写数据库 |
| DB.GetUser(ctx, id) | 读取用户，不执行动作授权 |
| DB.Close() | 释放数据库连接 |

### 接入 demo

这是供宿主调用的 Go 接入函数。将代码放在仓库内独立包中可编译；实际执行前须由调用方提供下述真实依赖。

```go
package demo

import (
	"context"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// ReadDemoUser 演示迁移后身份读取；调用方提供专用 PostgreSQL DSN 和已授权 ID，返回用户或错误。
// Migrate 会写 schema，必须使用演示数据库；defer 关闭连接，不创建用户记录。
func ReadDemoUser(ctx context.Context, dsn, userID string) (*iam.User, error) {
	db, err := iam.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return nil, err
	}
	return db.GetUser(ctx, userID)
}
```

接入函数可编译；运行需要专用 PostgreSQL 和已授权用户 ID。生产通常在启动迁移一次，不在每次读取时迁移。

### 错误、状态与副作用

迁移和账务会写数据库。五个费用作用域是同一消费的不同视图，不能相加；数据库事务提交前不能确认 Redis 用量队列。

## 说明

`internal/iam` 是认证、管理网关、推理准入和用量报表共用的 PostgreSQL 身份与账务域。它持有用户、组织、团队、成员关系、项目、API 密钥、路由模板绑定、审计记录、不可变用量事件、请求正文、日聚合、各层花费以及 RPM/TPM 分配。本目录只提供 Go 方法；HTTP 身份认证和角色判断由网关完成后再调用这些方法。

资源层级为组织 → 团队 → 用户 → 密钥。项目用于进一步收窄团队权限，但不作为额度树中的独立父节点。一个用户最多属于一个团队，因为账单和速率准入必须得到唯一祖先路径。所有账号均为普通用户，所有密钥均属于用户，不引入业务账号或业务密钥类型。明文密码和密钥都不落库：密码使用 bcrypt，密钥只保存哈希，明文仅在创建或轮换时返回一次。

用量记录采用追加和幂等模型。一个成功批次在同一事务内写入不可变事件、可选请求/响应日志、日聚合和所有适用主体的花费。事件保存调用当时的归属、供应商、模型、token 数和实际应用价格快照；之后成员关系或目录价格变化不会改写历史。异步任务走独立生命周期，只记录一次初始请求，并在任务完成时恰好结算一次成本。

## 对外接口

### 数据库生命周期与错误

`Open(ctx, dsn) (*DB, error)` 通过 xorm/pgx 打开 PostgreSQL、执行 `schema.sql`，并对齐应用、xorm、连接会话和数据库时区。`Migrate` 可重复应用 schema，`Close` 释放连接池。数据库错误向上传递，常见约束结果会归一为 `ErrNotFound`、`ErrConflict` 或 `ErrInvalid`。`RecordAudit`、`ListAudit`、`QueryAudit` 写入或分页读取审计记录；搜索在计数和分页前执行，显示名优先使用 detail 中的不可变快照，再批量回填当前名称。

### 用户、组织、团队、项目与密钥

`Bootstrap`、`EnsureAdmin`、`Login`、`CreateUser`、`GetUser`、`UserByEmail`、`UpdateProfile`、`SetPassword`、`AdminUpdateUser`、`DeleteUser` 实现账号生命周期。输入会被规范化和校验，密码只保存哈希，删除或降级最后一个平台管理员会被拒绝。`ListUsersPage(UserListQuery)` 把搜索、状态、角色、团队、组织筛选与调用方提供的授权范围取交集，仅允许白名单排序字段，默认页大小 100、上限 500，返回 `(rows, total, error)`。用户列表的 SSOID 条件没有对应用户查询字段，指定时返回空结果；企业 OIDC 绑定另存于 enterprise_login_identities，该列表条件不会查询这些绑定。

组织和团队入口包括 `CreateOrg(..., rates ...RateLimits)`、`CreateOrgWithConfig(..., OrgConfig, rates ...RateLimits)`、`UpdateOrg`、`CreateTeam`、`UpdateTeamProfile`、`AdminUpdateTeam(..., rates ...RatePatch)`、`MoveTeam` 及相应查询、列表和删除方法。成员方法负责组织管理员和团队成员的增删改，保留至少一个团队管理员，并拒绝同一用户进入第二个团队。项目方法保证项目模型和预算不超出所属团队。变更在事务内写审计；外键、配额或速率分配失败会回滚整个变更。

`CreateKey` 和 `RotateKey` 返回 `(key, plaintext, error)`，明文只暴露一次。`KeyByHash` 是认证查询入口；`ActiveKey`、`OwnedBy`、`TouchKey`、状态/删除/重置方法和筛选列表支撑网关策略。`AllowedModelsForUser`、`AllowedModelsForTeam` 与 `AllowedModelsForKey` 对组织、团队、项目、密钥模型集合逐层取交集；某层为空表示继承，最终 nil 表示整条链未限制，非 nil 空结果表示无可用模型。组织模型为平台设置的授权上限，修改后立即影响已有会话和密钥；团队创建、修改或转移不得突破目标组织范围。组织 `models` 与 JSON 对象 `metadata` 在创建、详情、列表和部分更新中往返；省略字段保留原值，null 或空集合显式清空。启动迁移为现有组织添加两个默认空字段，不导入旧系统数据。`ParseExpiry` 只接受支持的相对时长格式，错误格式直接失败。`ResetKeySpend` 修改当前计数，但不删除历史事件。

### 配额、速率、模板与用量 API

`QuotaPath(ctx, kind, id, hot)` 返回 `(teamID, organizationID, exhaustedScope, error)`。它仅读取调用者所在组织（含兄弟预留），无团队个人仅读取自身和个人密钥，通过 `hot` 加入尚未刷库的 Redis 花费，为固定额度子节点保留未消费余额，并报告目标路径上第一个耗尽层级。父节点缺失、成员归属歧义和非法分配都会返回错误。它是调用前检查，供应商最终结算仍可能超过最后一点余额。

`ValidateRates` 拒绝负数以及超出 PostgreSQL `INT` 的 RPM/TPM。`RatePlan(ctx, kind, id)` 返回 `live.RatePlan`，其中既有目标路径，也保留会占用祖先容量的兄弟及后代节点；节点缺失、父级歧义和循环会失败。IAM 负责构造有效无环的索引与路径，`live.CheckRatePlan` 和 `live.AdmitRatePlan` 会直接使用这些索引。

路由模板 API 负责 JSON body 的创建、更新、列表、读取、删除，以及组织/团队/项目绑定。`Settings` 遇到存量非法 JSON 时返回空设置，避免单条坏记录阻断所有继承请求。`RouteTemplateUsage` 和 `RouteTemplateUsageCounts` 返回绑定位置；网关删除前先检查使用情况。空绑定表示恢复继承。

`RecordUsage(records)` 在写库前验证整个批次，去除批次内重复 request ID，并依靠数据库 request ID 唯一约束实现跨请求和并发幂等。任一步失败都会返回错误并回滚事件、日志、聚合和主体计数。`RecordTaskUsage(record, initial, settle)` 通过 PostgreSQL advisory transaction lock 串行处理同一任务：初建只计一次请求且费用为零，轮询只更新状态和正文，首次完成结算才增加 token 与花费；归属和开始时间始终沿用初始事件。

普通调用在未配置 Redis 或 Redis 入队失败时，先通过 `RecordUsageDurably` 独立提交 `usage_pending` 中的完整快照，再尝试 `RecordUsage`。账单事务或确认删除失败时保留快照，`FlushPendingUsage` 在无 Redis 时也会重放，新实例可以继续恢复；重放不重新调用供应商、不按当前价格重算，成功后才删除快照。`durable=false` 明确表示快照本身未保存：本机制不覆盖 PostgreSQL 整库不可写、磁盘丢失或任务生命周期结算，也不新增预算预占能力。已经完成的上游响应不会因账单事务失败改成可诱发客户端重试的 500。

`UsageQuery` 同时携带授权范围和业务筛选。`ListUsageSessions`、`ListUsage`、`CountUsage`、`GetUsageEvent`、`GetRequestLog`、`DailyUsage` 以及模型/密钥/团队汇总，都会先把授权条件写入 SQL，再排序、计数和分页。`AuditLogRead` 记录特权请求日志读取。SQL、解码和上下文错误原样上送；集合查询无结果时返回空切片，单条查询无结果时返回 `ErrNotFound`。

### 实际 HTTP 注册与调用方

本目录不直接挂载 HTTP handler。`internal/gateway/identity/mount.go` 将这些 API 注册为 `/user/*`、`/organization/*`、`/team/*`、`/project/*`、`/route_template/*` 和 `GET /audit/logs`；`internal/gateway/routes.go` 的登录和 bootstrap handler 也调用 IAM。`internal/gateway/keys/mount.go` 注册密钥生成、服务密钥、列表/详情、更新、阻止/恢复、删除、批量、轮换/重置、别名和密钥健康接口。`internal/gateway/usage/mount.go` 注册花费日志与详情、会话/全局报表、日活动、模型/密钥/团队/供应商/标签汇总、成本计算和健康接口。网关中间件传入 actor 和授权范围，并把 IAM 错误映射成 HTTP 状态。推理认证会调用密钥、用户、模型检查以及共用一次归属读取的 `QuotaAndRates`；后续目标复核使用 `QuotaPath`，不再占分钟容量；live 花费刷写器调用 `RecordUsage`。

## 内部核心实现

### 事务、约束与时间

`DB.tx` 绑定上下文、开启事务，在回调或提交失败时回滚，全部领域写入成功后才提交。并发分配可能冲突时，资源变更会锁定受影响的用户、组织或团队。`schema.sql` 提供 request ID 唯一约束、成员和绑定外键、检查约束、索引、聚合表与审计存储。连接串注入具名 session 时区，数据库服务器使用匹配的 POSIX 时区，使应用写入时间与 `DEFAULT now()` 保持同一时刻，避免 UTC+8 环境出现八小时偏移。

### 预算与速率分配

`quotaTree` 读取组织、团队、用户、成员和密钥并连接成森林。固定预算子节点向父级保留 `max(limit-spend, 0)`；共享子节点把固定后代的保留额继续上传。`validateQuota` 只校验被修改节点及其祖先：分配调整不回退已有消费，也不阻断无关资料修改。运行时 `quotaTreeFor` 仅读取当前归属分量，首次准入通过 `QuotaAndRates` 共用读取，目标复核通过 `QuotaPath` 读取最新数据。两者把 Redis 热花费同时纳入路径消费和保留额计算，并把目标子节点自身的保留额度作为 credit 加回，避免它被分配给自己的容量拦截。

RPM 和 TPM 复用相同拓扑。`rateAllocated` 累加固定子节点，并让固定后代穿过共享节点；零是真实限额，nil 表示共享或未设置。`buildRatePlan` 从目标向根回溯并检测循环、缺失父级，再加入完整根分量，确保兄弟保留额参与判断。节点 ID 排序后得到稳定索引，路径按从窄到宽输出。配置变更在同一事务内校验非负值与父级分配容量。

### 身份不变量与审计快照

邮箱按不区分大小写方式规范化，密码使用 bcrypt，存储角色和控制台角色分别转换。成员变更要求活跃用户，禁止多个团队归属路径，并保留最后一个团队/平台管理员。退出团队保留个人密钥，新调用按当前归属和计费授权检查，历史事件保持原归属；用户删除或目录停用时可以撤销凭证。模型范围按组织、团队、项目、密钥逐层求交集，预算按归属链求上限。审计写入尽量捕获 actor/object 名称；读取时不会为补全名称扩大授权，因此已删除资源可能保持无名。

### 用量账务与失败边界

`RecordUsage` 在事务前拒绝空 ID、负数或非有限价格、非法 token 和时间，再由 `insertEvent` 判断请求是否首次出现。只有新事件才写请求日志、增加日请求数，并增加组织、团队、项目、用户和密钥花费；所有聚合使用事件捕获的归属和日期。数据库失败不会留下部分计数。Redis 队列只在数据库事务提交后确认，而 request ID 唯一约束使提交后、确认前崩溃产生的重放仍安全。

任务账务甚至在记录尚不存在时也按 request ID 哈希加事务锁。重复 initial 是空操作；轮询不能增加请求数或花费；失败任务不能回到执行态；结算必须是 `completed` 且 key/user/model 与原事件一致。零费用完成也会标记为已结算。用量读取先构造授权 SQL，再叠加筛选、稳定排序、总数和分页；本地日统计按请求的分钟偏移移动时间。

### 依赖与验证

本包依赖 PostgreSQL/pgx/xorm、bcrypt、用于速率计划类型的 `internal/live` 和 `internal/util`。单元与回归测试覆盖时区设置、密钥过期和个人密钥、额度/速率分配、用户分页、路由模板、审计筛选/快照、用量分页/幂等和任务生命周期。数据库测试需要可用的 PostgreSQL 测试 DSN，缺失时可能跳过；Redis 行为由 `internal/live` 测试，外部身份供应商和模型供应商不在本包边界内。

## 源码职责与入口

- [`audit_display.go`](audit_display.go) 实现审计文本提取、分页前搜索谓词，以及授权分页后的批量显示名回填，由 `QueryAudit` 调用。
- [`db.go`](db.go) 定义 `DB`、统一领域错误、`Open`/`Migrate`/`Close`、事务/session/时区辅助、`Actor`/`Audit`/`AuditFilter`、审计写入和审计列表/查询入口。
- [`keys.go`](keys.go) 定义密钥记录和输入、secret 生成/哈希/轮换、生命周期和筛选、模型交集、预算上限、过期解析、团队密钥撤销与花费重置；认证入口是 `KeyByHash`，管理入口是各 CRUD/状态方法。
- [`models.go`](models.go) 定义用户、组织、团队、成员、项目、审计和 bootstrap 持久化结构，状态/角色常量、表映射、活跃/管理员判断和 UUID 生成。
- [`quota.go`](quota.go) 构建并校验私有额度树，计算固定子级保留和速率分配，限制单团队归属，并导出 `QuotaPath`、`ErrQuotaAllocation`、`ErrMultipleTeams`。
- [`rates.go`](rates.go) 定义 `RateLimits`、区分未提交与显式清空的 patch、数值校验、`RatePlan`，以及供 `internal/live` 使用的稳定根分量计划构造。
- [`route_template.go`](route_template.go) 定义模板归属与存储、JSON 设置解析、CRUD、绑定校验、scope 表选择和使用位置/数量查询。
- [`task_usage.go`](task_usage.go) 通过 `RecordTaskUsage` 实现 advisory lock 任务生命周期和 completed 恰好一次结算。
- [`teams.go`](teams.go) 实现组织/团队 CRUD、可见范围和管理员查询、成员不变量、项目 CRUD、层级锁、模型/预算收窄、速率 patch 和事务审计。
- [`usage.go`](usage.go) 定义事件、请求日志、日记录和 `UsageRecord`；`RecordUsage` 与私有事件/日志/日聚合/scope 辅助函数实现原子幂等账务。
- [`usage_read.go`](usage_read.go) 定义授权 `UsageQuery`、会话和聚合结果类型、SQL scope/筛选/排序构造、事件/日志查询、日报、汇总与特权读取审计。
- [`user_pagination.go`](user_pagination.go) 定义 `UserListQuery`、带授权的目录 SQL、排序白名单、limit/offset 规范化和 `ListUsersPage`。
- [`users.go`](users.go) 实现 bootstrap、登录和 bcrypt、角色转换、账号 CRUD、授权/普通列表、锁、最后管理员保护以及资料/密码/管理员更新。
- [`schema.sql`](schema.sql) 是全部 IAM 资源、约束、索引、用量聚合、请求日志、审计、路由模板绑定和 bootstrap 状态的幂等 PostgreSQL schema，由 `Open` 和 `Migrate` 执行。
