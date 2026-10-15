package routing

import (
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	contract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
)

// TestBindConnection 验证固定续接绑定已解析的账户、地址、型号和传输，而普通会话可重新选路。
// 参数 t 为上下文；前置内存部署，覆盖正常、缺失旧指纹、连接变更及不能编码的输入。
// 哈希不得暴露测试凭据，定价调整不使连接失效；无供应商调用或持久数据，测试结束回收内存。
func TestBindConnection(t *testing.T) {
	base := config.ModelEntry{ModelName: "public", LiteLLMParams: map[string]any{"model": "upstream", "api_base": "https://a.example/v1", "api_key": "private-account-a", "litellm_credential_name": "connection-a", "custom_llm_provider": "openai"}, ModelInfo: map[string]any{"id": "stable", "transport": "bypass_openai_responses"}}
	first := contract.RoutePlan{}
	if !BindConnection(&first, base) || len(first.Target.ConnectionID) != 64 || strings.Contains(first.Target.ConnectionID, "private") {
		t.Fatal("首轮未保存安全的连接指纹")
	}
	for _, tc := range []struct {
		key   string
		value any
		valid bool
	}{
		{"api_base", "https://b.example/v1", false}, {"api_key", "private-account-b", false},
		{"model", "another-upstream", false}, {"litellm_credential_name", "connection-b", false},
		{"custom_llm_provider", "another-provider", false}, {"headers", map[string]any{"X-Account": "b"}, false},
		{"input_cost_per_token", 0.02, true}, {"timeout", 10, true},
		{"headers", func() {}, false},
	} {
		t.Run(tc.key, func(t *testing.T) {
			dep := base
			dep.LiteLLMParams = map[string]any{}
			for k, v := range base.LiteLLMParams {
				dep.LiteLLMParams[k] = v
			}
			dep.LiteLLMParams[tc.key] = tc.value
			plan := contract.RoutePlan{Required: true, Target: first.Target}
			if BindConnection(&plan, dep) != tc.valid || plan.Target.ConnectionID != first.Target.ConnectionID {
				t.Fatal("固定续接未保留创建时的连接归属")
			}
		})
	}
	plan := contract.RoutePlan{Required: true, Target: first.Target}
	if !BindConnection(&plan, base) {
		t.Fatal("原连接被错误拒绝")
	}
	changed := base
	changed.ModelInfo = map[string]any{"id": "stable", "transport": "bypass_openai_chat"}
	if BindConnection(&plan, changed) {
		t.Fatal("传输变化后仍可续接")
	}
	plan.Required = false
	if !BindConnection(&plan, changed) || plan.Target.ConnectionID == first.Target.ConnectionID {
		t.Fatal("普通偏好未接受新连接")
	}
	missing := contract.RoutePlan{Required: true}
	if BindConnection(&missing, base) || BindConnection(nil, base) {
		t.Fatal("缺少归属的固定计划被放行")
	}
}
