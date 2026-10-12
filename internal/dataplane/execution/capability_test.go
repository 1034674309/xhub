// 本文件验证 execution 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package execution

import (
	"net/http"
	"reflect"
	"testing"

	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	dpruntime "github.com/sunqirui1987/xhub/internal/dataplane/runtime"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// TestCapabilitySetsStaySeparate 前置独立能力接口，验证适配、透传和后台刷新只消费各自边界。
// 参数 t 为测试上下文；检查函数类型和接口能力，无网络、持久化或数据清理副作用。
func TestCapabilitySetsStaySeparate(t *testing.T) {
	var adapted func(dpcontract.Adapted, http.ResponseWriter, *http.Request, string) = Serve
	var bypass func(dpcontract.Bypass, http.ResponseWriter, *http.Request, providercontract.Hit) = ServeBypass
	var flush func(dpruntime.Runtime) = dpruntime.Flush
	_, _, _ = adapted, bypass, flush
	adaptedType := reflect.TypeOf((*dpcontract.Adapted)(nil)).Elem()
	bypassType := reflect.TypeOf((*dpcontract.Bypass)(nil)).Elem()
	if _, ok := adaptedType.MethodByName("PinOfficial"); ok {
		t.Fatal("适配接口不应消费原生任务归属")
	}
	if _, ok := bypassType.MethodByName("ResponseCache"); ok {
		t.Fatal("原生透传接口不应消费适配响应缓存")
	}
}
