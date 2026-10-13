# 路由模板编译与部署调度

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

请求先解析身份范围对应的模板，Compile 并检查 Err，再用候选目录和本次运行状态调用 Schedule。按返回顺序执行，实际成功目标交给 affinity 和结算链。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| SelectRouteTemplate / Compile | 选择并校验单请求路由快照 |
| Schedule(pool, alias, strategy, state, pin) | 返回有序部署候选 |
| DeploymentID / CooldownID | 区分稳定部署与运行冷却身份 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	models := []config.ModelEntry{{ModelName: "demo-model", LiteLLMParams: map[string]any{"model": "upstream-model"}}}
	pool := router.Schedule(models, "demo-model", "random", router.State{}, "")
	fmt.Println(len(pool), pool[0].ModelName)
}
```

输出 1 demo-model；只演示单候选调度。生产调用之前仍需完成能力、授权、预算和模板校验。

### 错误、状态与副作用

pin 不能绕过停用、冷却或零权重；组中的真实模型身份仍需授权和计费。Compile 的错误存在结果 Err 中，不能忽略。

## 说明

本包把持久化路由配置和当前模型目录编译成不可变的单请求 RouteSettings 快照，再生成有序部署尝试列表。它负责模型匹配、显式组、相对流量权重、重试时间值、三类回退、身份范围继承、冷却过滤、粘性优先和策略算法；不注册 HTTP、不鉴权、不调用供应商、不记录指标，也不持久化配置。

精确公开模型先于通配模式。通配规则按具体程度排序，并用捕获值改写上游模型。禁用部署会被排除。除非调用方自行保存，函数只返回新切片、映射、文档或只读结果。

### 依赖与测试

本包使用 config 模型、catalog 价格解析、IAM 路由模板、llm 通配辅助和日志。测试覆盖权重、通配、组、回退图、模板编译/校验、清理、范围解析、策略、粘性、冷却身份及当前时段价格。

## 对外接口

本包不注册 handler。gateway identity 在 /route_template/... 提供模板管理，在 /routing/groups 提供组，在 /router/settings 提供设置；这些 handler 调用本包 API。dataplane 使用编译后的 RouteSettings 与 Schedule。

Compile(selected, store, models) 读取模型默认值与模板显式配置，校验引用，并通过 RouteSettings.Err 返回错误，不执行写入。iam.SelectRouteTemplate 按 key、team、organization 选择首个绑定；所选文档不存在时使用内置模板。内置设置为一次尝试、60 秒、阈值三、无冷却。

解析/校验 API 覆盖 FallbackPolicy、Group、Policy、默认权重、模板、目录、策略和部署归属。RouteSettings.ForModel 按精确模型规则、组、模型默认值的优先级解析；Candidates 只展开显式组名；Schedule 应用权重和可选 pin。Retries、TimeoutSeconds、AllowedFails、CooldownSeconds 返回已校验值。All、Available、Order、Pick、DeploymentID、CooldownID 执行调度。CleanAllocations 与 CleanTemplateWeights 深度清理悬空引用。

## 内部核心实现

匹配先找精确 alias；不存在时按字面长度与复杂度排序通配模式，编译安全正则，并把捕获值替换到上游模型。组保留成员真实模型身份，供鉴权和计费。DeploymentID 依次取 deployment_id、model_info.id。CooldownID 组合 api_base、模型和稳定身份，稳定身份依次取 pricing_id、deployment_id、model_info.id，并加入 credential name 隔离账号。

traffic-split 按正的有限相对权重独立抽样；未列部署默认权重一，明确零和冷却项排除。random 与 simple-shuffle 均匀随机。least-busy、cost-based-routing、latency-based-routing、usage-based-routing、tag-based-routing 比较运行状态或配置后备值。成本策略优先当前有效费率表，再用平价；未定价或非法值视为正无穷。pin 只有仍匹配、健康且权重为正才置顶。

模板解析拒绝未知字段、重复模型规则/组、非法重试数、非分流策略权重、组重叠、悬空部署 ID、未知回退节点、过长/重复目标，以及跨三类回退的环。Compile 同时校验局部和合并全局图。清理逻辑深拷贝 JSON，删除悬空分配；零或单部署不保留权重；完整剩余配置若全零则恢复默认均等行为。

## 源码职责与入口

- [compile.go](compile.go)：ConfigReader，合并并校验默认、组和回退。
- [fallback.go](fallback.go)：FallbackPolicy 严格解析、分类目标和跨类别图校验。
- [groups.go](groups.go)：Group/GroupArgs、名称/成员/权重校验和候选展开。
- [policy.go](policy.go)：Allocation、Policy、DefaultWeights、严格解析、份额与部署归属。
- [router.go](router.go)：State、精确/通配匹配、全部策略、加权抽样、运行身份、价格比较和策略校验。
- [schedule.go](schedule.go)：健康/权重过滤和会话目标置顶。
- [settings.go](settings.go)：RouteSettings、范围解析、builtin、模型优先级、重试值和模板字段校验。
- [template.go](template.go)：组与三类回退提取及实时目录校验。
- [weight_cleanup.go](weight_cleanup.go)：悬空分配清理和深拷贝模板修复。

验证命令：go test ./internal/router -count=1。Redis 新鲜度、实时并发、供应商健康和真实价格源正确性属于外部边界。
