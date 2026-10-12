// 本文件验证 routing 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package routing

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// TestFallbackClassificationMarkers 验证全部专用标记、大小写与状态优先级，并拒绝非错误包络。
// 前置纯内存错误正文，断言正确分类且不依赖供应商或存储，无资源清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestFallbackClassificationMarkers(t *testing.T) {
	for _, category := range []struct {
		kind    string
		markers []string
	}{
		{"context", []string{"CONTEXT_LENGTH_EXCEEDED", "context_window_exceeded", "Maximum Context Length", "prompt is too long", "input is too long"}},
		{"content", []string{"CONTENT_POLICY_VIOLATION", "content_filter", "content filtering policy", "Safety Policy", "safety_violation"}},
	} {
		for _, marker := range category.markers {
			t.Run(marker, func(t *testing.T) {
				raw, _ := json.Marshal(map[string]any{"error": map[string]any{"message": marker}})
				if got := FallbackKind(400, raw); got != category.kind {
					t.Fatalf("标记%s分类为%s", marker, got)
				}
				if got := FallbackKind(503, raw); got != "general" {
					t.Fatalf("503应优先通用策略: %s", got)
				}
			})
		}
	}
	for _, raw := range []string{`null`, `[]`, `{"error":null}`, `{"message":"context_length_exceeded"}`, `{"error":{"code":"invalid_request"},"text":"content_filter"}`} {
		if got := FallbackKind(400, []byte(raw)); got != "" {
			t.Fatalf("非专用错误触发回退: %s -> %s", raw, got)
		}
	}
}

// TestFallbackQueueNestedIsolation 验证嵌套链保持兄弟目标顺序、空池子链继续和请求间隔离。
// 前置共享不可变策略与两条独立请求队列；断言每条链顺序、类别切换和耗尽，不修改设置，无外部数据。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
// 测试只操作内存配置或本地假上游，HTTP 服务及正文由本例关闭，不创建外部付费任务或持久化数据。
func TestFallbackQueueNestedIsolation(t *testing.T) {
	settings := router.BuiltinSettings()
	settings.ModelFallbacks = map[string]router.FallbackPolicy{
		"a":       {Fallbacks: []string{"missing", "b"}, ContextWindow: []string{"c"}},
		"missing": {Fallbacks: []string{"d"}},
		"b":       {ContentPolicy: []string{"c"}},
	}
	models := []config.ModelEntry{}
	for _, name := range []string{"b", "c", "d"} {
		models = append(models, config.ModelEntry{ModelName: name, LiteLLMParams: map[string]any{"deployment_id": name}})
	}
	before, _ := json.Marshal(settings)
	first, second := NewFallbackQueue(settings, "a", false), NewFallbackQueue(settings, "a", false)
	for _, tc := range []struct {
		q          *fallbackQueue
		kind, want string
	}{
		{first, "general", "b"}, {second, "context", "c"}, {first, "content", "d"}, {first, "general", "c"}, {first, "general", ""}, {second, "general", ""},
	} {
		pool, result := tc.q.Next(tc.kind, models, router.State{})
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		got := ""
		if len(pool) > 0 {
			got = pool[0].ModelName
		}
		if got != tc.want {
			t.Fatalf("%s下一目标%s，预期%s", tc.kind, got, tc.want)
		}
	}
	after, _ := json.Marshal(settings)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("请求队列修改了共享策略")
	}
	if len(second.Seen) != 2 {
		t.Fatalf("其他请求污染已访问集: %v", second.Seen)
	}
}
