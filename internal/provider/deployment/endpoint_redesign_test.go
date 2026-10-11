package deployment_test

import (
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	providercapability "github.com/sunqirui1987/xhub/internal/provider/capability"
	providerdeployment "github.com/sunqirui1987/xhub/internal/provider/deployment"
)

// TestEndpointRedesignContract 校验三类原生接口及 Bypass 目录、独立图片操作和移除旧 ID。
// 参数 t 为测试上下文；前置注册目录，返回无；无网络与持久数据，无需清理。
func TestEndpointRedesignContract(t *testing.T) {
	for _, old := range []string{"image", "bypass:fal-video"} {
		if providercapability.KnownEndpoint(old) {
			t.Fatalf("旧端点仍被接受: %s", old)
		}
	}
	for _, e := range providercapability.EndpointTypes() {
		if e.Category != "openai" && e.Category != "vertex" && e.Category != "claude" && e.Category != "bypass" {
			t.Fatalf("缺少分类: %+v", e)
		}
	}
	for _, tc := range []struct {
		transport, endpoint string
		valid               bool
	}{
		{"openai_image_generation", "image_generation", true}, {"openai_image_generation", "image_edit", false},
		{"openai_image_edit", "image_edit", true}, {"openai_image_edit", "image", false},
		{"gemini_generate_content", "gemini", true}, {"gemini_generate_content", "vertex", true},
		{"vertex_generate_content", "vertex", true}, {"openai_videos", "video", true},
	} {
		m := config.ModelEntry{ModelName: "alias", LiteLLMParams: map[string]any{"model": "real-model", "custom_llm_provider": "custom"}, ModelInfo: map[string]any{"transport": tc.transport, "endpoint_types": []string{tc.endpoint}}}
		if err := providerdeployment.ValidateDeployment(m); (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
		if tc.valid && len(providerdeployment.DeploymentEndpoints(m)) == 0 {
			t.Errorf("未公开已实现端点: %s", tc.endpoint)
		}
	}
}

// TestGoogleAliasValidation 验证所有对话协议的公开别名支持路径和冒号；空段、点段提供中文原因，上游 ID 不受限制。
// 参数 t 为测试上下文，返回无；只读注册表，无持久数据及清理。
func TestGoogleAliasValidation(t *testing.T) {
	for _, transport := range []string{"gemini_generate_content", "vertex_generate_content", "bypass_openai_chat", "bypass_openai_responses", "bypass_anthropic_messages"} {
		for _, name := range []string{"alias", "alias name", "", ".", "..", "group/model", "alias:op", "group/model:latest", "名称/版本：最新", "group//model", "group/../model", "/model", "model/"} {
			m := config.ModelEntry{ModelName: name, LiteLLMParams: map[string]any{"model": "group/real:latest"}, ModelInfo: map[string]any{"transport": transport, "endpoint_types": []string{"chat"}}}
			valid := name == "alias" || name == "alias name" || name == "group/model" || name == "alias:op" || name == "group/model:latest" || name == "名称/版本：最新"
			if err := providerdeployment.ValidateDeployment(m); (err == nil) != valid {
				t.Errorf("别名 %q 校验错误: %v", name, err)
			} else if !valid && !strings.Contains(err.Error(), "对外模型名称") {
				t.Errorf("别名 %q 缺少中文校验说明: %v", name, err)
			}
			if !valid && len(providerdeployment.DeploymentEndpoints(m)) > 0 {
				t.Errorf("非法别名 %q 公布了不可调用路径", name)
			}
		}
	}
}
