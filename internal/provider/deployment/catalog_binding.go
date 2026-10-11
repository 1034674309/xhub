package deployment

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sunqirui1987/xhub/internal/config"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// ValidateCatalogBinding 校验供应商目录与模型的实现白名单，供保存和凭据水合调用。
// 参数 catalogID 来自可信连接，m 为部署；返回错误或 nil，无副作用。
// 目录仅约束已登记专用能力；普通目录模型可以显式选择通用协议，由 ValidateDeployment 校验连接类型和端点。
func ValidateCatalogBinding(catalogID string, m config.ModelEntry) error {
	catalogID = strings.TrimSpace(catalogID)
	if catalogID == "" {
		return nil
	}
	models, ok := providerregistry.CatalogModels()[catalogID]
	if !ok {
		// 中转商目录 ID 是连接元数据，不要求每个商家都实现独立执行器；能力仍由注册传输验证。
		return nil
	}
	model := strings.TrimPrefix(m.ParamString("model", ""), catalogID+"/")
	ids := models[model]
	id := SelectedTransport(m)
	for _, t := range providerregistry.Transports() {
		if t.ID == id && (t.CatalogID == "" || (t.CatalogID == catalogID && slices.Contains(ids, id))) {
			return nil
		}
	}
	return fmt.Errorf("供应商目录 %s 的模型 %s 不支持所选上游协议", catalogID, model)
}
