package regression

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

// TestLiveTextProtocolMatrix 显式验证已配置供应商的模型在原生 Responses/Chat、JSON/SSE 四种组合中的契约。
// 参数 t 为上下文；只有 XHUB_AGENT_LIVE=1 且提供 LIVE_KEY/BASE 才访问真实供应商并产生费用。
// MODELS 显式限制模型集合（默认三类各一个）；请求要求极短英文回答并限制输出，不测试图片或把模型目录当能力保证。
// 网关、身份、账单使用私有 PostgreSQL schema，退出自动清理；失败记录原始供应商状态但不输出凭据。
func TestLiveTextProtocolMatrix(t *testing.T) {
	if os.Getenv("XHUB_AGENT_LIVE") != "1" {
		t.Skip("live supplier acceptance is opt-in")
	}
	key, base := os.Getenv("XHUB_AGENT_LIVE_KEY"), strings.TrimRight(os.Getenv("XHUB_AGENT_LIVE_BASE"), "/")
	if key == "" || base == "" {
		t.Fatal("LIVE_KEY/BASE must be explicitly configured")
	}
	names := strings.Split(os.Getenv("XHUB_AGENT_LIVE_MODELS"), ",")
	if len(names) == 1 && names[0] == "" {
		names = []string{"gpt-5.6-sol", "grok-4.6", "deepseek-v4-flash"}
	}
	if len(names) > 12 {
		t.Fatal("live acceptance is limited to 12 explicitly selected text models")
	}
	var models []config.ModelEntry
	for _, name := range names {
		for _, protocol := range []string{"responses", "chat"} {
			dep := chatDeployment(name + "-" + protocol)
			dep.LiteLLMParams["model"] = name
			dep.LiteLLMParams["api_base"] = base
			dep.LiteLLMParams["api_key"] = key
			dep.ModelInfo["endpoint_types"] = []string{protocol}
			if protocol == "responses" {
				dep.ModelInfo["transport"] = "bypass_openai_responses"
			}
			models = append(models, dep)
		}
	}
	h := newHarness(t, models...)
	h.live = true
	admin := h.adminSession()
	owner := h.openScope(t, admin, "live-protocol")
	for _, name := range names {
		for _, protocol := range []string{"responses", "chat"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/stream=%v", name, protocol, stream), func(t *testing.T) {
					alias := name + "-" + protocol
					body := map[string]any{"model": alias, "stream": stream, "disable_fallbacks": true}
					path := "/v1/responses"
					if protocol == "responses" {
						body["input"] = "Reply with only OK."
						body["max_output_tokens"] = 256
					} else {
						path = "/v1/chat/completions"
						body["messages"] = []any{map[string]any{"role": "user", "content": "Reply with only OK."}}
						if strings.HasPrefix(name, "gpt-") || name == "codex-auto-review" {
							body["max_completion_tokens"] = 256
						} else {
							body["max_tokens"] = 256
						}
					}
					reply := h.do("POST", path, owner.key, body)
					if reply.status != 200 {
						t.Fatalf("供应商兼容失败: %s", reply.describe())
					}
					if stream {
						terminal := "[DONE]"
						if protocol == "responses" {
							terminal = "response.completed"
						}
						if !strings.Contains(string(reply.body), terminal) && !strings.Contains(string(reply.body), "response.incomplete") {
							t.Fatalf("供应商流未成功完成: %s", reply.describe())
						}
					}
					h.flushSpend()
					bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
					if bill["status"] != "success" || bill["api_key"] != owner.keyID {
						t.Fatalf("供应商成功请求未记账: %v", bill)
					}
					t.Logf("%s %s stream=%v passed", name, protocol, stream)
				})
			}
		}
	}
}
