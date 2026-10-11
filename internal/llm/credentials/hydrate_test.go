package credentials

import (
	"testing"
)

// TestHydrateCredentialAPIKeyReplacesStaleDeploymentKey 验证可信密钥替换旧密钥而地址保留部署设置；仅内存输入，无清理。
func TestHydrateCredentialAPIKeyReplacesStaleDeploymentKey(t *testing.T) {
	got := Hydrate(map[string]any{
		"api_key":                 "qiniu-test-key",
		"api_base":                "https://api.qnaigc.com/v1",
		"litellm_credential_name": "qiniu",
	}, map[string]any{
		"api_key":  "sk-real",
		"api_base": "https://credential.example/v1",
	})
	if got["api_key"] != "sk-real" {
		t.Fatalf("api_key %v", got["api_key"])
	}
	if got["api_base"] != "https://api.qnaigc.com/v1" {
		t.Fatalf("api_base %v", got["api_base"])
	}
}

// TestHydrateKeepsDeploymentKeyWhenCredentialHasNone 验证空密钥不覆盖部署并填入缺失地址；仅内存输入，无清理。
func TestHydrateKeepsDeploymentKeyWhenCredentialHasNone(t *testing.T) {
	got := Hydrate(map[string]any{
		"api_key":  "sk-model",
		"api_base": "",
	}, map[string]any{
		"api_key":  "   ",
		"api_base": "https://credential.example/v1",
	})
	if got["api_key"] != "sk-model" {
		t.Fatalf("api_key %v", got["api_key"])
	}
	if got["api_base"] != "https://credential.example/v1" {
		t.Fatalf("api_base %v", got["api_base"])
	}
}

// TestHydrateBoundaries 验证 nil、空白、未知字段、环境变量与输入隔离；参数 t 为单测上下文。
// 前置临时环境变量，由 t.Setenv 恢复；返回无，不访问外部服务或写入持久化。
func TestHydrateBoundaries(t *testing.T) {
	t.Setenv("XHUB_TEST_CREDENTIAL", "env-secret")
	params := map[string]any{"api_key": "old", "custom_llm_provider": " OpenAI "}
	got := Hydrate(params, map[string]any{"api_key": "os.environ/XHUB_TEST_CREDENTIAL", "unknown": "must-not-copy", "azure_ad_token": "unsupported", "vertex_credentials": "unsupported", "api_base": " "})
	if got["api_key"] != "env-secret" || got["custom_llm_provider"] != "openai" || got["unknown"] != nil || got["azure_ad_token"] != nil || got["vertex_credentials"] != nil || got["api_base"] != nil || params["api_key"] != "old" {
		t.Fatalf("凭据边界错误: %#v", got)
	}
	if got := Hydrate(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("nil 输入结果: %#v", got)
	}
	if got := Hydrate(nil, map[string]any{"api_key": "os.environ/XHUB_TEST_MISSING_CREDENTIAL"}); got["api_key"] != "" {
		t.Fatalf("缺失环境变量未降级为空: %#v", got)
	}
}
