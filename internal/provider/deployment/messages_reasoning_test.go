package deployment_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
	"github.com/sunqirui1987/xhub/internal/provider/deployment"
)

// TestCertifiedMessagesReasoning 验证管理员认证后的effort映射、候选一致性与原请求不变。
// 前置内存Messages部署；缺省、none和非法effort各走明确路径，cap不被扩大；没有网络或持久资源。
func TestCertifiedMessagesReasoning(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mode, effort any
		want         bool
	}{
		{"unconfigured", nil, "low", false}, {"disabled", "disabled", "low", false}, {"low", "adaptive", "low", true}, {"medium", "adaptive", "medium", true}, {"high", "adaptive", "high", true}, {"max", "adaptive", "max", true}, {"none", "adaptive", "none", true}, {"missing", "adaptive", nil, true}, {"minimal", "adaptive", "minimal", false}, {"xhigh", "adaptive", "xhigh", false}, {"invalid", "adaptive", 42, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dep := config.ModelEntry{ModelName: "test", LiteLLMParams: map[string]any{"model": "actual", "custom_llm_provider": "anthropic"}, ModelInfo: map[string]any{"transport": "bypass_anthropic_messages", "endpoint_types": []string{"responses"}}}
			if tc.mode != nil {
				dep.ModelInfo["messages_reasoning_mode"] = tc.mode
			}
			body := map[string]any{"input": "Hello", "max_output_tokens": 64, "reasoning": map[string]any{"effort": tc.effort, "summary": "auto"}}
			d, err := llmdialogue.ParseDialogue("openai-responses", body)
			if err != nil {
				if !tc.want {
					return
				}
				t.Fatal(err)
			}
			before, _ := json.Marshal(d.Native)
			pool, _ := deployment.Candidates([]config.ModelEntry{dep}, "responses", &d)
			got, _, err := deployment.EncodeDialogue(dep, d)
			if (err == nil) != tc.want || (len(pool) == 1) != tc.want {
				t.Fatalf("候选与编码不一致: %v %v", pool, err)
			}
			after, _ := json.Marshal(d.Native)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("编码修改原始推理设置")
			}
			if !tc.want {
				return
			}
			if got["max_tokens"] != float64(64) {
				t.Fatalf("输出上限被扩大: %v", got)
			}
			if tc.effort == nil {
				if got["thinking"] != nil {
					t.Fatal("缺省注入thinking")
				}
				return
			}
			thinking := got["thinking"].(map[string]any)
			if tc.effort == "none" {
				if thinking["type"] != "disabled" {
					t.Fatal("显式none语义改变")
				}
				return
			}
			if thinking["type"] != "adaptive" || got["output_config"].(map[string]any)["effort"] != tc.effort {
				t.Fatalf("effort映射错误: %v", got)
			}
		})
	}
}

// TestMessagesReasoningConfigurationValidation 验证非法配置和其他协议无法获得未实现的签名桥接。
// 前置内存配置；断言错类型/模式/非Messages传输均拒绝，Chat低推理仍拒绝；无外部资源。
func TestMessagesReasoningConfigurationValidation(t *testing.T) {
	for _, value := range []any{true, 42, "guess", []any{}} {
		if deployment.ValidateMessagesReasoning(config.ModelEntry{ModelInfo: map[string]any{"messages_reasoning_mode": value}}) == nil {
			t.Fatalf("非法模式被接受: %v", value)
		}
	}
	if deployment.ValidateMessagesReasoning(config.ModelEntry{ModelInfo: map[string]any{"messages_reasoning_mode": "adaptive", "transport": "bypass_openai_chat"}}) == nil {
		t.Fatal("非Messages接受adaptive")
	}
	dep := config.ModelEntry{ModelName: "test", LiteLLMParams: map[string]any{"model": "actual", "custom_llm_provider": "anthropic"}, ModelInfo: map[string]any{"messages_reasoning_mode": "adaptive", "transport": "bypass_anthropic_messages", "endpoint_types": []string{"chat"}}}
	d, err := llmdialogue.ParseDialogue("openai-chat", map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Hello"}}, "reasoning_effort": "low"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := deployment.EncodeDialogue(dep, d); err == nil {
		t.Fatal("未实现Chat签名回放却放行")
	}
}
