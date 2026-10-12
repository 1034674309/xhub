package contract

import (
	"net/http"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
	dpdiagnostics "github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	"github.com/sunqirui1987/xhub/internal/hooks"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
	"github.com/sunqirui1987/xhub/internal/plugin"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	"github.com/sunqirui1987/xhub/internal/router"
	"github.com/sunqirui1987/xhub/internal/util"
)

// SpendLog 是用量行。聊天循环和 Bypass 都调用它，两边都不自己写数据库。
// 网关实现在 gateway/spend.go。必须先 RememberExchange 和 AnnotateCall，再 RecordSpend，
// 因为 RecordSpend 会按 callID 把暂存的头、正文和备注取走并删掉。
type SpendLog interface {
	// RememberExchange 把这一次调用的请求头、请求体和响应体暂存到 callID 上。
	// 网关只在打开「在日志里保存请求内容」时才真正落库。
	//
	// 参数 callID：本次请求的 x-litellm-call-id。r：入站请求，用来抄方法和头。
	// 参数 reqBody：原始请求字节。respBody：准备写入日志的响应字节。查询没有正文时传 nil。
	// 返回：无。
	// 调用：Serve 的拒绝、缓存命中、流式成功、非流式成功；ServeBypass 的创建和查询。
	// 测试：gateway/prompt_log_test.go 覆盖 promptJSON 的落库形状。数据面夹具里的实现是空的。
	RememberExchange(callID string, r *http.Request, reqBody, respBody []byte)

	// AnnotateCall 记下首字时间、供应商、缓存和部署。RecordSpend 会按 callID 取走这条备注。
	// 参数 callID：与 RememberExchange 相同。note：这一次要写进日志的事实。TTFTMs 为 nil 时该列留空。
	// 返回：无。
	// 调用：Serve 的缓存命中和成功路径；ServeBypass 在转发结束后。
	// 测试：bypass_logic_test.go 断言创建笔记里有供应商、TTFT 和部署 id。
	AnnotateCall(callID string, note CallNote)

	// RecordSpend 写一条用量。它读取并清掉 callID 上的暂存，计算费用，更新响应头里的 cost。
	// 缓存命中或状态码大于等于 400 时，token 仍保留，扣费金额强制为 0。
	//
	// 参数 w：写 x-litellm-response-cost 等头。p：调用方，用来记用户、团队和密钥。
	// 参数 callID：取暂存。alias：对外模型名。op：call_type，聊天是 chat，Bypass 是「类型:动作」。
	// 参数 usage：prompt_tokens 和 completion_tokens。nil 当作空用量，费用为 0。
	// 参数 start：请求开始时间，用来算耗时。cacheHit：为 true 时不按 token 扣费。
	// 参数 status：写进日志的 HTTP 状态。depID：api_base|model，用来找部署上的单价。
	// 返回：无。有 Redis 时行进队列，否则当场写 PostgreSQL。
	// 调用：Serve、ServeBypass，以及 gateway WriteChatJSON、WriteCacheHit 内部。
	// 测试：prompt_log_test.go TestSpendLogRoundTripReturnsPromptPayload；bypass_logic_test.go 检查 call_type 和只扣一次。
	RecordSpend(w http.ResponseWriter, p *auth.Principal, callID, alias, op string, usage map[string]any, start time.Time, cacheHit bool, status int, depID string)
}

// RequestHost 是适配和透传共同消费的身份、路由、凭据与传输边界。
// 网关实现这些方法；协议模块不依赖具体 Server 或数据库。
type RequestHost interface {
	// RequireLLMPrincipal 解析可以发起推理的身份。失败时响应已经写成 401，并返回 nil。
	// 参数 w：写 401。r：从 Authorization 或会话 cookie 取身份。
	// 返回：调用方。nil 表示未授权。
	// 调用：Serve 开头。测试：failure_log_test.go 的 logHost 固定返回一个会话身份。
	RequireLLMPrincipal(w http.ResponseWriter, r *http.Request) *auth.Principal
	// ResolveRequest 在重试前再解析一次身份，确认密钥没有在这次请求中途失效。
	// 参数 r：同一入站请求。
	// 返回：新的身份。密钥被撤销或过期时返回错误，Serve 中止重试。
	// 调用：Serve 在 num_retries 大于 0 且第一次尝试之后。测试夹具返回固定身份。
	ResolveRequest(r *http.Request) (*auth.Principal, error)
	// EnforceIdentityLimits 检查模型允许名单、预算，以及 RPM 或 TPM。拒绝时响应已写好。
	//
	// 参数 w、path：写 403 或 429，path 决定错误包络。p：调用方。alias：对外模型名。
	// 参数 est：EstimateTokens 给出的上界，用来扣 TPM。
	// 返回：true 表示可以继续。false 表示调用方必须 return。
	// 调用：Serve 在读完模型名之后，以及每次重试之前。测试夹具恒返回 true。
	EnforceIdentityLimits(w http.ResponseWriter, path string, p *auth.Principal, alias string, est int) bool
	// GatewayConfig 返回进程配置。Serve 用它的模型表；路由策略和重试次数改读 RouteSettingsFor。
	// 参数：无。
	// 返回 *config.Config（*config.Config）：当前进程配置，含模型表和路由策略，不会复制。
	// 调用：dataplane/execution/official.go、dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	GatewayConfig() *config.Config
	// RouteSettingsFor 返回这一次请求生效的路由设置：策略、重试次数、超时和冷却阈值。
	//
	// 调用方选中的模板由身份链在预算检查时解析出来，所以这里不遍历、不额外查询。
	// 没有任何一层选中模板时返回平台默认那一份，行为和这个功能存在之前一样。
	//
	// 参数 p（*auth.Principal）：已经解析的调用方，含预算链填入的模板选择。
	// 返回 prefs.RouteSettings（prefs.RouteSettings）：这次请求生效的设置和来源。
	// 调用：dataplane/execution/serve.go
	// 测试：route_settings_test.go
	RouteSettingsFor(p *auth.Principal) prefs.RouteSettings
	// RouteState 返回这一刻的冷却、延迟、用量和正在处理的请求数。没有 Redis 时只有 Busy。
	// 参数：无。
	// 返回 router.State（router.State）：这一刻的冷却、延迟、用量和并发，交给路由器排序。
	// 调用：dataplane/execution/official.go、dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	RouteState() router.State
	// PlanRoute 在选部署之前解析会话，并读取动态部署偏好或固定父响应归属。
	// 参数 r：读会话头。alias：对外模型名。body：读标准 session_id、父响应和稳定 prompt。p：用来隔离租户。
	// 返回：RoutePlan。Pinned 非空时两类执行器验证当前资格后优先复用；Required 只允许父响应原目标。
	// 调用：Serve。测试：gateway/affinity/affinity_test.go。
	PlanRoute(r *http.Request, alias string, body map[string]any, p *auth.Principal) RoutePlan
	// CommitRoute 把这次真正用到的部署记到会话上，供下一轮继续打到同一上游。
	// 参数 plan：PlanRoute 的结果。deploymentID：稳定部署 ID。responseID：响应里的 id，可空。
	// 返回：无。
	// 调用：Serve 在流式或非流式成功之后。测试：affinity_test.go TestConcurrentClaimAndLateCommit。
	CommitRoute(plan RoutePlan, deploymentID, responseID string)
	// AttachCredential 按部署上的凭据名补上 api_key 和 api_base。没有凭据名时原样返回。
	// 参数 dep：路由选出的一行模型。
	// 返回：填好密钥的副本。凭证库不可用或凭证无效时返回错误，Serve 跳过该部署。
	// 调用：Serve 每个部署一次；ServeBypass 在创建和查询时。测试夹具原样返回 dep。
	AttachCredential(dep config.ModelEntry) (config.ModelEntry, error)
	// HTTPClient 返回访问上游的客户端。超时来自路由配置。
	// 参数：无。
	// 返回 *http.Client（*http.Client）：访问上游的 HTTP 客户端，超时来自路由设置。
	// 调用：dataplane/execution/official.go、dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	HTTPClient() *http.Client
	// NoteFailure 记下这个部署的一次失败，供下次路由决定是否冷却。
	//
	// 阈值跟着这次请求的设置走，所以调用方把那设置一并传进来：一个租户把
	// allowed_fails 调低之后，它的失败按它自己的阈值计数，而不是按全局的值。
	//
	// 参数 id：部署 id。settings：这次请求生效的路由设置。返回：无。
	// 调用：Serve 在 5xx 或 429 之后。实现转到 dataplane.RecordFailure。
	// 测试：无直接单测
	NoteFailure(id string, settings prefs.RouteSettings)
}

// Adapted 是适配执行器消费的能力；参数方法由网关实现，响应缓存和护栏只属于适配链。
// 调用 execution.Serve；原生任务归属不属于本接口。
type Adapted interface {
	RequestHost

	// GuardrailBlocks 在聊天发往上游之前跑护栏。拦截时响应已经写好。
	// 参数 callID：本次调用 id。body：请求 JSON。
	// 返回：blocked 为 true 时 msg 是给调用方的说明。未命中时 blocked 为 false，msg 为空。
	// 调用：Serve，且只在 op 为 chat 或空时。测试：gateway/guardrail_block_test.go。
	GuardrailBlocks(callID string, body map[string]any) (bool, string)

	// ResponseCache 返回进程内和 Redis 的响应缓存。缓存关闭时 Get 仍然可调用，只是没有命中。
	// 参数：无。
	// 返回 *util.DualCache（*util.DualCache）：响应缓存。未命中时 Get 返回空。
	// 调用：dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	ResponseCache() *util.DualCache

	// HookEngine 返回并发钩子。Begin 占用一个名额，返回的函数在请求结束时释放。 超过并行上限时 Begin 的结果由网关侧写成 429，这里只负责调用 Begin。
	// 参数：无。
	// 返回 *hooks.Engine（*hooks.Engine）：并发钩子。Begin 占用一个名额，返回的函数在请求结束时释放。
	// 调用：dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	HookEngine() *hooks.Engine

	// Extensions 返回推理前的扩展注册表。没有注册项时 Run 不拒绝，调用继续走向上游。
	// 参数：无。
	// 返回 *plugin.Registry（*plugin.Registry）：推理前扩展。没有注册项时不会拒绝调用。
	// 调用：dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	Extensions() *plugin.Registry

	// IncBusy 把该部署正在处理的请求数加一。id 是 api_base|model。RouteState 的 Busy 读这个计数。
	// 参数 id（string）：部署 id，形状是 api_base|model。空串时计数没有对应的部署。
	// 返回：无。该部署的在途请求数加一。RouteState 的 Busy 读这个数。
	// 调用：dataplane/execution/serve.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	IncBusy(id string)

	// DecBusy 把该部署正在处理的请求数减一。必须和 IncBusy 成对，包含失败路径。
	// 参数 id：与 IncBusy 相同。返回：无。
	// 调用：Serve 在请求结束时，包括失败路径，必须和 IncBusy 成对。
	// 测试：无直接单测
	DecBusy(id string)

	// NoteLatency 记下从这个请求开始到响应头的毫秒数。
	// 参数 id：部署 id。ms：从请求开始到响应头的毫秒数。返回：无。
	// 调用：Serve 在收到上游响应头之后。
	// 测试：无直接单测
	NoteLatency(id string, ms float64)

	// SetChatHeaders 给聊天响应写上调用 id、对外模型名和本次上游地址。
	// 参数 w：响应。p：调用方，用来决定哪些头可以暴露。alias：对外模型名。apiBase：本次上游根地址。
	// 返回：无。
	// 调用：Serve 非流式成功，以及流式开始写出之前。
	// 测试：无直接单测
	SetChatHeaders(w http.ResponseWriter, p *auth.Principal, alias, apiBase string)

	// WriteCacheHit 把缓存正文写回调用方，并记一条扣费为 0 的用量。token 数仍保留。
	//
	// 参数 w、p、callID、alias：与 RecordSpend 相同。ck：缓存键。op：操作名。
	// 参数 hit：缓存的响应字节。start：请求开始时间。
	// 返回：无。响应已写完。
	// 调用：Serve 在 ResponseCache.Get 命中时。测试：TestServeLogsCacheHitAndStreamMetrics。
	WriteCacheHit(w http.ResponseWriter, p *auth.Principal, callID, alias, ck, op string, hit []byte, start time.Time)

	// WriteChatJSON 把上游 JSON 写给调用方并记用量。非成功状态不会被当成成功正文放进缓存。
	//
	// 参数 provider：custom_llm_provider 或从模型 id 拆出的供应商。respBody：上游正文。
	// 参数 status：上游状态码。depID：部署 id。其余参数与 WriteCacheHit 相同。
	// 返回：无。
	// 调用：Serve 的非流式成功路径。流式路径自己写字节并直接 RecordSpend。
	// 测试：failure_log_test.go
	WriteChatJSON(w http.ResponseWriter, p *auth.Principal, callID, alias, ck, op, provider string, respBody []byte, status int, start time.Time, depID string)

	SpendLog
}

// Bypass 是官方接口转发。它不把正文编成聊天补全，不读响应缓存，不跑护栏。
// 网关在 bypass.go 的 serveBypass 里，路径已经匹配到端点类型之后调用 ServeBypass。
// 公共能力通过 RequestHost 复用，本接口只增加原生任务所需的模型查找和归属。
type Bypass interface {
	RequestHost

	// Models 返回当前进程里的全部部署。Bypass 再用端点类型过滤。
	// 参数：无。
	// 参数：无。
	// 调用：dataplane/runtime/live.go、dataplane/execution/official.go、gateway/bypass.go、gateway/wire.go
	// 测试：bypass_logic_test.go、failure_log_test.go
	Models() []config.ModelEntry

	// FindDeployment 按钉住的部署 id 找回那条模型配置。进程里已经没有这行时返回假。
	// 参数 id：PinOfficial 存下来的值。
	// 返回：找到时 ok 为 true。进程里已经没有这行时 ok 为 false，查询回 404。
	// 调用：serveBypassFollow。测试：bypass_logic_test.go。
	FindDeployment(id string) (config.ModelEntry, bool)

	// PinOfficial 把官方任务 id 钉到创建时的部署，有效期七天。
	// 参数 taskID：从创建响应按端点类型的 task_id 字段取出。deploymentID：api_base|model。
	// 返回：无。
	// 调用：serveBypassCreate 在上游返回任务 id 之后。测试：gateway/affinity_pin_test.go 固定 7 天。
	PinOfficial(taskID, deploymentID string)

	// OfficialDeployment 按任务 id 取回创建时的部署。没有钉时返回空串。
	// 参数 taskID：路径参数或查询参数里的任务 id。
	// 返回：部署 id，或空串。
	// 调用：serveBypassFollow。测试：bypass_logic_test.go 清掉钉之后再查得到 404。
	OfficialDeployment(taskID string) string
	// PinOfficialContext 与任务归属一起保存创建时的计费事实。
	// 参数 taskID（string）：任务标识；facts（provider.TaskContext）：请求中的计费事实。
	// 返回：无。
	// 调用：官方任务创建成功后。
	// 测试：official_settlement_test.go。
	PinOfficialContext(taskID string, facts providercontract.TaskContext)
	// OfficialContext 读取任务创建时记录的计费事实。
	// 参数 taskID（string）：任务标识。
	// 返回 provider.TaskContext：已保存的计费事实；缺失时为零值。
	// 调用：官方任务查询结算时。
	// 测试：official_settlement_test.go。
	OfficialContext(taskID string) providercontract.TaskContext

	SpendLog
}

// RoutePlan 是选部署前生成的请求私有计划，普通偏好可动态重选，父响应归属必须保持。
// Alias 是对外模型名。SessionID 写入用量行，把同一会话的请求收成一组。
// Caller 是密钥哈希或用户 id，避免不同租户共用一根钉。
// 适配和原生入口均重新验证候选，成功后才比较更新偏好；不是持久化强绑定。
type RoutePlan struct {
	// SessionID 的识别来源，不是供应商来源或路由策略。由 PlanRoute 生成，经 CallNote 写入日志。
	// explicit：标准 X-Session-Id/正文 session_id；parent：授权父响应继承；
	// cache_key：prompt_cache_key 哈希；prompt：稳定首轮内容指纹；generated：随机 UUID。
	// cache_key/prompt 在日志中标为推断；不参与权限判断。空值表示未记录来源；本字段通过日志载体序列化。
	SessionSource string
	// 本轮读取或实际执行的部署、模型池及回退路径；执行前重新验证当前配置中的资格。
	Target RouteTarget
	// 无偏好时原子申请候选并返回并发赢家；nil 表示不协调。只协调缓存，不锁上游执行。
	Claim func(RouteTarget) RouteTarget
	// 成功后比较更新本轮读取的版本并续期；nil 表示不写偏好，迟到结果不能覆盖已变化的记录。
	Save func(RouteTarget)
	// 调用方、请求模型和 responses 入口隔离的一小时临时上下文；执行器追加本轮输入和成功结果，不是原厂长期 store。
	History []llmdialogue.Turn
	// 能力目录中的入口标识，作为缓存范围隔离不同协议；不是供应商地址。
	Endpoint string
	// 父响应已确定原部署时为 true；只允许该部署，不能进入普通跨目标回退。
	Required bool
	// 身份冲突、非法信号或父响应/上下文不可用等计划错误，执行器必须在上游调用前拒绝。
	Err error
	// 客户端请求的公开模型或路由组；回退时保持原值，用作动态偏好范围。
	Alias string
	// 明确、继承、推断或生成的会话标识，用于响应头和授权范围内日志归组；不是响应 ID，不授予历史读取权限。
	SessionID string
	// 鉴权生成的 key:<hash> 或 user:<id>，用于隔离调用方；客户端不能指定，空值不建立偏好。
	Caller string
	// 候选稳定 deployment_id，执行器从当前目录转换为运行 ID；普通请求优先复用，Required 请求只允许该目标。
	Pinned string
}

// RouteStep 是实际回退路径的一条边，每轮按当前模板验证。
type RouteStep struct {
	// 回退失败源的公开模型或组；没有显式组链时可以是实际成员名。
	From string `json:"from"`
	// 对应错误类别回退列表中的目标公开模型或组，发送前仍检查权限、能力与预算。
	To string `json:"to"`
	// 回退类别：general（429/5xx/可重放网络错误）、context（上下文超限）、content（上游内容拒绝）。
	Kind string `json:"kind"`
}

// RouteTarget 保存动态偏好；Pool 是展开的模型或组，Revision 防止比较更新出现 ABA。
type RouteTarget struct {
	// 部署目录的稳定 ID，地址或定价变化不改变它；每轮从当前目录恢复执行配置，删除后失效。
	DeploymentID string `json:"deployment_id"`
	// 实际部署所属公开模型；组调用和回退时可以不同于客户端请求别名，上游型号另行保存。
	Model string `json:"model"`
	// 实际选中部署的公开模型池或显式组，决定当前策略、权重及回退源。
	Pool string `json:"pool"`
	// 原请求别名到实际模型池的有序回退边，每轮按当前模板验证；直接命中时为空，不能作为授权。
	Path []RouteStep `json:"path,omitempty"`
	// 每次缓存写入生成的随机版本，避免比较更新的 ABA 问题；不是时间戳或排序编号。
	Revision string `json:"revision"`
}

// CallNote 是写进用量行的事实。由 AnnotateCall 暂存，RecordSpend 取走。
// TTFTMs 是首字节或同步整次的毫秒数，nil 表示这次没量到。
// Provider 是 custom_llm_provider，或 Bypass 的供应商前缀。
// CacheKey 是响应缓存键。CacheHit 为 true 时扣费金额为 0。
// SessionID 与 RoutePlan.SessionID 相同。部署身份由 RecordSpend 的 depID 参数传递。
type CallNote struct {
	// 是否实际流式执行，供性能统计区分首字延迟与非流完整响应耗时。
	Streamed bool
	// 传输观察器产生的脱敏响应头及原始用量诊断；nil 表示没有上游观察事实。
	Upstream *dpdiagnostics.UpstreamDiagnostics
	// 异步任务创建时的原请求 ID，轮询仍使用自己的 callID，供更新原任务日志。
	TaskRequestID string
	// 上游任务生命周期状态，用于更新创建日志，不决定同步重试策略。
	TaskStatus string
	// 是否为异步任务创建事件，创建成功时实际费用可能尚未确定。
	TaskInitial bool
	// 查询取得成功实测结果时为 true，通知账单原子结算原任务并防止重复收费。
	TaskSettlement bool
	// 指定账单落在原任务请求上，不改变暂存诊断使用的 callID。
	SettlementID string
	// 查询不计入同步路由 TPM 时为 true，避免反复轮询放大 token 用量。
	SkipRouteUsage bool
	// 流请求的首字节或非流请求的完整响应耗时，单位毫秒；nil 表示没有测量。
	TTFTMs *int
	// 执行供应商/适配器标识，区别于具体供应商连接 ID。
	Provider string
	// 网关响应缓存键，与会话指纹、原生 prompt_cache_key 及偏好缓存键相互独立。
	CacheKey string
	// 是否返回网关响应缓存；命中仍记录用量但推理费用为 0，不提交上游成功偏好。
	CacheHit bool
	// 明确、继承、推断或生成的会话标识，用于响应头和授权范围内日志归组；不是响应 ID，不授予历史读取权限。
	SessionID string
	// SessionID 的识别来源，不是供应商来源或路由策略。由 PlanRoute 生成，经 CallNote 写入日志。
	// explicit：标准 X-Session-Id/正文 session_id；parent：授权父响应继承；
	// cache_key：prompt_cache_key 哈希；prompt：稳定首轮内容指纹；generated：随机 UUID。
	// cache_key/prompt 在日志中标为推断；不参与权限判断。空值表示未记录来源；本字段通过日志载体序列化。
	SessionSource string
}
