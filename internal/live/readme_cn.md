# Redis 热状态、限流准入与持久化用量交接

[English](readme.md)

## 说明

`internal/live` 管理必须立即在多个网关进程间共享、但不属于 IAM 持久账本的状态：部署冷却、近期延迟和分钟用量、分层 RPM/TPM 计数、尚未落库的花费、可靠用量队列及短期字符串。PostgreSQL 仍是身份和已结算用量的事实来源；本包填补请求准入到 IAM 事务提交之间的窗口。

本包不注册 HTTP 处理器。请求执行流程从 IAM 构造限流计划，调用 `AdmitRatePlan`，记录部署遥测，并用 `EnqueueSpend` 发布完成用量。刷写器先查看日志、通过 `internal/iam` 提交，再调用 `AckFlushed`。

## 对外接口

### 连接、遥测与计数

- `Open(url) (*Client, error)` 解析 Redis URL、连接并 ping；解析、网络、认证或 ping 失败时关闭客户端并返回错误。`Close()` 对 nil 安全。
- `RecordFailure(id, allowed, cooldown)` 计数并在阈值处设置冷却；`allowed < 1` 或 nil 客户端无副作用。`Cooled(ids)` 返回冷却中的 ID，读取失败降级为空表。`AddLatency` 保留一小时内最近 20 个样本，`Latencies` 对合法样本求平均。
- `AddUsage`/`Usages` 维护部署 UTC 分钟 token 总量。`HitRPM`、`HitTPM` 增加单个分钟计数并返回新值，不提供两维联合准入。

### 限流准入

- `RateScope` 携带主体类型、ID 和可选 RPM/TPM；`RateNode` 增加父索引；`RatePlan` 包含完整分配树和当前调用者从叶到根的 `Path`。
- `CheckRatePlan(plan, rpmUsed, tpmUsed, est) string` 是无副作用参考算法。空串表示通过，拒绝返回 `<kind> rpm_limit` 或 `<kind> tpm_limit`。调用方必须提供有效无环树、合法 path/parent 索引及与 `Nodes` 等长的用量切片；函数会直接索引。负预估按 0，显式零限额会拒绝。
- `AdmitRates(scopes, est, now)` 把每个 scope 转成独立根节点（`Parent == -1`），并按输入顺序生成 `Path`；它不会构造父子链。`AdmitRatePlan(plan, est, now)` 对 UTC 分钟键执行一个 Redis Lua 脚本，先完整检查 RPM，再检查 TPM，全部通过后才增加整条路径。Redis/脚本错误原样返回；网关将其视为依赖故障，不能无计数放行。

### 花费交接与字符串

- `SpendRef(kind, id)` 生成 IAM 配额检查使用的热花费标识。`ChargeSpend`、`HotSpend`、`PeekSpend`、`AckSpend`、`TakeSpend`、`ClearSpendQueue` 提供旧聚合操作；只有查看后确认能在数据库失败时保留数据。
- `EnqueueSpend(SpendLog)` 要求 request ID 和合法主体增量。Lua 原子追加日志并增加所有主体热花费。request ID 是永久幂等键：重试成功但不修改状态，第一次 payload 生效。`EnqueueLog` 只排入日志。
- `PeekLogs(n)` 返回解码值和原始值但不删除，并在首个损坏项处停止。PostgreSQL 提交后，`AckFlushed(deltas, n, head)` 原子校验所见队头、扣减热花费并裁剪前缀；队头变化时不操作。`AckLogs`、`DrainLogs` 是破坏性更强的底层接口。
- `GetString(ctx, key)` 只对存在且可读的键返回 `(value, true)`；`SetString(ctx, key, value, ttl)` 写 TTL 字符串，nil 客户端静默跳过。

调用方包括推理准入和路由、用量刷写器、IAM 配额检查及部署遥测；HTTP 路由由 gateway 包注册。

## 内部核心实现

### 分层预留

树结构用于在共享祖先上为固定子额度预留容量。算法为每个节点计算下方未使用的固定分配：固定子节点贡献 `max(limit-used, 0)`，共享子节点透传固定后代预留。检查路径节点时扣除兄弟预留，再用 `credit` 加回当前路径自己的份额，避免请求被自身额度阻挡。先检查 RPM，再以同一拓扑检查 TPM。函数假设树无环、索引合法且每节点都有对应 usage；损坏输入可能 panic 或无限递归。IAM 负责构造有效计划。

`AdmitRatePlan` 把节点、父索引、限额、预估量和路径编码进 Lua 参数，每节点建立两个键。Lua 读取全部计数、完成两维判断，最后才增加路径键；TTL 为 120 秒。单脚本避免多进程竞争和祖先部分增加，`CheckRatePlan` 用相同规则提供确定性测试。

### 可靠用量交接

`EnqueueSpend` 先验证并序列化事件，再由脚本同时更新日志队列、去重集合、待刷集合和主体浮点计数。IAM 在 `QuotaPath` 中加入热值。刷写顺序是查看、数据库提交、确认：IAM 事务失败时 Redis 状态保留，成功后才进行队头校验确认。IAM 也按 `request_id` 去重，因此提交与确认之间崩溃后重放仍安全。

遥测读取在 Redis 缺失时有意降级，若干可选方法接受 nil；原子准入和可靠排队必须有真实 Redis。入队成功不等待磁盘 fsync 或副本 quorum，Redis 持久化和保留策略是外部部署边界。

### 依赖与验证

本包依赖 Redis 和 `internal/util`，持久消费者是 `internal/iam`。`rate_plan_test.go` 覆盖有效分配树、共享祖先、零限额和预估；计划前置条件仍由调用方保证。`rates_test.go` 对照纯算法验证 Lua 准入和原子增加；`redis_test.go` 覆盖并发去重、非法队列输入、损坏队头、确认和过期。没有测试 Redis 时相关用例跳过。

## 源码职责与入口

### `rate_plan.go`

定义 `RateNode`、`RatePlan`、纯函数入口 `CheckRatePlan`，以及子索引、固定后代预留和路径 credit 等私有辅助逻辑。实现直接索引节点和用量切片，因此结果契约与有效计划前置条件都必须和 Lua 一致。

### `rates.go`

定义 `RateScope`、`rateAdmission` Lua、`AdmitRates`、`AdmitRatePlan`。不校验索引，直接序列化调用方计划，构造 UTC 分钟键和参数、设置两秒 Redis 超时、把脚本节点/维度索引映射为拒绝原因，并返回 Redis 错误。

### `redis.go`

定义 `Client`、`SpendLog`、键规则、连接生命周期、冷却/延迟/分钟遥测、旧花费计数、可靠队列操作和 TTL 字符串。脚本保证入队幂等与确认原子性；私有解析可忽略损坏遥测值，但绝不越过损坏的可靠队列项。
