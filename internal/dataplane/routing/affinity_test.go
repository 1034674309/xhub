package routing

import (
	"github.com/sunqirui1987/xhub/internal/config"
	contract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	"github.com/sunqirui1987/xhub/internal/router"
	"testing"
)

// TestAffinityRevalidatesFallback 验证回退偏好遵守链路撤销、零权重、冷却和请求禁用，父响应只允许原归属。
// 前置内存目录与完整模板；普通失效偏好恢复正常池，无外部数据需要清理。
func TestAffinityRevalidatesFallback(t *testing.T) {
	models := []config.ModelEntry{
		{ModelName: "a", LiteLLMParams: map[string]any{"deployment_id": "a-id", "model": "a"}},
		{ModelName: "b", LiteLLMParams: map[string]any{"deployment_id": "b-id", "model": "b"}},
		{ModelName: "b", LiteLLMParams: map[string]any{"deployment_id": "b-alt", "model": "b-alt"}},
	}
	target := contract.RouteTarget{DeploymentID: "b-id", Model: "b", Pool: "b", Path: []contract.RouteStep{{From: "a", To: "b", Kind: "general"}}}
	for _, mode := range []string{"valid", "revoked", "zero", "cooldown", "disabled", "parent", "parent-revoked", "wrong-model"} {
		t.Run(mode, func(t *testing.T) {
			settings := prefs.BuiltinSettings()
			settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"b"}}}
			state := router.State{}
			if mode == "revoked" || mode == "parent-revoked" {
				settings.ModelFallbacks = map[string]router.FallbackPolicy{}
			}
			if mode == "zero" {
				settings.ModelDefaults = map[string]any{"b": map[string]any{"allocations": []any{map[string]any{"deployment_id": "b-id", "weight": 0}, map[string]any{"deployment_id": "b-alt", "weight": 1}}}}
			}
			if mode == "cooldown" {
				state.Cooldown = map[string]bool{router.CooldownID(models[1]): true}
			}
			plan := contract.RoutePlan{Target: target, Required: mode == "parent" || mode == "parent-revoked"}
			if mode == "wrong-model" {
				plan.Target.Model = "unrelated"
			}
			cfg := settings.ForModel("a")
			pool := cfg.Schedule(models, "a", state, "")
			queue := NewFallbackQueue(settings, "a", mode == "disabled" || mode == "parent")
			got, _ := queue.SelectAffinity(&plan, pool, "a", models, state, cfg)
			if mode == "parent-revoked" {
				if len(got) != 0 {
					t.Fatal("被撤销的父响应路径仍可调用")
				}
				return
			}
			want := "a-id"
			if mode == "valid" || mode == "parent" {
				want = "b-id"
			}
			if len(got) == 0 || router.DeploymentID(got[0]) != want {
				t.Fatalf("模式 %s 目标 %v，期望 %s", mode, got, want)
			}
			if mode == "parent" && len(got) != 1 {
				t.Fatal("父响应出现替代候选")
			}
		})
	}
}
