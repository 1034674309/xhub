package router

import (
	"github.com/sunqirui1987/xhub/internal/config"
	"testing"
)

// TestScheduleAffinityOrder 验证粘性置顶后的其余候选仍按并发排序，冷却粘性不能被使用。
// 前置三个内存部署和运行快照；不改变目录，无外部数据需要清理。
func TestScheduleAffinityOrder(t *testing.T) {
	list := []config.ModelEntry{}
	for _, id := range []string{"slow", "fast", "pin"} {
		list = append(list, config.ModelEntry{ModelName: "m", LiteLLMParams: map[string]any{"deployment_id": id, "model": id}})
	}
	pin := CooldownID(list[2])
	state := State{Busy: map[string]int{CooldownID(list[0]): 9, CooldownID(list[1]): 1, pin: 20}}
	got := Schedule(list, "m", "least-busy", state, pin)
	if len(got) != 3 || DeploymentID(got[0]) != "pin" || DeploymentID(got[1]) != "fast" || DeploymentID(got[2]) != "slow" {
		t.Fatalf("粘性剩余排序错误: %v", got)
	}
	state.Cooldown = map[string]bool{pin: true}
	got = Schedule(list, "m", "least-busy", state, pin)
	if len(got) != 2 || DeploymentID(got[0]) != "fast" {
		t.Fatalf("冷却粘性未失效: %v", got)
	}
}
