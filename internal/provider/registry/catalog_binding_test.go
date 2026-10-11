package registry_test

import (
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	_ "github.com/sunqirui1987/xhub/internal/provider/qiniu"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	_ "github.com/sunqirui1987/xhub/internal/provider/volcengine"
)

// TestCatalogModelsCopy 验证目录副本不允许调用方改变执行权限；前置七牛注册，修改副本后无需清理全局表。
func TestCatalogModelsCopy(t *testing.T) {
	copy := providerregistry.CatalogModels()
	copy["qiniu"]["bytedance/doubao-seedance-2-0-260128"][0] = "forged"
	delete(copy, "volcengine")
	fresh := providerregistry.CatalogModels()
	if fresh["qiniu"]["bytedance/doubao-seedance-2-0-260128"][0] != "qiniu_contents_generation" || fresh["volcengine"] == nil {
		t.Fatal("目录返回值改变了注册表")
	}
}
