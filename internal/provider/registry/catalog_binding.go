package registry

import (
	"slices"
)

// CatalogModels 返回目录 ID 到上游模型、执行传输白名单的独立副本。
// 参数无；返回编译时供应商注册的能力表，供公开目录及表单使用；不读取任意文件路径。
func CatalogModels() map[string]map[string][]string {
	mu.Lock()
	defer mu.Unlock()
	out := map[string]map[string][]string{}
	for id, models := range catalogModels {
		out[id] = map[string][]string{}
		for model, ids := range models {
			out[id][model] = slices.Clone(ids)
		}
	}
	return out
}
