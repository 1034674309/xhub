// 本文件验证 usage 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package usage

import (
	"reflect"
	"testing"
)

// TestCompleteUsagePreservesReportedZeroAndCacheInput 验证供应商明确报告零输入/输出时不被文本估算覆盖，缓存读写用量保持原值。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestCompleteUsagePreservesReportedZeroAndCacheInput(t *testing.T) {
	usage := map[string]any{
		"input_tokens":                float64(0),
		"output_tokens":               float64(0),
		"cache_read_input_tokens":     float64(19),
		"cache_creation_input_tokens": float64(3),
	}
	got := CompleteUsage(usage, map[string]any{"messages": []any{map[string]any{"content": "estimate me"}}}, []byte("data: {\"choices\":[{\"delta\":{\"content\":\"text\"}}]}\n"))
	if !reflect.DeepEqual(got, usage) {
		t.Fatalf("explicit usage changed: got %#v want %#v", got, usage)
	}
}

// TestEstimateTokensDoesNotCountOutputLimitAsInput 验证 max_tokens 输出限制不计入输入估算，避免把输出额度当作已输入 token。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestEstimateTokensDoesNotCountOutputLimitAsInput(t *testing.T) {
	without := EstimateTokens(map[string]any{"prompt": "abcd"})
	with := EstimateTokens(map[string]any{"prompt": "abcd", "max_tokens": 100000})
	if with != without {
		t.Fatalf("max_tokens changed input estimate: with=%d without=%d", with, without)
	}
}
