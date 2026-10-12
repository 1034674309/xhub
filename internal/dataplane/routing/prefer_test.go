// 本文件验证 routing 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package routing

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

// TestPreferDeploymentMovesThePinnedOneFirst 验证已存在偏好部署移动到首项，其余候选保持相对顺序且原切片不被改写。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestPreferDeploymentMovesThePinnedOneFirst(t *testing.T) {
	pool := []config.ModelEntry{
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://a.example", "model": "m"}},
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://b.example", "model": "m"}},
	}
	got := PreferDeployment(pool, "https://b.example|m")
	if got[0].ParamString("api_base", "") != "https://b.example" {
		t.Fatalf("first deployment %s", got[0].ParamString("api_base", ""))
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
}

// TestPreferDeploymentLeavesAnUnknownPinAlone 验证未知运行身份不改变候选顺序，不凭偏好创造额外部署。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestPreferDeploymentLeavesAnUnknownPinAlone(t *testing.T) {
	pool := []config.ModelEntry{
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://a.example", "model": "m"}},
	}
	got := PreferDeployment(pool, "missing")
	if got[0].ParamString("api_base", "") != "https://a.example" {
		t.Fatalf("pool changed: %+v", got)
	}
}
