package volcengine_test

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/catalog"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	_ "github.com/sunqirui1987/xhub/internal/provider/volcengine"
)

func TestSeedanceModelsAndPaths(t *testing.T) {
	typ := find(t, "ark_contents_generation")
	if typ.Kind != providercontract.KindBypass || typ.StripPrefix != "volcengine" || typ.TaskID != "id" || typ.ModelField != "model" {
		t.Fatalf("type %+v", typ)
	}
	if len(typ.Providers) != 1 || typ.Providers[0] != "volcengine" {
		t.Fatalf("providers %v", typ.Providers)
	}
	names := map[string]string{}
	for _, action := range typ.Actions {
		names[action.Name] = action.Method + " " + action.PublicPath
	}
	if names["create"] != "POST /api/v3/contents/generations/tasks" ||
		names["get"] != "GET /api/v3/contents/generations/tasks/{id}" ||
		names["list"] != "GET /api/v3/contents/generations/tasks" {
		t.Fatalf("actions %v", names)
	}
	create, ok := providerregistry.Match("POST", "/api/v3/contents/generations/tasks")
	get, getOK := providerregistry.Match("GET", "/api/v3/contents/generations/tasks/cgt-1")
	list, listOK := providerregistry.Match("GET", "/api/v3/contents/generations/tasks")
	if !ok || !getOK || !listOK || create.Action.Name == get.Action.Name || get.Names["id"] != "cgt-1" || list.Action.Name != "list" {
		t.Fatalf("match create=%s get=%s list=%s", create.Action.Name, get.Action.Name, list.Action.Name)
	}
	if _, ok := providerregistry.Match("POST", "/v3/contents/generations/tasks"); ok {
		t.Fatal("volcengine path matched the qiniu path")
	}
	if providerregistry.ModelEndpoints()["volcengine/doubao-seedance-2-0-260128"] != "ark_contents_generation" ||
		providerregistry.ModelEndpoints()["volcengine/doubao-seedance-2-0-fast-260128"] != "ark_contents_generation" {
		t.Fatal("seedance models did not default to the ark endpoint")
	}
	if got := providercontract.OfficialID("volcengine", "volcengine/doubao-seedance-2-0-260128"); got != "doubao-seedance-2-0-260128" {
		t.Fatalf("official id %s", got)
	}
	in, out, ok := catalog.TokenRates("volcengine/doubao-seedance-2-0-260128")
	if ok {
		t.Fatalf("price %v %v %v", in, out, ok)
	}
	if _, _, ok := catalog.TokenRates("volcengine/doubao-seedance-2-0-fast-260128"); ok {
		t.Fatal("fast has no list price")
	}
}

func find(t *testing.T, id string) providercontract.Transport {
	t.Helper()
	for _, typ := range providerregistry.Transports() {
		if typ.ID == id {
			return typ
		}
	}
	t.Fatalf("missing type %s", id)
	return providercontract.Transport{}
}
