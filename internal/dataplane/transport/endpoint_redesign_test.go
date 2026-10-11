package transport

import (
	"testing"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// TestNativeURLParameterEncoding 验证模型斜杠与空格编码；参数 t 为上下文，固定路径纯函数无需清理。
func TestNativeURLParameterEncoding(t *testing.T) {
	hit := providercontract.Hit{Action: providercontract.Action{UpstreamPath: "/models/{model}:generateContent"}, Names: map[string]string{"model": "real/name with space"}}
	got := BypassDeploymentURL("https://example.test/projects/p", hit)
	if got != "https://example.test/projects/p/models/real%2Fname%20with%20space:generateContent" {
		t.Fatal(got)
	}
	if got := bypassURL("https://example.test/root/v1", "/v1/images/edits"); got != "https://example.test/root/v1/images/edits" {
		t.Fatal(got)
	}
}
