package routing

import (
	"encoding/json"
	"strings"

	"github.com/sunqirui1987/xhub/internal/config"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	"github.com/sunqirui1987/xhub/internal/router"
)

// fallbackKind 分类上游失败，参数为状态和错误正文，返回 general/context/content 或空。
// 调用：适配与原生执行；仅识别上游错误包络，不把成功文本或本地护栏当作触发器。
func FallbackKind(status int, body []byte) string {
	if status >= 500 || status == 429 {
		return "general"
	}
	if status < 400 {
		return ""
	}
	var doc map[string]any
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	raw, ok := doc["error"]
	if !ok {
		return ""
	}
	text, _ := json.Marshal(raw)
	s := strings.ToLower(string(text))
	for _, marker := range []string{"context_length_exceeded", "context_window_exceeded", "maximum context length", "prompt is too long", "input is too long"} {
		if strings.Contains(s, marker) {
			return "context"
		}
	}
	for _, marker := range []string{"content_policy_violation", "content_filter", "content filtering policy", "safety policy", "safety_violation"} {
		if strings.Contains(s, marker) {
			return "content"
		}
	}
	return ""
}

// fallbackQueue 保存请求私有的有序模型队列，最多访问 32 个不同模型以限制链放大。
// 已输出流式字节或响应继续请求不调用队列；重复、禁用和不兼容目标不重放。
type fallbackQueue struct {
	// 本请求的完整路由设置快照，保留已解析模型默认和回退图，不修改共享模板。
	settings prefs.RouteSettings
	// 按配置顺序等待访问的公开模型/组队列，Next 消费，Seen 去重。
	pending []string
	// 已访问公开模型/组集合，用于环保护及最多 32 个目标的访问限制。
	Seen map[string]bool
	// 当前失败时采用的回退源，显式组链优先，否则由实际成员更新。
	Current string
	// 当前展开部署的模型/组名；不同于可能指向成员的 Current。
	PoolAlias string
	// 禁止本请求新建跨模型回退，也禁止复用跨模型动态偏好；父响应归属仍须单独校验。
	Disabled bool
	// 原请求别名到实际模型池的有序回退边，每轮按当前模板验证；直接命中时为空，不能作为授权。
	Path []dpcontract.RouteStep
	// 每个排队目标首次发现的完整回退路径，恢复成功偏好及验证模板可达性使用。
	paths map[string][]dpcontract.RouteStep
}

// newFallbackQueue 初始化请求状态，参数为完整配置、起始模型和禁用标志。
// 返回私有队列，不修改共享设置；调用：两类数据面创建入口。
func NewFallbackQueue(settings prefs.RouteSettings, alias string, disabled bool) *fallbackQueue {
	return &fallbackQueue{settings: settings, Current: alias, PoolAlias: alias, Seen: map[string]bool{alias: true}, Disabled: disabled, paths: map[string][]dpcontract.RouteStep{}}
}

// next 在当前模型耗尽后加入对应错误目标并选择下一兼容模型。
// 参数为错误类型、兼容部署与运行状态；返回部署和模型配置，耗尽或禁用时为空。
// 调用方发送前验证目标权限与预算；空池跳过，配置错误通过 Err 返回，无共享状态修改。
func (q *fallbackQueue) Next(kind string, models []config.ModelEntry, state router.State) ([]config.ModelEntry, prefs.RouteSettings) {
	if q.Disabled {
		return nil, q.settings
	}
	q.enqueue(q.Current, kind)
	for len(q.pending) > 0 && len(q.Seen) < 32 {
		name := q.pending[0]
		q.pending = q.pending[1:]
		if q.Seen[name] {
			continue
		}
		q.Seen[name] = true
		q.Current = name
		q.PoolAlias = name
		q.Path = q.paths[name]
		settings := q.settings.ForModel(name)
		if settings.Err != nil {
			return nil, settings
		}
		state.Allocations = settings.Policy.Shares()
		pool := settings.Schedule(models, name, state, "")
		if len(pool) > 0 {
			return pool, settings
		}
		q.enqueue(name, "general")
	}
	return nil, q.settings
}

// enqueue 保存每条候选的实际回退路径；参数为失败源和错误类别，返回无。
// Next 调用；保留首次路径与配置顺序，重复目标不扩大可达范围。
func (q *fallbackQueue) enqueue(from, kind string) {
	for _, target := range q.settings.ModelFallbacks[from].Targets(kind) {
		if _, exists := q.paths[target]; !exists {
			path := append([]dpcontract.RouteStep(nil), q.Path...)
			q.paths[target] = append(path, dpcontract.RouteStep{From: from, To: target, Kind: kind})
		}
		q.pending = append(q.pending, target)
	}
}

// useDeployment 选择本次失败的回退源，显式组链优先，否则采用当前真实成员。
// 参数 name 为部署公开模型名；两类数据面发送前调用，返回空，仅更新请求私有状态。
// poolAlias 在切换回退池时重置，避免第一次成员的配置粘住后续成员。
func (q *fallbackQueue) UseDeployment(name string) {
	q.Current = name
	if _, explicit := q.settings.ModelFallbacks[q.PoolAlias]; explicit {
		q.Current = q.PoolAlias
	}
}

// hasFallbackTargets 判断三类错误是否任一可重放；原生入口选择缓冲响应时调用，无修改。
func HasFallbackTargets(p router.FallbackPolicy) bool {
	return len(p.Fallbacks)+len(p.ContextWindow)+len(p.ContentPolicy) > 0
}
