package transport

import (
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestNativeSupplierURLs 验证任何域名都只遵循配置地址与显式传输，模型前缀由所选传输处理。
// 参数 t：测试上下文。返回：无；只组合本地声明，不访问供应商。
func TestNativeSupplierURLs(t *testing.T) {
	for _, spec := range []struct{ base, path, wantURL, model, wantModel string }{
		{"https://api.qnaigc.com/v1", "/bypass/openai/v1/responses", "https://api.qnaigc.com/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://api.modelink.ai/custom/v1", "/bypass/openai/v1/responses", "https://api.modelink.ai/custom/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://api.openai.com/v1", "/bypass/openai/v1/responses", "https://api.openai.com/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://configured.example/bypass/openai/v1", "/bypass/openai/v1/responses", "https://configured.example/bypass/openai/v1/responses", "openai/gpt-test", "openai/gpt-test"},
	} {
		t.Run(spec.wantURL, func(t *testing.T) {
			hit, ok := providerregistry.Match("POST", spec.path)
			if !ok {
				t.Fatal("unregistered native path", spec.path)
			}
			if got := BypassDeploymentURL(spec.base, hit); got != spec.wantURL {
				t.Errorf("URL = %s, want %s", got, spec.wantURL)
			}
			if got := providercontract.OfficialID(hit.Transport.StripPrefix, spec.model); got != spec.wantModel {
				t.Errorf("model = %s, want %s", got, spec.wantModel)
			}
		})
	}
}
