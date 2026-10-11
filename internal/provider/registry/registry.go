package registry

import (
	"maps"
	"slices"
	"strings"
	"sync"

	providercapability "github.com/sunqirui1987/xhub/internal/provider/capability"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	"github.com/sunqirui1987/xhub/internal/util"
)

var (
	mu             sync.Mutex
	transports     []providercontract.Transport
	modelEndpoints = map[string]string{}
	catalogModels  = map[string]map[string][]string{}
)

// RegisterTransport 登记一个内置转发方式。供应商文件在 init 里调它。
// 参数 t（Transport）：要登记的内置转发方式。
// 返回：无。id 为空或没有动作时不登记——一个没有动作的转发方式无处可转。
// 调用：provider/volcengine/seedance.go
// 测试：seedance_test.go
func RegisterTransport(t providercontract.Transport) {
	t.ID = strings.TrimSpace(t.ID)
	t.Kind = providercontract.Kind(strings.TrimSpace(string(t.Kind)))
	if t.ID == "" || len(t.Actions) == 0 {
		return
	}
	t = cloneTransport(t)
	for i := range t.Actions {
		t.Actions[i].Method = strings.ToUpper(strings.TrimSpace(t.Actions[i].Method))
	}
	mu.Lock()
	transports = append(transports, t)
	mu.Unlock()
}

// Transports 返回已登记的内置转发方式。
// 参数：无。
// 返回 []Transport（[]Transport）：已登记的转发方式。
// 调用：PublicBody。
// 测试：seedance_test.go
func Transports() []providercontract.Transport {
	mu.Lock()
	defer mu.Unlock()
	out := make([]providercontract.Transport, len(transports))
	for i, t := range transports {
		out[i] = cloneTransport(t)
	}
	return out
}

// PublicBody 是添加模型的载荷：公开能力、执行传输和模型默认值。
//
// endpoint_types 是公开协议能力；capabilities 是能力与路径说明；
// transports 是后端登记的执行传输；models 是模型到默认 transport 的映射。
// 参数：无。
// 返回 map[string]any（map[string]any）：能力、转发方式和模型默认端点。
// 调用：catalog/classify.go、gateway/bypass.go、gateway/catalog.go
// 测试：无直接单测
func PublicBody() map[string]any {
	return map[string]any{
		"endpoint_types": providercapability.EndpointTypes(),
		"capabilities":   providercapability.Capabilities(),
		"transports":     Transports(),
		"models":         ModelEndpoints(),
		"catalogs":       CatalogModels(),
	}
}

// init 记一次目录载入。目录是进程启动时由各供应商包的 init 填好的，
// 这一行是"目录确实装上了"的凭证。
// 参数：无。
// 返回：无。只写一行进程日志。
// 调用：Go 在载入这个包时自动执行。
// 测试：无直接单测
func init() { util.Debug("provider registry loaded") }

// cloneTransport 隔离注册与读取边界上的可变配置，供 RegisterTransport 和 Transports 调用。
// 参数 t 为调用方传输；返回独立的切片、映射及计量回调容器；函数值保留，无外部副作用。
func cloneTransport(t providercontract.Transport) providercontract.Transport {
	t.Actions = slices.Clone(t.Actions)
	t.Providers = slices.Clone(t.Providers)
	t.Headers = maps.Clone(t.Headers)
	t.SupplierPrefixes = maps.Clone(t.SupplierPrefixes)
	if t.Billing != nil {
		billing := *t.Billing
		t.Billing = &billing
	}
	return t
}
