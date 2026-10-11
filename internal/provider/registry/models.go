package registry

import (
	"strings"

	"github.com/sunqirui1987/xhub/internal/catalog"
)

// Model 是内置目录里显示在添加模型选择器上的一行。
//
// TransportID 是目录推荐的执行配置；用户入口由部署独立声明。
type Model struct {
	ID          string
	Provider    string
	Official    string
	Mode        string
	TransportID string
	Source      string
	Input       float64
	Output      float64
	Priced      bool
	PriceModel  string
	PriceSource string
}

// RegisterModel 登记模型默认传输及供应商目录的模型实现白名单。
// 同一目录型号可以登记多个执行实现；对外目录 ID 与显示名称无关。
// 参数 m（Model）：正在累加或展示的模型。
// 返回：无。id 为空时不登记。
// 调用：provider/volcengine/seedance.go
// 测试：catalog_binding_test.go。
func RegisterModel(m Model) {
	m.ID = strings.TrimSpace(m.ID)
	if m.ID == "" {
		return
	}
	if m.TransportID != "" {
		mu.Lock()
		modelEndpoints[m.ID] = m.TransportID
		if m.Provider != "" {
			if catalogModels[m.Provider] == nil {
				catalogModels[m.Provider] = map[string][]string{}
			}
			model := m.Official
			if model == "" {
				model = strings.TrimPrefix(m.ID, m.Provider+"/")
			}
			catalogModels[m.Provider][model] = append(catalogModels[m.Provider][model], m.TransportID)
		}
		mu.Unlock()
	}
	mode := m.Mode
	if mode == "" {
		mode = m.TransportID
	}
	catalog.Contribute(catalog.Row{
		ID:          m.ID,
		Provider:    m.Provider,
		Mode:        mode,
		TransportID: m.TransportID,
		Source:      m.Source,
		Input:       m.Input,
		Output:      m.Output,
		Priced:      m.Priced,
		Official:    m.Official,
		PriceModel:  m.PriceModel, PriceSource: m.PriceSource,
	})
}

// ModelEndpoints 把内置模型 id 映射到表单要预选的默认 transport ID。
// 参数：无。
// 返回 map[string]string（map[string]string）：模型 id 到端点类型。
// 调用：PublicBody。
// 测试：match_test.go、seedance_test.go
func ModelEndpoints() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]string, len(modelEndpoints))
	for k, v := range modelEndpoints {
		out[k] = v
	}
	return out
}
