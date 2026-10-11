package requestlog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPromptJSONKeepsHeadersBodyAndResponse 前置含凭据的请求，验证正文、查询和头脱敏；内存测试无需清理。
func TestPromptJSONKeepsHeadersBodyAndResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-local-master")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "trace-1")
	messages, response, proxy := Encode(req,
		[]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
		[]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"hi"}}]}`),
	)
	var msgs []map[string]any
	if err := json.Unmarshal([]byte(messages), &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0]["content"] != "hello" {
		t.Fatalf("messages %s", messages)
	}
	if !strings.Contains(response, "chatcmpl-1") || !strings.Contains(response, `"hi"`) {
		t.Fatalf("response %s", response)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(proxy), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["method"] != "POST" || doc["url"] != "/v1/chat/completions" {
		t.Fatalf("proxy %s", proxy)
	}
	headers, _ := doc["headers"].(map[string]any)
	if headers["Authorization"] != "***" {
		t.Fatalf("authorization leaked: %v", headers)
	}
	if headers["X-Request-Id"] != "trace-1" || headers["Content-Type"] != "application/json" {
		t.Fatalf("headers %v", headers)
	}
	body, _ := doc["body"].(map[string]any)
	if body["model"] != "gpt-4o-mini" {
		t.Fatalf("body %v", body)
	}
	if strings.Contains(proxy, "sk-local-master") || strings.Contains(messages, "sk-") {
		t.Fatalf("secret leaked: %s", proxy)
	}
	polled := httptest.NewRequest(http.MethodGet, "/api/v1/generate/record-info?taskId=suno-1", nil)
	_, _, polledProxy := Encode(polled, nil, []byte(`{"data":{"taskId":"suno-1"}}`))
	if !strings.Contains(polledProxy, "/api/v1/generate/record-info?taskId=suno-1") {
		t.Fatalf("query dropped %s", polledProxy)
	}
}
