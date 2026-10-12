// 本文件验证 transport 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
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
