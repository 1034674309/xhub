package routing

import (
	"github.com/sunqirui1987/xhub/internal/config"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	"github.com/sunqirui1987/xhub/internal/router"
)

// RuntimePin 将稳定 ID 转为当前运行身份；参数为当前兼容候选和部署 ID，返回运行 ID 或空。
// 两类执行入口调用；地址变化不破坏会话偏好，缺少稳定 ID 的部署不建立粘性。
func RuntimePin(models []config.ModelEntry, id string) string {
	if id != "" {
		for _, dep := range models {
			if router.DeploymentID(dep) == id {
				return router.CooldownID(dep)
			}
		}
	}
	return ""
}

// Target 记录实际成功池与回退路径；参数为部署，返回稳定目标，执行器在尝试前调用。
// 不改变路由队列；无稳定 ID 时上层不会提交会话偏好。
func (q *fallbackQueue) Target(dep config.ModelEntry) dpcontract.RouteTarget {
	return dpcontract.RouteTarget{DeploymentID: router.DeploymentID(dep), Model: dep.ModelName, Pool: q.PoolAlias, Path: append([]dpcontract.RouteStep(nil), q.Path...)}
}

// preferredPool 重新验证偏好路径和当前候选；参数为目标、原始别名、目录及运行状态，返回合法池和设置。
// SelectAffinity 调用；路径每条边须仍存在，成员链只能从当前池展开的成员出发，禁用回退不能复用跨模型偏好。
func (q *fallbackQueue) preferredPool(target dpcontract.RouteTarget, alias string, models []config.ModelEntry, state router.State, allowFallback bool) ([]config.ModelEntry, prefs.RouteSettings) {
	poolName := alias
	if target.DeploymentID == "" {
		return nil, q.settings
	}
	if len(target.Path) > 0 && !allowFallback {
		return nil, q.settings
	}
	for _, step := range target.Path {
		if step.Kind != "general" && step.Kind != "context" && step.Kind != "content" {
			return nil, q.settings
		}
		sourceOK := step.From == poolName
		// 无组别名链时，现有回退规则允许真实成员的回退链。
		if !sourceOK {
			if _, explicit := q.settings.ModelFallbacks[poolName]; explicit {
				return nil, q.settings
			}
			for _, group := range q.settings.RoutingGroups {
				if group.Name == poolName {
					for _, member := range group.Models {
						sourceOK = sourceOK || member == step.From
					}
				}
			}
		}
		edgeOK := false
		for _, name := range q.settings.ModelFallbacks[step.From].Targets(step.Kind) {
			edgeOK = edgeOK || name == step.To
		}
		if !sourceOK || !edgeOK {
			return nil, q.settings
		}
		poolName = step.To
	}
	if target.Pool != poolName {
		return nil, q.settings
	}
	settings := q.settings.ForModel(poolName)
	if settings.Err != nil {
		return nil, settings
	}
	pin := RuntimePin(models, target.DeploymentID)
	if pin == "" {
		return nil, settings
	}
	state.Allocations = settings.Policy.Shares()
	pool := settings.Schedule(models, poolName, state, pin)
	if len(pool) == 0 || router.DeploymentID(pool[0]) != target.DeploymentID || pool[0].ModelName != target.Model {
		return nil, settings
	}
	return pool, settings
}

// SelectAffinity 应用已验证偏好并协调首次并发申请；参数为计划、当前池、原别名、候选和状态，返回池及生效设置。
// 执行器发送前调用，目标权限与凭据仍由原执行链检查；固定响应归属不申请动态偏好。
func (q *fallbackQueue) SelectAffinity(plan *dpcontract.RoutePlan, pool []config.ModelEntry, alias string, models []config.ModelEntry, state router.State, settings prefs.RouteSettings) ([]config.ModelEntry, prefs.RouteSettings) {
	if plan.Required {
		owned, cfg := q.preferredPool(plan.Target, alias, models, state, true)
		if len(owned) > 0 {
			q.PoolAlias, q.Current, q.Path = plan.Target.Pool, plan.Target.Pool, plan.Target.Path
			return owned[:1], cfg
		}
		return nil, settings
	}
	target := plan.Target
	preferred, chosen := q.preferredPool(target, alias, models, state, !q.Disabled)
	if len(preferred) > 0 {
		pool, settings = preferred, chosen
	}
	if len(pool) == 0 {
		return pool, settings
	}
	if plan.Claim != nil && router.DeploymentID(pool[0]) != "" {
		candidate := q.Target(pool[0])
		if len(preferred) > 0 {
			candidate = target
		}
		winner := plan.Claim(candidate)
		if won, cfg := q.preferredPool(winner, alias, models, state, !q.Disabled); len(won) > 0 {
			pool, settings, target = won, cfg, winner
			preferred = won
		}
	}
	if len(preferred) > 0 {
		q.PoolAlias, q.Current, q.Path = target.Pool, target.Pool, append([]dpcontract.RouteStep(nil), target.Path...)
		for _, step := range q.Path {
			q.Seen[step.From] = true
			q.Seen[step.To] = true
		}
	}
	return pool, settings
}
