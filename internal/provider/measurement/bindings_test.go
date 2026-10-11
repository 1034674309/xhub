package measurement_test

import (
	"testing"

	providermeasurement "github.com/sunqirui1987/xhub/internal/provider/measurement"
)

// TestNativeTerminalUsageBlocking 验证流式图片终态缺少规格不能被中间事件的空用量过滤逻辑吞掉。
func TestNativeTerminalUsageBlocking(t *testing.T) {
	request := map[string]any{"quality": "auto", "size": "auto"}
	partial := providermeasurement.NativeStreamUsage("openai-images", []byte("data: {\"type\":\"image_generation.partial_image\"}\n\n"), request)
	if len(partial) != 0 {
		t.Fatal(partial)
	}
	final := providermeasurement.NativeStreamUsage("openai-images", []byte("data: {\"type\":\"image_generation.completed\",\"usage\":{\"input_tokens\":10}}\n\n"), request)
	if final["pricing_blocked"] != "image_variant_unknown" {
		t.Fatal(final)
	}
}
