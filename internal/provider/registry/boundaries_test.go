package registry

import (
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// TestRegistryPublicSnapshot 验证公开目录结构和副本隔离；前置当前启动注册表，返回值修改不得污染下一次读取。
// 参数 t 为测试上下文；只修改返回副本，无持久化或外部数据需要清理。
func TestRegistryPublicSnapshot(t *testing.T) {
	before := PublicBody()
	for _, key := range []string{"endpoint_types", "capabilities", "transports", "models", "catalogs"} {
		if before[key] == nil {
			t.Fatalf("公开目录缺少 %s", key)
		}
	}
	before["models"].(map[string]string)["forged"] = "forged"
	before["catalogs"].(map[string]map[string][]string)["forged"] = map[string][]string{"x": {"forged"}}
	after := PublicBody()
	if after["models"].(map[string]string)["forged"] != "" || after["catalogs"].(map[string]map[string][]string)["forged"] != nil {
		t.Fatal("公开目录副本污染注册表")
	}
}

// TestRegistryPathBoundaries 验证模板后缀、空参数及 Google 多段模型的匹配边界；纯内存输入，无需清理。
// 参数 t 为测试上下文；成功时参数必须精确保留，失败时不得接受路径穿越或额外段。
func TestRegistryPathBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, pattern, path string
		valid               bool
		names               map[string]string
	}{
		{"empty_template", "", "/tasks", false, nil},
		{"empty_path", "/tasks", "", false, nil},
		{"suffix_mismatch", "/tasks/{id}:get", "/tasks/a:delete", false, nil},
		{"empty_id", "/tasks/{id}:get", "/tasks/:get", false, nil},
		{"dot_id", "/tasks/{id}", "/tasks/.", false, nil},
		{"parent_id", "/tasks/{id}", "/tasks/..", false, nil},
		{"suffix_success", "/tasks/{id}:get/", "/tasks/a:get/", true, map[string]string{"id": "a"}},
		{"google_operation", "/models/{model}:generateContent", "/models/vendor/model:streamGenerateContent", false, nil},
		{"google_empty", "/models/{model}:generateContent", "/models/:generateContent", false, nil},
		{"google_empty_segment", "/models/{model}:generateContent", "/models/vendor//model:generateContent", false, nil},
		{"google_parent_segment", "/models/{model}:generateContent", "/models/vendor/../model:generateContent", false, nil},
		{"google_multi", "/models/{model}:generateContent", "/models/vendor/model:generateContent", true, map[string]string{"model": "vendor/model"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			names, ok := matchPath(tc.pattern, tc.path)
			if ok != tc.valid || ok && !reflect.DeepEqual(names, tc.names) {
				t.Fatalf("模板匹配越界 names=%v ok=%v", names, ok)
			}
		})
	}
}

// TestRegistryRegistrationBoundaries 验证空型号不登记、无供应商映射及 adapted 传输不抢占 bypass。
// 参数 t 为测试上下文；前置临时进程注册表，结束恢复全部修改的注册状态；不写真实供应商目录。
func TestRegistryRegistrationBoundaries(t *testing.T) {
	mu.Lock()
	savedTransports, savedEndpoints, savedCatalogs := transports, modelEndpoints, catalogModels
	transports, modelEndpoints, catalogModels = nil, map[string]string{}, map[string]map[string][]string{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		transports, modelEndpoints, catalogModels = savedTransports, savedEndpoints, savedCatalogs
		mu.Unlock()
	})
	RegisterModel(Model{ID: " "})
	RegisterModel(Model{ID: " local ", TransportID: "adapted", Mode: "chat"})
	RegisterModel(Model{ID: "no-transport"})
	if !reflect.DeepEqual(ModelEndpoints(), map[string]string{"local": "adapted"}) || len(CatalogModels()) != 0 {
		t.Fatal("空型号或供应商污染映射")
	}
	RegisterSupplier(Supplier{Slug: " "})
	// 空名称由目录拒绝，仍验证凭据字段转换且不污染进程目录。
	RegisterSupplier(Supplier{Slug: "local", Fields: []ProviderField{{Key: "api_key", Label: "API key", Type: "password", Required: true, Default: "test"}}})
	RegisterTransport(contract.Transport{ID: "adapted", Kind: contract.KindAdapted, Actions: []contract.Action{{Method: "POST", PublicPath: "/local"}}})
	if _, ok := Match("POST", "/local"); ok {
		t.Fatal("adapted 传输抢占 bypass")
	}
}
