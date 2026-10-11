package deployment

import (
	"strings"

	"github.com/sunqirui1987/xhub/internal/config"
	providercapability "github.com/sunqirui1987/xhub/internal/provider/capability"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// 把动态值收成去掉空白的字符串。不是字符串时为空串。
// 参数 v（any）：JSON 里读出的动态值。
// 返回 string（string）：按字符串读出的值。
// 调用：仅在 registry.go 内使用
// 测试：无直接单测
func str(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// SelectedCapabilities 返回一条部署应答的能力 id 列表。
//
// 从 model_info.endpoint_types 读取能力 id；字段缺失时不声明任何能力。
//
// 认不出的 id（realtime、batch、ocr）被忽略，不放进任何能力。一项都认不出时
// 返回空切片，调用方据此拒绝这条部署——不在这里补默认 chat，那会让一条只写了
// realtime 的部署意外应答所有对话请求。
//
// 参数 m（config.ModelEntry）：一条部署。
// 返回 []string（[]string）：能力 id 列表，可能为空。
// 调用：AdaptedPool 过滤适配池。
// 测试：registry_test.go
func SelectedCapabilities(m config.ModelEntry) []string {
	if m.ModelInfo == nil {
		return nil
	}
	if list := stringList(m.ModelInfo["endpoint_types"]); len(list) > 0 {
		return normalizeCapabilities(list)
	}
	return nil
}

// normalizeCapabilities 把一批能力 id 去重并保持顺序，未知 id 丢弃。
// 参数 list（[]string）：要收的 id。
// 返回 []string（[]string）：能力 id 列表。
// 调用：SelectedCapabilities。
// 测试：registry_test.go
func normalizeCapabilities(list []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range list {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !providercapability.KnownEndpoint(id) || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// SelectedTransport 返回一条部署的转发方式 id。
//
// model_info.transport 必须是登记过的内置 id，否则返回空串。
//
// 内置 Bypass 之外的 bypass 形状不存在：后台没登记过的转发方式，运维在界面上
// 也选不到、存不进。
//
// 参数 m（config.ModelEntry）：一条部署。
// 返回 string（string）：转发方式 id，未配置或无效时为空。
// 调用：AdaptedPool 过滤适配池；dataplane/official.go 选部署。
// 测试：registry_test.go
func SelectedTransport(m config.ModelEntry) string {
	if m.ModelInfo == nil {
		return ""
	}
	if id := str(m.ModelInfo["transport"]); id != "" {
		if _, ok := transportByID(id); ok {
			return id
		}
	}
	return ""
}

// transportByID 只接受当前登记的 bypass transport。
// 参数 id（string）：待查找的传输方式标识。
// 返回 string、bool：登记的 ID 与是否存在。
// 调用：SelectedTransport。
// 测试：registry_test.go。
func transportByID(id string) (string, bool) {
	for _, transport := range providerregistry.Transports() {
		if transport.ID == id {
			return id, true
		}
	}
	return "", false
}

// IncludesCapability 报告这条部署是否应答这个能力。
// 参数 m（config.ModelEntry）：一条部署；capability（string）：能力 id。
// 返回 bool（bool）：应答这个能力时为真。
// 调用：AdaptedPool。
// 测试：registry_test.go
func IncludesCapability(m config.ModelEntry, capability string) bool {
	for _, id := range SelectedCapabilities(m) {
		if id == capability {
			return true
		}
	}
	return false
}

// Includes 报告这条部署是否选中了这个转发方式 id。
// 参数 m（config.ModelEntry）：一条部署；typeID（string）：转发方式 id。
// 返回 bool（bool）：选中时为真。
// 调用：dataplane/official.go
// 测试：registry_test.go
func Includes(m config.ModelEntry, typeID string) bool {
	for _, t := range providerregistry.Transports() {
		if t.ID == typeID {
			return AllowsEndpoint(m, t.EndpointID) == nil
		}
	}
	return false
}

// BoundTransports 把这条部署声明的转发方式解析成登记好的条目。
// 参数 m（config.ModelEntry）：一条部署。
// 返回 []Transport（[]Transport）：这条部署应答的内置转发方式。
// 调用：测试。
// 测试：registry_test.go
func BoundTransports(m config.ModelEntry) []providercontract.Transport {
	if ValidateDeployment(m) != nil {
		return nil
	}
	for _, t := range providerregistry.Transports() {
		if t.ID == SelectedTransport(m) {
			return []providercontract.Transport{t}
		}
	}
	return nil
}

// 把配置里的字符串或数组收成 []string。
// 参数 v（any）：JSON 里读出的动态值。
// 返回 []string（[]string）：字符串列表。
// 调用：SelectedCapabilities、SelectedTransport。
// 测试：无直接单测
func stringList(v any) []string {
	switch list := v.(type) {
	case []string:
		return list
	case []any:
		var out []string
		for _, item := range list {
			if s := str(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
