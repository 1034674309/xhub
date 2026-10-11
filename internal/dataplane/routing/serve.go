package routing

import (
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// dropDisabled removes deployments the dashboard has disabled.
// 参数 pool：已经按模型名匹配过的部署。
// 返回 active：model_info.disabled 不为 true 的部署。disabled：被拿掉的个数。
// 调用：Serve、pickDeployment。
// 测试：disabled_test.go。
func DropDisabled(pool []config.ModelEntry) ([]config.ModelEntry, int) {
	active := make([]config.ModelEntry, 0, len(pool))
	disabled := 0
	for _, dep := range pool {
		if dep.Disabled() {
			disabled++
			continue
		}
		active = append(active, dep)
	}
	return active, disabled
}

// preferDeployment 把钉住的部署移到候选列表的第一位。id 为空或不在列表里时顺序不变。
// 参数 pool：路由排序后的部署。id：deployment_affinity 里存的 CooldownID。
// 返回：同一批部署，钉住的那条在下标 0。
// 调用：Serve 在 PlanRoute 给出 Pinned 之后。测试：prefer_test.go TestPreferDeployment*。
func PreferDeployment(pool []config.ModelEntry, id string) []config.ModelEntry {
	if id == "" {
		return pool
	}
	for i, entry := range pool {
		if router.CooldownID(entry) != id {
			continue
		}
		if i == 0 {
			return pool
		}
		out := make([]config.ModelEntry, 0, len(pool))
		out = append(out, entry)
		out = append(out, pool[:i]...)
		out = append(out, pool[i+1:]...)
		return out
	}
	return pool
}
