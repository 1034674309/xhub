package regression

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// encryptedMessagesContent 还原客户端从 JSON/SSE 取得并原样回传的 Messages 内容块。
// 参数为测试上下文、真实 HTTP 回复及流开关，返回含完整 signature/data/input 的数组；只解析响应，所有后台数据由 harness 清理。
func encryptedMessagesContent(t *testing.T, reply reply, stream bool) []any {
	t.Helper()
	if !stream {
		return reply.json()["content"].([]any)
	}
	blocks := []any{}
	for _, line := range strings.Split(reply.text(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		index, _ := event["index"].(float64)
		if event["type"] == "content_block_start" {
			blocks = append(blocks, event["content_block"])
		} else if event["type"] == "content_block_delta" {
			block := blocks[int(index)].(map[string]any)
			delta := event["delta"].(map[string]any)
			switch delta["type"] {
			case "thinking_delta":
				block["thinking"] = stringField(block, "thinking") + stringField(delta, "thinking")
			case "signature_delta":
				block["signature"] = stringField(block, "signature") + stringField(delta, "signature")
			case "text_delta":
				block["text"] = stringField(block, "text") + stringField(delta, "text")
			case "input_json_delta":
				var input map[string]any
				if err := json.Unmarshal([]byte(stringField(delta, "partial_json")), &input); err != nil {
					t.Fatal(err)
				}
				block["input"] = input
			}
		}
	}
	return blocks
}

// TestMessagesEncryptedReasoningToolRoundTrip 验证真实路由将有/无摘要的 Responses 密文封装并原样续接工具结果。
// 参数 t 为上下文；本地上游检查路径、密文和 ID 省略，JSON/SSE 均验证记账，未归属会话和非法 Key 必须外发前拒绝；harness 清理数据与服务。
func TestMessagesEncryptedReasoningToolRoundTrip(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, summary := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%v/summary=%v", stream, summary), func(t *testing.T) {
				var count atomic.Int32
				up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if r.URL.Path != "/v1/responses" || json.NewDecoder(r.Body).Decode(&body) != nil {
						t.Error("Messages 密文没有回到 Responses 上游")
						w.WriteHeader(400)
						return
					}
					count.Add(1)
					second := strings.Contains(string(mustJSON(body)), "file-value")
					if second {
						found := false
						for _, value := range body["input"].([]any) {
							item := value.(map[string]any)
							if item["type"] == "reasoning" && item["encrypted_content"] == "test-cipher" {
								found = true
								if item["id"] != nil {
									t.Error("封装重建的推理不应伪造上游 ID")
								}
							}
						}
						if !found {
							t.Error("工具续轮丢失原始密文")
						}
					}
					var call map[string]any
					if !second {
						call = map[string]any{"type": "function_call", "call_id": "call_1", "name": "read", "arguments": "{}"}
					}
					response := nativeAgentResponse(fmt.Sprintf("r_%d", count.Load()), "Done", true, call)
					if !summary {
						response["output"].([]any)[0].(map[string]any)["summary"] = []any{}
					}
					writeAgentResponse(w, response, body["stream"] == true)
				}))
				t.Cleanup(up.Close)
				dep := chatDeployment("messages-encrypted-roundtrip")
				dep.LiteLLMParams["api_base"] = up.URL + "/v1"
				dep.ModelInfo["transport"], dep.ModelInfo["endpoint_types"] = "bypass_openai_responses", []string{"messages"}
				dep.ModelInfo["pricing_source"] = "manual"
				h := newHarness(t)
				admin := h.adminSession()
				// 与控制台一致通过管理 API 创建稳定部署，固定连接归属不能依赖无 ID 的静态夹具。
				saved := h.ok("POST", "/model/new", admin, map[string]any{"model_name": dep.ModelName, "litellm_params": dep.LiteLLMParams, "model_info": dep.ModelInfo}).json()
				id := saved["model_info"].(map[string]any)["id"].(string)
				owner := h.openScope(t, admin, "encrypted-messages")
				headers := map[string]string{"X-Session-Id": "encrypted-roundtrip"}
				body := map[string]any{"model": dep.ModelName, "stream": stream, "max_tokens": 256,
					"messages": []any{map[string]any{"role": "user", "content": "Read the file."}},
					"tools":    []any{map[string]any{"name": "read", "input_schema": map[string]any{"type": "object"}}}}
				first := h.doHeaders("POST", "/v1/messages", owner.key, body, headers)
				if first.status != 200 {
					t.Fatal(first.describe())
				}
				content := encryptedMessagesContent(t, first, stream)
				body["messages"] = []any{map[string]any{"role": "user", "content": "Read the file."},
					map[string]any{"role": "assistant", "content": content},
					map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call_1", "content": "file-value"}}}}
				second := h.doHeaders("POST", "/v1/messages", owner.key, body, headers)
				if second.status != 200 || !strings.Contains(second.text(), "Done") || (stream && !strings.Contains(second.text(), "message_stop")) {
					t.Fatalf("带密文工具续轮失败: %s", second.describe())
				}
				before := count.Load()
				if rejected := h.doHeaders("POST", "/v1/messages", owner.key, body, map[string]string{"X-Session-Id": "not-owned"}); rejected.status != 400 {
					t.Fatalf("未归属会话没有拒绝: %s", rejected.describe())
				}
				if rejected := h.doHeaders("POST", "/v1/messages", "sk-invalid", body, headers); rejected.status != 401 {
					t.Fatalf("无效 Key 没有拒绝: %s", rejected.describe())
				}
				if count.Load() != before {
					t.Fatal("失败鉴权或未归属密文仍访问上游")
				}
				h.flushSpend()
				for _, response := range []reply{first, second} {
					bill := h.ok("GET", "/spend/logs/ui/"+response.header("x-litellm-call-id"), admin, nil).json()
					cost, _ := floatField(bill, "spend")
					if bill["status"] != "success" || bill["api_key"] != owner.keyID || !nearlyEqual(cost, 8*testInputRate+2*testOutputRate) {
						t.Fatalf("密文工具往返的日志或费用错误: %v", bill)
					}
				}
				// 保留公开模型和部署 ID 更新 Key 后，旧封装必须在外发前拒绝，不能转交到新账户。
				dep.LiteLLMParams["api_key"] = "sk-changed-account"
				h.ok("PATCH", "/model/"+id+"/update", admin, map[string]any{"litellm_params": dep.LiteLLMParams})
				if rejected := h.doHeaders("POST", "/v1/messages", owner.key, body, headers); rejected.status != 409 || count.Load() != before {
					t.Fatalf("连接变更后旧密文仍被外发: %s", rejected.describe())
				}
			})
		}
	}
}
