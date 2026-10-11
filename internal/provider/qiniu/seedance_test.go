package qiniu_test

import (
	"testing"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	_ "github.com/sunqirui1987/xhub/internal/provider/qiniu"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestQiniuSeedanceKeepsTheBytedancePrefix 验证协议能力仍在，但只能由手工 Custom 凭据选择。
// 参数 t：测试上下文。返回：无；只检查本地注册表，不创建凭据或访问外部供应商。
func TestQiniuSeedanceKeepsTheBytedancePrefix(t *testing.T) {
	typ := providercontract.Transport{}
	for _, item := range providerregistry.Transports() {
		if item.ID == "qiniu_contents_generation" {
			typ = item
		}
	}
	if typ.ID == "" || typ.StripPrefix != "qiniu" {
		t.Fatalf("type %+v", typ)
	}
	if len(typ.Providers) != 3 || typ.Providers[0] != "custom" || typ.Providers[1] != "custom_openai" || typ.Providers[2] != "openai" {
		t.Fatalf("providers %v", typ.Providers)
	}
	create, ok := providerregistry.Match("POST", "/v3/contents/generations/tasks")
	if !ok || create.Transport.ID != "qiniu_contents_generation" || create.Action.Name != "create" {
		t.Fatalf("create %+v %v", create.Transport.ID, ok)
	}
	if _, ok := providerregistry.Match("POST", "/api/v3/contents/generations/tasks"); ok {
		t.Fatal("qiniu registration matched the volcengine path")
	}
	models := []string{
		"bytedance/doubao-seedance-2-0-260128",
		"bytedance/doubao-seedance-2-0-fast-260128",
		"bytedance/doubao-seedance-2-0-mini-260615",
		"bytedance/doubao-seedance-2-5-260628",
	}
	ends := providerregistry.ModelEndpoints()
	for _, id := range models {
		key := "qiniu/" + id
		if ends[key] != "qiniu_contents_generation" {
			t.Fatalf("%s default %s", key, ends[key])
		}
		if got := providercontract.OfficialID("qiniu", key); got != id {
			t.Fatalf("official %s -> %s", key, got)
		}
	}
}
