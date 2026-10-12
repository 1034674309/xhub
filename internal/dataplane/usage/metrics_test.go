// 本文件验证 usage 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package usage

import (
	"testing"
	"time"
)

// TestOutputTokensCountsStreamedTextWhenUsageIsMissing 验证缺失实测用量时，流式文本能得到非零输出估算。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestOutputTokensCountsStreamedTextWhenUsageIsMissing(t *testing.T) {
	raw := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
	if got := OutputTokens(raw); got < 1 {
		t.Fatalf("output tokens %d", got)
	}
}

// TestTTFTMillisOmitsAnUnmeasuredDelay 验证零延迟保持未测量语义，正延迟转为正确毫秒指针。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestTTFTMillisOmitsAnUnmeasuredDelay(t *testing.T) {
	if TtftMillis(0) != nil {
		t.Fatal("zero ttft must stay unset")
	}
	got := TtftMillis(1500 * time.Millisecond)
	if got == nil || *got != 1500 {
		t.Fatalf("ttft %v", got)
	}
}
