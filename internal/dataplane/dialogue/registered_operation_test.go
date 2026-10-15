// 本文件验证 dialogue 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package dialogue

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
)

// TestDialogueRequestBodyRemovesConsumedProxyFields 验证网关消费的护栏和追踪字段不会进入统一协议转换。
// 前置：正文包含被护栏改写后的消息、两个代理字段和一个未知能力；结果：仅删除已消费字段，原正文不变，未知能力继续留给解析器拒绝；无外部数据需要清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestDialogueRequestBodyRemovesConsumedProxyFields(t *testing.T) {
	body := map[string]any{
		"model": "public", "messages": []any{map[string]any{"role": "user", "content": "[手机号已隐藏]"}},
		"session_id": "conversation", "prompt_cache_key": "native-cache", "guardrails": []any{"redact"}, "litellm_trace_id": "trace-1", "unknown_capability": true,
	}
	clean := DialogueRequestBody(body)
	for _, key := range []string{"session_id"} {
		if clean[key] != nil || body[key] == nil {
			t.Fatalf("会话字段未剥离或原正文被修改: %s", key)
		}
	}
	if _, ok := clean["guardrails"]; ok {
		t.Fatal("guardrails 仍进入统一协议转换")
	}
	if _, ok := clean["litellm_trace_id"]; ok {
		t.Fatal("追踪字段仍进入统一协议转换")
	}
	if clean["unknown_capability"] != true {
		t.Fatal("未知能力被静默删除")
	}
	if !reflect.DeepEqual(clean["messages"], body["messages"]) {
		t.Fatal("护栏改写后的消息没有保留")
	}
	if _, ok := body["guardrails"]; !ok {
		t.Fatal("整理统一正文修改了原始请求")
	}
	if clean["prompt_cache_key"] != "native-cache" {
		t.Fatal("原生缓存参数被删除")
	}
	d, err := llmdialogue.ParseDialogue("openai-chat", clean)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = llmdialogue.EncodeDialogue(d, "openai-responses", "model"); err == nil {
		t.Fatal("未知能力跨协议必须拒绝")
	}
	if out, err := llmdialogue.EncodeDialogue(d, "openai-chat", "model"); err != nil || out["unknown_capability"] != true {
		t.Fatal("同协议扩展能力必须保留")
	}
}

// TestBuildRegisteredOperation 验证非对话执行只读取显式 transport，并用注册动作生成地址、鉴权和上游模型，剥离回退开关。
// 前置条件是本地 OpenAI 图片传输已登记；测试不发网络请求，也不依赖外部凭据；结果只存在内存中，无需清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestBuildRegisteredOperation(t *testing.T) {
	dep := config.ModelEntry{
		LiteLLMParams: map[string]any{
			"model":               "custom/image-model",
			"api_base":            "http://127.0.0.1:39999/root",
			"api_key":             "local-secret",
			"custom_llm_provider": "qiniu",
		},
		ModelInfo: map[string]any{
			"transport":      "openai_image_generation",
			"endpoint_types": []any{"image_generation"},
		},
	}
	body := map[string]any{"model": "public-alias", "prompt": "cat", "api_base": "https://must-not-forward.example", "disable_fallbacks": true, "metadata": map[string]any{"request": "test"}, "id": "item", "user": "owner", "vendor_extension": "keep"}
	upstream, err := BuildRegisteredOperation(dep, body, "images")
	if err != nil {
		t.Fatal("显式图片传输构造失败:", err)
	}
	if upstream.URL != "http://127.0.0.1:39999/root/v1/images/generations" {
		t.Fatalf("注册路径不正确: %s", upstream.URL)
	}
	if got := upstream.Header.Get("Authorization"); got != "Bearer local-secret" {
		t.Fatalf("注册鉴权不正确: %q", got)
	}
	var payload map[string]any
	if err := json.Unmarshal(upstream.Body, &payload); err != nil {
		t.Fatal("正文不是 JSON:", err)
	}
	if payload["model"] != "custom/image-model" || payload["prompt"] != "cat" {
		t.Fatalf("上游正文不正确: %#v", payload)
	}
	if _, exists := payload["api_base"]; exists {
		t.Fatalf("代理字段进入上游正文: %#v", payload)
	}
	if _, exists := payload["disable_fallbacks"]; exists {
		t.Fatalf("回退开关进入上游正文: %#v", payload)
	}
	for _, key := range []string{"metadata", "id", "user", "vendor_extension"} {
		if !reflect.DeepEqual(payload[key], body[key]) {
			t.Fatalf("合法字段 %s 被修改或误删: %#v", key, payload)
		}
	}
	if body["model"] != "public-alias" {
		t.Fatalf("构造请求修改了调用方正文: %#v", body)
	}
}

// TestBuildRegisteredOperationRequiresCredentials 验证显式执行缺少连接信息时返回可分类错误。
// 前置条件是传输已登记但部署没有地址和密钥；测试不访问网络，断言错误后无数据需要清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestBuildRegisteredOperationRequiresCredentials(t *testing.T) {
	dep := config.ModelEntry{
		LiteLLMParams: map[string]any{"model": "image-model"},
		ModelInfo: map[string]any{
			"transport":      "openai_image_generation",
			"endpoint_types": []any{"image_generation"},
		},
	}
	_, err := BuildRegisteredOperation(dep, map[string]any{"prompt": "cat"}, "images")
	if !errors.Is(err, ErrDialogueCredentials) {
		t.Fatalf("缺少凭据返回错误不正确: %v", err)
	}
}
