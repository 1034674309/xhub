package registry_test

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/catalog"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

func TestBypassPathsStayOnTheirProviders(t *testing.T) {
	qiniu, ok := providerregistry.Match("POST", "/v3/contents/generations/tasks")
	if !ok || qiniu.Transport.ID != "qiniu_contents_generation" || qiniu.Transport.ModelField != "model" {
		t.Fatalf("qiniu create %+v %v", qiniu.Transport.ID, ok)
	}
	ark, ok := providerregistry.Match("POST", "/api/v3/contents/generations/tasks")
	if !ok || ark.Transport.ID != "ark_contents_generation" || ark.Transport.TaskID != "id" {
		t.Fatalf("ark create %+v", ark.Transport.ID)
	}
	got, ok := providerregistry.Match("GET", "/api/v3/contents/generations/tasks/cgt-1")
	if !ok || got.Names["id"] != "cgt-1" || got.Action.Name != "get" {
		t.Fatalf("ark poll %+v %+v", got.Action, got.Names)
	}
	if _, ok := providerregistry.Match("POST", "/v1/chat/completions"); ok {
		t.Fatal("adapted chat must not be claimed by bypass")
	}
}

// TestRegisteredTransportsAreTheOnlyBypassSource 钉住 bypass 的来源只有一处：
// 后台登记的转发方式。
//
// 部署上自带的路径表不再参与匹配。这段代码原来覆盖的是一条部署自带文档的自定义
// bypass，现在那条路关掉了，所以要断言它匹配不上。
// 参数 t（*testing.T）：当前测试。
// 返回：无。
func TestRegisteredTransportsAreTheOnlyBypassSource(t *testing.T) {
	if _, ok := providerregistry.Match("GET", "/api/v1/generate/record-info"); ok {
		t.Fatal("a deployment document was accepted as a bypass source")
	}
	if _, ok := providerregistry.Match("POST", "/v2/openapi/task"); ok {
		t.Fatal("a deployment document was accepted as a bypass source")
	}

	if got := providercontract.OfficialID("qiniu", "qiniu/bytedance/doubao-seedance-2-0-260128"); got != "bytedance/doubao-seedance-2-0-260128" {
		t.Fatalf("official id %s", got)
	}
	in, out, ok := catalog.TokenRates("volcengine/doubao-seedance-2-0-260128")
	if ok {
		t.Fatalf("rates %v %v %v", in, out, ok)
	}
	if providerregistry.ModelEndpoints()["volcengine/doubao-seedance-2-0-260128"] != "ark_contents_generation" {
		t.Fatal("model did not preselect the ark endpoint type")
	}
}
