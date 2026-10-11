package registry

import (
	"strings"

	"github.com/sunqirui1987/xhub/internal/catalog"
)

// ProviderField 是下拉里还没有的供应商要用的一项凭据输入。
type ProviderField struct {
	Key      string
	Label    string
	Type     string
	Required bool
	Default  string
}

// Supplier 是一条下拉项。已有的名字保持原样。
type Supplier struct {
	Name        string
	Slug        string
	Display     string
	Placeholder string
	Fields      []ProviderField
}

// RegisterSupplier 登记展示名称和凭据字段；实际地址必须由连接显式配置。
// 参数 s（Supplier）：要登记的供应商。
// 返回：无。slug 为空时不登记。
// 调用：provider/volcengine/seedance.go
// 测试：无直接单测
func RegisterSupplier(s Supplier) {
	s.Slug = strings.TrimSpace(s.Slug)
	if s.Slug == "" {
		return
	}
	fields := make([]catalog.ProviderField, 0, len(s.Fields))
	for _, f := range s.Fields {
		fields = append(fields, catalog.ProviderField{
			Key: f.Key, Label: f.Label, Type: f.Type, Required: f.Required, Default: f.Default,
		})
	}
	catalog.ContributeProvider(catalog.ProviderRow{
		Name: s.Name, Slug: s.Slug, Display: s.Display, Placeholder: s.Placeholder, Fields: fields,
	})
}
