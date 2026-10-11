package routing

import (
	"fmt"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	"github.com/sunqirui1987/xhub/internal/router"
)

// TestFallbackClassification 验证限流/服务错误与专用错误分类；成功、普通4xx和非法JSON不触发，无外部状态。
func TestFallbackClassification(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, want string
	}{
		{500, "", "general"}, {429, "", "general"}, {400, "invalid-json", ""},
		{400, "{}", ""}, {200, "{\"error\":\"context_length_exceeded\"}", ""},
		{400, "{\"error\":{\"code\":\"context_length_exceeded\"}}", "context"},
		{400, "{\"error\":{\"message\":\"maximum context length exceeded\"}}", "context"},
		{403, "{\"error\":{\"code\":\"content_policy_violation\"}}", "content"},
		{401, "{\"error\":{\"code\":\"invalid_api_key\"}}", ""},
	} {
		if got := FallbackKind(tc.status, []byte(tc.body)); got != tc.want {
			t.Fatalf("状态%d 正文%s: %q != %q", tc.status, tc.body, got, tc.want)
		}
	}
}

// TestFallbackQueue 验证有序目标、去重、空池跳过、禁用和错误传播；前置内存路由，断言无共享配置修改，无需清理。
func TestFallbackQueue(t *testing.T) {
	settings := prefs.BuiltinSettings()
	settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"missing", "b", "c"}}, "b": {Fallbacks: []string{"c", "a"}}}
	models := []config.ModelEntry{{ModelName: "b", LiteLLMParams: map[string]any{"deployment_id": "b"}}, {ModelName: "c", LiteLLMParams: map[string]any{"deployment_id": "c"}}}
	q := NewFallbackQueue(settings, "a", false)
	for _, want := range []string{"b", "c", ""} {
		pool, _ := q.Next("general", models, router.State{})
		if want == "" {
			if len(pool) != 0 {
				t.Fatal("重复模型被再次调用")
			}
		} else if len(pool) != 1 || pool[0].ModelName != want {
			t.Fatalf("回退顺序 %s: %v", want, pool)
		}
	}
	if pool, _ := NewFallbackQueue(settings, "a", true).Next("general", models, router.State{}); len(pool) != 0 {
		t.Fatal("禁用回退未生效")
	}
	if pool, _ := NewFallbackQueue(settings, "a", false).Next("content", models, router.State{}); len(pool) != 0 {
		t.Fatal("专用类别误用通用策略")
	}
	settings.ModelDefaults = map[string]any{"b": map[string]any{"allocations": "bad"}}
	_, result := NewFallbackQueue(settings, "a", false).Next("general", models, router.State{})
	if result.Err == nil {
		t.Fatal("目标权重错误未传播")
	}
}

// TestFallbackQueueBoundAndCooldown 验证冷却目标跳过和请求最多访问32个模型的边界。
// 参数 t 为测试上下文，无返回；前置内存模型目录和冷却快照，验证链不可无限放大，无持久数据。
func TestFallbackQueueBoundAndCooldown(t *testing.T) {
	settings := prefs.BuiltinSettings()
	settings.ModelFallbacks = map[string]router.FallbackPolicy{}
	models := []config.ModelEntry{}
	for i := 0; i < 40; i++ {
		name := fmt.Sprint(i)
		settings.ModelFallbacks[name] = router.FallbackPolicy{Fallbacks: []string{fmt.Sprint(i + 1)}}
		models = append(models, config.ModelEntry{ModelName: name, LiteLLMParams: map[string]any{"model": name, "deployment_id": name}})
	}
	q := NewFallbackQueue(settings, "0", false)
	state := router.State{Cooldown: map[string]bool{router.CooldownID(models[1]): true}}
	count := 0
	for {
		pool, _ := q.Next("general", models, state)
		if len(pool) == 0 {
			break
		}
		if pool[0].ModelName == "1" {
			t.Fatal("冷却目标未跳过")
		}
		count++
	}
	if count != 30 || len(q.Seen) != 32 {
		t.Fatalf("回退边界错误: 调用%d 已访问%d", count, len(q.Seen))
	}
}

// TestFallbackGroupSource 验证组链优先、显式禁用和每次实际成员更新；仅内存队列，测试后自动释放。
func TestFallbackGroupSource(t *testing.T) {
	settings := prefs.BuiltinSettings()
	settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"backup-a"}}, "b": {ContextWindow: []string{"backup-b"}}}
	q := NewFallbackQueue(settings, "group", false)
	q.UseDeployment("a")
	q.UseDeployment("b")
	if q.Current != "b" {
		t.Fatalf("成员源粘住: %s", q.Current)
	}
	settings.ModelFallbacks["group"] = router.FallbackPolicy{}
	q = NewFallbackQueue(settings, "group", false)
	q.UseDeployment("b")
	if q.Current != "group" {
		t.Fatal("显式组禁用没有优先")
	}
	settings.ModelFallbacks["group"] = router.FallbackPolicy{ContextWindow: []string{"backup-b"}}
	if !HasFallbackTargets(settings.ModelFallbacks["group"]) || HasFallbackTargets(router.FallbackPolicy{}) {
		t.Fatal("专用错误缓冲判定错误")
	}
}
