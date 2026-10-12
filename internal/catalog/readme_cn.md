# 模型目录、路由分类与用量计价

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

执行链先 NormalizeUsage，再以请求开始时刻调用 CostFromRates 或 CostAt。检查 priced 后保存 Charge 对应快照，持久化由 IAM 结算链完成。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| NormalizeUsage(map) | 统一实测量及媒体计价维度 |
| CostFromRates / CostAt | 返回 Charge 和可计价标记 |
| Snapshot / SnapshotUsage | 序列化当次价格事实 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/catalog"
	"time"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	usage := catalog.NormalizeUsage(map[string]any{"prompt_tokens": 100, "completion_tokens": 20})
	rates := []catalog.Rate{
		{Measure: "token", Side: "input", Window: "all", USD: 0.000001},
		{Measure: "token", Side: "output", Window: "all", USD: 0.000002},
	}
	charge, priced := catalog.CostFromRates(rates, usage, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	fmt.Printf("%t %.6f\n", priced, charge.Total)
}
```

输出 true 0.000140；使用显式固定费率，结果不依赖远端市场源。

### 错误、状态与副作用

Rate.USD 已是每基础单位单价，不能再除 UnitSize。priced=false 表示无法计价，不能解释为免费；刷新目录不能重算历史账单。

## 说明

`internal/catalog` 为网关提供路由目录、模型/供应商元数据、凭据字段描述、市场价格转换、运行时价格分层、用量归一化和费用计算。进程以内嵌资源启动，接受供应商包贡献，可用远端 feed 替换生成的市场层，也允许管理接口应用运行时模型/供应商覆盖。目录在读写锁保护下驻留内存；内嵌文件是启动输入，不会在运行时监视磁盘变化。

计价支持 token、图片、秒和查询次数，可按输入/输出侧、缓存、媒体规格以及北京时间高峰/低谷选择不同费率。`Rate.USD` 是导入阶段换算后的每基础单位价格，计费时直接乘用量，不能再次除以 `UnitSize`。字段存在且为零表示真实免费，费率缺失则表示对应用量无法计价。费用快照同时保存实测量和实际应用费率，因此后续目录刷新不会改变历史账单。

本包还负责 public、推理、数据面和已移除 mixed 路径的分类。`routes.json` 描述路由形状和公共响应元数据，不代表每个供应商都有可用适配器或账号；凭据元数据也只描述配置字段。

## 对外接口

### 路由与公共数据 API

`Load() []Route` 解码内嵌 `routes.json`，非法 JSON 会记录日志并返回空路由。`AuthOf`、`IsMixedPath`、`IsPublicPath`、`IsDataPlanePath`、`IsLLMPrefix` 对规范化路径分类。`HasPrefix` 遵守路径段边界，`PathMatch` 支持单段 `{name}` 和剩余路径 `:path`/`{name:path}` 占位。`PublicBody(path)` 为受支持公共路径返回内嵌固定结构，普通目录项返回空列表。这些函数只读，不自行完成请求认证。

`Raw`、`CostMap`、`Count`、`LoadedAt`、`ProviderModels`、`KnownProvider` 向模型列表、通配展开、公共模型中心和管理接口提供当前内存目录及供应商索引。`TokenRates(model)` 返回每基础 token 的输入/输出价格，没有合适目录行时 `ok=false`。`Cost(model, prompt, completion)` 是仅 token 的兼容计算器，`Format` 输出无多余尾零的小数字符串。返回映射应作为目录数据读取，修改必须走写入 API。

`Providers()` 返回价格文档中的供应商行。`CredentialProviders()` 将其与独立内嵌的凭据注册表和供应商贡献合并，补充默认 API base、应用 DeepSeek 兼容字段、按 provider/slug 去重、按显示名排序，并深拷贝追加的价格供应商，避免调用方修改共享状态。该结果只描述 UI 凭据输入，不验证适配器或凭据可用性。

### 计价与用量 API

`NormalizeUsage(map[string]any) Usage` 把供应商不同格式归一为 token、缓存 token、图片、视频秒数和查询次数。OpenAI/Responses 的嵌套 cached token 是 prompt/input 总量子集，会从未缓存输入中扣除；Anthropic 顶层 cache read/write 与 `input_tokens` 并列，需要相加且不能重复计算。显式零优先于别名，非法或互相矛盾的数量不会变成可计费用量。

`CostAt(model, usage, startedAt)` 选择当前目录行；只有至少一个上报量有兼容费率时才返回 `(Charge, true)`。未知模型或无法计价的度量返回 `ok=false`，含义是未计价而不是免费。`CostFromRates` 对调用方费率表计价；`RatesFromFlat` 和 `CostFromFlatOrRates` 转换旧平铺字段，同时保留缺失与零的差别。`DecodeRates` 接受内嵌或部署 JSON 形状。`WindowAt`/`IsPeakHour` 根据调用开始时间选择时段。`Snapshot` 保存应用费率，`SnapshotUsage` 保存异步实测量、费率和是否成功计价。

### 目录修改与 feed

`Contribute(Row)` 允许供应商包补充缺失模型和官方 ID 别名，但不会覆盖内嵌价格；`ContributeProvider(ProviderRow)` 补充凭据/下拉元数据。空 ID 或供应商会被忽略。`BaselineModel`、`ModelRow`、`IsBaseline`、`SetModel`、`RemoveModel` 区分生成基线与当前模型行；供应商对应入口是 `SetProvider`、`RemoveProvider`、`IsBaselineProvider`。set/remove 在全局锁内重建相关索引并修改进程内存，持久化覆盖由调用方负责。

`BuildPriceDocument(raw)` 校验并把市场响应转换为 `PriceDocument`，非法输入或空模型集合会失败。`FetchMarket(ctx, url)` 按上下文发起 HTTP 请求并返回传输、状态码或解码错误。`ReloadFromMarket` 拉取后调用 `ApplyDocument`；`ApplyDocument` 原子替换生成的 live 层，拒绝空文档，重建供应商索引，并重新叠加本地供应商/模型贡献和覆盖。拉取或转换失败时当前目录保持不变。`PriceSource`、`PriceGeneratedAt`、`MarkReloaded`、`EnvForced` 暴露来源、刷新状态和仅本地目录环境开关。

### 实际 HTTP 注册与调用方

本包不直接挂载 handler。`internal/gateway/models/mount.go` 用它实现 `GET /v1/models`、`GET /models`、路由组/default/fallback/available/info、`GET /model/cost_map`、来源/刷新/定时刷新、`GET /price/catalog`、模型/供应商价格写入与重置，以及模型 CRUD/启用/禁用/内置模型接口。`internal/gateway/routes.go` 注册 `GET /public/v1/model_hub` 及 facet、`/public/model_hub`、`/public/model_hub/info`、`/public/endpoints`、`/model_hub` 及 facet 和 `POST /model_hub/update_useful_links`。推理和用量 handler 在 IAM 保存快照前调用归一化与计价；供应商包在注册时调用贡献 API。HTTP 校验、授权、持久覆盖、调度和状态码映射位于 gateway/store 层。

## 内部核心实现

### 分层目录与并发

启动时解析内嵌 `pricedata.json`，按 `litellm_provider` 索引模型并记录载入时间。实现把不可变基线模型/供应商映射与可变 live 映射分开。市场文档替换生成层，操作员行和供应商包贡献再叠加；显式 seller/source 绑定防止无关 feed 来源抢占已定价行。全部读写使用 `modelCostMu`，写操作在释放锁前重建别名和供应商索引，因此读者只会看到完整旧目录或完整新目录。空文档会被拒绝，避免坏 feed 清空账务价格。

模型查询按顺序生成候选键，包括请求的网关名、供应商前缀形式、官方别名和 price-model 绑定；非对象或无价格空壳会被跳过，直到找到兼容行。`KnownProvider` 决定通配展开时首段是否为供应商，避免错误剥离组织/模型名。贡献会记录官方别名与价格来源，但不会覆盖基线已有行。

### Feed 转换与费率协议

`feed.go` 接受市场响应的多种字段形状，归一化供应商/模型身份、模式、variant、单位大小和原始价格字段，再生成 `PriceDocument`。已知 feed key 映射为明确的 `measure`、`side`、`variant`、`window`；未知 key 走字符串推断并输出 debug 诊断。`measureOf` 映射 token、second、picture、query，`perUnit` 把按单位包报价恰好一次换算为每基础单位美元。只有元数据而无价格的模型仍可展示但保持未计价；真实数字零会保留。

`Rate` 携带度量、侧、可选 variant/window、USD、来源字段和原单位元数据。费率选择先匹配度量和侧，再匹配具体用量 variant 与开始时段。无时段费率可覆盖所有时段；若费率表明确区分高低峰，缺少高峰价时不会偷偷用更便宜的低谷价。缓存读取绝不退回普通输入价。媒体规格互相矛盾时计价失败，而不是任意选档。应用费率和数量分别累计到 `Charge`。

### 用量归一化与时段

私有数值读取器把“是否存在”和数值分开追踪，在别名之间保留 null、缺失、显式零的差别。嵌套缓存子集会被限制到输入总量；顶层并列缓存量保持相加。图片 modality、分辨率、秒数和搜索查询进入独立 measure/variant 桶，避免不同度量串价。总实测量为零不会被视为已计价调用。

高峰为 `Asia/Shanghai` 周一至周五 09:00–12:00、14:00–18:00，边界使用左闭右开。内嵌中国节假日数据强制法定假日休息、调休周末上班。时区数据不可用时初始化退化到固定 UTC+8；节假日 JSON 无法读取时记录错误并退化为仅按工作日判断。价格时段只由调用开始时间决定。

### 凭据元数据、失败边界与验证

`provider_fields.json` 与价格供应商分开解析。凭据合并会复制嵌套 map/slice，应用已知 API base 默认值，只在 provider/slug 不冲突时追加贡献字段，并为 DeepSeek 加入 `api_base` 兼容字段。随附 `provider_fields.LICENSE` 记录上游许可，分发时必须保留。解析失败会记录日志并返回其他仍有效来源，不会中止模型计价。

测试覆盖路由授权回归、凭据供应商合并与隔离、feed 转换和空 feed 拒绝、费率/时段/缓存/媒体/查询选择、显式零和用量别名、seller 绑定、中国节假日与调休边界。只有明确执行远端拉取时才需要网络；适配器可用性、供应商凭据、操作员覆盖持久化和 IAM 账务属于本包外部集成边界。

## 源码职责与入口

- [`classify.go`](classify.go) 定义 `Route`/`AuthClass`，解析内嵌路由，分类 public/data-plane/LLM/mixed 路径，匹配路径模板并构造固定公共响应。
- [`contribute.go`](contribute.go) 定义模型/供应商贡献结构、受锁保护的贡献与官方别名注册表、`Contribute`、`ContributeProvider`；私有辅助把贡献行和字段转换为目录 JSON。
- [`cost.go`](cost.go) 提供仅 token 的兼容入口 `Cost` 和小数格式化 `Format`，二者使用当前目录 token 价格。
- [`cost_at.go`](cost_at.go) 定义归一化 `Usage`、`Charge`、`AppliedRate`、`PriceSnapshot`，负责费率解码、平铺字段转换、供应商用量归一化、候选模型解析、variant/window 选择、费用计算与快照序列化。
- [`credential_providers.go`](credential_providers.go) 解析 `provider_fields.json`，合并凭据与价格供应商元数据，深拷贝追加项，去重，补 API base/DeepSeek 默认值并返回排序后的 `CredentialProviders`。
- [`embed.go`](embed.go) 内嵌 `routes.json`、自动路由预设、价格数据和凭据字段；`Embedded` 校验 JSON 并统一启动日志。
- [`feed.go`](feed.go) 定义市场 wire 结构和自定义解码，把原始价格 key/单位转换为 `PriceDocument`，执行带上下文 HTTP 拉取，拒绝空文档，并导出 `BuildPriceDocument`、`FetchMarket`、`ReloadFromMarket`。
- [`holiday.go`](holiday.go) 内嵌并解析中国节假日例外，初始化上海时区/降级时区，实现 `IsPeakHour`。
- [`model_cost.go`](model_cost.go) 初始化目录，持有全局锁、live map、供应商索引、载入时间和已知供应商集合，并导出 raw/map/count/token/provider/来源读取 API。
- [`price_write.go`](price_write.go) 维护基线与 live 模型/供应商层、深拷贝辅助、覆盖/贡献重放、原子文档应用及模型/供应商 set/remove/query。
- [`pricedata.go`](pricedata.go) 定义 `PriceDocument`，解码内嵌基线和来源信息，导出供应商元数据并建立基线价格/供应商状态。
- [`rates.go`](rates.go) 定义 `Rate` 协议和私有 feed 字段分类、measure/side/variant/window 推断、数值提取以及报价单位到基础单位换算。
- [`routes.json`](routes.json) 是 `Load`、分类辅助和 `PublicBody` 使用的内嵌路由/公共响应目录；它描述协议路径，不描述运行时适配器是否就绪。
- [`publicdata`](publicdata/readme_cn.md) 包含内嵌价格、中国日历例外、凭据字段及其许可、自动路由预设；子目录 README 说明每种资源的格式和消费者。
