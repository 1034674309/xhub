package regression

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// portableCompleted 从真实 JSON/SSE 获取最终响应，参数为测试上下文及 HTTP 回复，返回对象。
// 供迁移回归使用；缺少完成事件时失败，不生成响应或修改历史，无外部数据需清理。
func portableCompleted(t *testing.T, reply reply, stream bool) map[string]any {
	t.Helper()
	if !stream {
		return reply.json()
	}
	for _, line := range strings.Split(reply.text(), "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event) == nil && event["type"] == "response.completed" {
			return event["response"].(map[string]any)
		}
	}
	t.Fatalf("缺少 Responses 完成事件: %s", reply.describe())
	return nil
}

// TestPortableResponsesModelSwitch 验证外部密文历史进入网关，在三类模型间往返后仍可回外部端点。
// 参数 t 为上下文；前置真实网关/数据库/个人密钥和严格本地供应商，验证 JSON/SSE、工具关联、游标隔离、错误、计费。
// 不改用户数据或客户端历史；harness 删除专属 schema，Cleanup 关闭上游，不依赖外部密钥。
func TestPortableResponsesModelSwitch(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		calls.Add(1)
		encoded, _ := json.Marshal(body)
		if body["model"] != "outside" && strings.Contains(string(encoded), "foreign-cipher") {
			t.Error("旧供应商密文被转发到新连接")
		}
		if strings.Contains(r.URL.Path, "chat/completions") {
			writeChatAgentResponse(w, "chat_public", "Done", "", "", body["stream"] == true)
			return
		}
		if body["model"] == "outside" && strings.Contains(string(encoded), "test-cipher") {
			t.Error("XHub 输出污染了切回外部的历史")
		}
		items, _ := body["input"].([]any)
		toolDone := false
		for _, v := range items {
			item, _ := v.(map[string]any)
			if item["type"] == "function_call_output" {
				toolDone = true
				if item["call_id"] != "call_1" || item["output"] != "file-value" {
					t.Error("工具结果关联丢失")
				}
			}
		}
		var call map[string]any
		if body["tools"] != nil && !toolDone {
			call = map[string]any{"id": "fc_1", "type": "function_call", "call_id": "call_1", "name": "read", "arguments": "{}"}
		}
		writeAgentResponse(w, nativeAgentResponse(fmt.Sprintf("resp_portable_%d", calls.Load()), "Done", true, call), body["stream"] == true)
	}))
	t.Cleanup(upstream.Close)
	h := newHarness(t)
	admin := h.adminSession()
	owner := h.openScope(t, admin, "portable-owner")
	for _, name := range []string{"gpt-test", "grok-test", "deepseek-test"} {
		dep := chatDeployment(name)
		dep.LiteLLMParams["api_base"] = upstream.URL + "/v1"
		transport := "bypass_openai_responses"
		if name == "deepseek-test" {
			transport = "bypass_openai_chat"
		}
		dep.ModelInfo = map[string]any{"transport": transport, "pricing_source": "manual", "endpoint_types": []string{"responses"}}
		h.ok("POST", "/model/new", admin, map[string]any{"model_name": name, "litellm_params": dep.LiteLLMParams, "model_info": dep.ModelInfo})
	}
	for _, stream := range []bool{false, true} {
		input := []any{map[string]any{"role": "user", "content": "用户原文"}, map[string]any{"type": "reasoning", "id": "rs_foreign", "encrypted_content": "foreign-cipher", "summary": []any{map[string]any{"type": "summary_text", "text": "Public plan"}}}, map[string]any{"role": "assistant", "content": "Earlier answer"}, map[string]any{"role": "user", "content": "Continue"}}
		original, _ := json.Marshal(input)
		for _, model := range []string{"gpt-test", "grok-test", "deepseek-test", "gpt-test"} {
			answer := h.do("POST", "/v1/responses", owner.key, map[string]any{"model": model, "input": input, "store": false, "stream": stream})
			if answer.status != 200 || strings.Contains(answer.text(), "encrypted_content") || answer.headers.Get("X-XHub-History-Mode") != "portable" {
				t.Fatalf("%s stream=%v 历史迁移失败: %s", model, stream, answer.describe())
			}
			result := portableCompleted(t, answer, stream)
			for _, value := range result["output"].([]any) {
				item, _ := value.(map[string]any)
				if item["type"] == "reasoning" && (item["id"] != nil || item["status"] != nil) {
					t.Fatalf("公开推理仍含来源 ID/状态: %v", item)
				}
			}
			input = append(input, result["output"].([]any)...)
			input = append(input, map[string]any{"role": "user", "content": "Continue"})
			h.flushSpend()
			bill := h.ok("GET", "/spend/logs/ui/"+answer.headers.Get("x-litellm-call-id"), admin, nil).json()
			if bill["api_key"] != owner.keyID || bill["status"] != "success" {
				t.Fatalf("迁移请求记账归属错误: %v", bill)
			}
		}
		// 切回外部时由客户端携带原来的外部密文；仅验证 XHub 没有向历史注入它自己的新密文。
		raw, _ := json.Marshal(map[string]any{"model": "outside", "input": input})
		response, err := http.Post(upstream.URL+"/v1/responses", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("切回外部失败")
		}
		if !bytes.Contains(original, []byte("foreign-cipher")) {
			t.Fatal("原客户端历史被修改")
		}
	}
	first := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": "gpt-test", "input": "Read the file", "tools": []any{map[string]any{"type": "function", "name": "read", "parameters": map[string]any{"type": "object"}}}}).json()
	next := h.do("POST", "/v1/responses", owner.key, map[string]any{"model": "gpt-test", "previous_response_id": first["id"], "input": []any{map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "file-value"}}})
	if next.status != 200 {
		t.Fatalf("内部完整推理的工具续轮失败: %s", next.describe())
	}
	stranger := h.openScope(t, admin, "portable-other")
	before := calls.Load()
	for _, body := range []map[string]any{{"model": "gpt-test", "previous_response_id": first["id"], "input": "Continue"}, {"model": "gpt-test", "input": 42}} {
		denied := h.do("POST", "/v1/responses", stranger.key, body)
		if denied.status != 400 {
			t.Fatalf("错误请求未拒绝: %s", denied.describe())
		}
	}
	if calls.Load() != before {
		t.Fatal("错误请求仍调用了上游")
	}
}
