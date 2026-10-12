// 本文件验证 routing 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package routing

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

// TestDropDisabledExcludesDisabled 验证明确停用的部署被剔除，其他部署保序且返回正确剔除数量。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestDropDisabledExcludesDisabled(t *testing.T) {
	active, removed := DropDisabled([]config.ModelEntry{
		{ModelName: "enabled"},
		{ModelName: "disabled", ModelInfo: map[string]any{"disabled": true}},
	})
	if removed != 1 || len(active) != 1 || active[0].ModelName != "enabled" {
		t.Fatalf("active=%#v removed=%d", active, removed)
	}
}
