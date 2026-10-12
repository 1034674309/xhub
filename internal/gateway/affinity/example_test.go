package affinity_test

import (
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/gateway/affinity"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// ExampleState 演示包外调用方如何使用会话偏好、父响应续接及官方任务归属。
// 无参数和返回值；前置独立内存 State、模拟鉴权身份及已验证候选，通过固定输出验证状态往返。
// 不发送真实上游请求、不做真实鉴权，生产执行器须负责这些步骤；测试结束释放内存，无外部数据。
func ExampleState() {
	// State 在网关启动时创建并跨请求复用；每个请求创建 State 会丢失内存偏好。
	state := affinity.New(nil)
	principal := &auth.Principal{UserID: "demo-user"} // 夹具；生产中必须取自鉴权结果。
	alias := "demo-model"
	body := map[string]any{"session_id": "demo-session", "input": "Hello"}
	request := httptest.NewRequest("POST", "/v1/responses", nil)

	plan := state.PlanRoute(request, alias, body, principal)
	if plan.Err != nil {
		panic(plan.Err) // 演示失败即终止；HTTP 执行器应在调用上游前返回对应错误。
	}
	// 仅演示状态 API，假定目录中已验证且符合当前权限、能力及权重的候选只有一个。
	// 生产链通过 SelectAffinity 验证 plan.Target 和 Claim 的赢家，不可直接盲用缓存目标。
	target := dpcontract.RouteTarget{DeploymentID: "demo-deployment", Model: alias, Pool: alias}
	if plan.Claim != nil {
		winner := plan.Claim(target)
		if winner.DeploymentID != target.DeploymentID || winner.Model != alias || winner.Pool != alias || len(winner.Path) != 0 {
			panic("unexpected target outside the demo candidate set")
		}
		target = winner
	}
	// 此处假定上游执行成功；失败尝试不能调用 CommitRoute。
	// 执行器提供实际成功目标及合并输入、输出后的完整历史，再发布响应 ID。
	plan.Target = target
	plan.History = []llmdialogue.Turn{{Role: "user", Text: "Hello"}, {Role: "assistant", Text: "Hi"}}
	state.CommitRoute(plan, target.DeploymentID, "demo-response")

	next := state.PlanRoute(request, alias, body, principal)
	if next.Err != nil {
		panic(next.Err)
	}
	fmt.Println(next.SessionSource, next.Pinned, next.Required)

	// 续接只提供增量输入，继承父会话、固定目标及历史；固定目标失效时应拒绝，不能改投其他部署。
	continuation := state.PlanRoute(request, alias, map[string]any{
		"previous_response_id": "demo-response", "input": "Continue",
	}, principal)
	if continuation.Err != nil {
		panic(continuation.Err)
	}
	fmt.Println(continuation.SessionSource, continuation.Pinned, continuation.Required, len(continuation.History))

	// 任务作用域由调用方隔离，演示值代表用户、协议与任务 ID 的组合，生产使用 officialTaskScope。
	// 任务 API 使用运行部署 ID（与上面的稳定部署 ID 属于不同用途），创建事实先于部署归属保存。
	scope := "demo-user/openai-videos/demo-task"
	state.PinOfficialContext(scope, providercontract.TaskContext{
		Known: true, StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Model: "demo-video-model",
	})
	state.PinOfficial(scope, "demo-provider|demo-video-model")
	fmt.Println(state.OfficialDeployment(scope), state.OfficialContext(scope).Known)

	// Output:
	// explicit demo-deployment false
	// parent demo-deployment true 2
	// demo-provider|demo-video-model true
}
