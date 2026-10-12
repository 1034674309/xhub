// 本文件提供停用剔除与候选顺序调整的纯局部操作，权限和凭据仍由执行器检查。
// 调用关系与边界：请求候选选择的辅助能力；具体参数、返回及副作用见各方法说明。
package routing

import (
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// DropDisabled 从部署池中剔除明确停用的条目，保留其他条目的原始顺序。
// 参数 pool 为目录或候选池；返回可用切片及剔除数量；两类入口在能力筛选或调度前调用。
// 空池返回空结果和零计数；停用与缺少能力是不同原因，诊断可利用计数说明配置状态。
// 不改写 ModelEntry 内部映射，不检查冷却、权限或凭据，调用方仍须执行后续筛选。
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

// PreferDeployment 把运行身份匹配的候选移到首位，其余候选保持相对顺序。
// 参数 pool 为已调度候选，id 为 CooldownID 运行身份；返回优先排列的池。
// 路由相关调用与测试使用；空 ID、未知 ID 或原本第一项匹配时直接返回原切片。
// 确需移动时新建切片，避免修改共享候选顺序；ModelEntry 内的映射仍是浅拷贝，不可借返回值改写共享配置。
// 仅影响排序，不使停用、冷却或未授权的部署重新获得资格。
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
