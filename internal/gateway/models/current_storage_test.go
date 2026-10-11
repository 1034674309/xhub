package models

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sunqirui1987/xhub/internal/store"
)

// storedModelHost 为启动装载测试提供真实存储和内存目录；不访问上游，schema 由测试清理。
type storedModelHost struct {
	modelTestHost
	st *store.Store
}

// RecordStore 返回隔离配置库，供 LoadStored 读取；没有额外副作用。
func (h *storedModelHost) RecordStore() *store.Store { return h.st }

// TestLoadStoredDoesNotUpgradeOrRepair 验证重启只装载，不补写字段或清理权重。
// 前置当前结构隔离数据库，覆盖空目录、空元数据及重复装载；验证存储字节不变，schema 自动销毁。
func TestLoadStoredDoesNotUpgradeOrRepair(t *testing.T) {
	st := openPriceStore(t)
	h := &storedModelHost{st: st}
	LoadStored(h)
	if len(h.models) != 0 {
		t.Fatal("空存储不应生成部署")
	}
	for _, row := range []store.ProxyModel{
		{ID: "current", ModelName: "current", Params: map[string]any{"model": "upstream"}, Info: map[string]any{"disabled": true}},
		{ID: "optional-info", ModelName: "optional-info", Params: map[string]any{}, Info: nil},
	} {
		if err := st.UpsertProxyModel(row); err != nil {
			t.Fatal(err)
		}
	}
	defaults := map[string]any{"allocations": []any{map[string]any{"deployment_id": "external", "weight": 1}}}
	if err := st.PutConfig("model_defaults", "current", defaults); err != nil {
		t.Fatal(err)
	}
	before, err := st.ListProxyModels()
	if err != nil {
		t.Fatal(err)
	}
	rawBefore, _ := json.Marshal(before)
	defaultsBefore, err := st.ListConfig("model_defaults")
	if err != nil {
		t.Fatal(err)
	}
	rawDefaultsBefore, _ := json.Marshal(defaultsBefore)
	LoadStored(h)
	LoadStored(h)
	if len(h.models) != 2 {
		t.Fatalf("重复装载追加部署: %#v", h.models)
	}
	for _, entry := range h.models {
		if entry.ModelInfo["db_model"] != true || entry.ModelInfo["id"] == nil {
			t.Fatalf("运行时标记缺失: %#v", entry)
		}
	}
	after, err := st.ListProxyModels()
	if err != nil {
		t.Fatal(err)
	}
	rawAfter, _ := json.Marshal(after)
	if string(rawBefore) != string(rawAfter) {
		t.Fatalf("装载回写数据库: before=%s after=%s", rawBefore, rawAfter)
	}
	saved, err := st.ListConfig("model_defaults")
	rawSaved, _ := json.Marshal(saved)
	if err != nil || string(rawSaved) != string(rawDefaultsBefore) {
		t.Fatalf("装载修改权重: %#v err=%v", saved, err)
	}
	LoadStored(&modelTestHost{}) // 无存储也必须安全返回。
}

// TestPriceOverridesDoNotReadOrMigrateRemovedModels 验证价格装载只读取当前下架状态。
// 前置废弃命名空间的隔离负向夹具，验证不迁移、不生成幽灵模型、不删除原夹具；schema 自动清理。
func TestPriceOverridesDoNotReadOrMigrateRemovedModels(t *testing.T) {
	h := &priceHost{store: openPriceStore(t), allow: true}
	for _, id := range []string{"kimi-k2", "absent-tombstone-test"} {
		if err := h.store.PutConfig("price_removed", id, true); err != nil {
			t.Fatal(err)
		}
	}
	LoadPriceOverrides(h)
	status, body := call(h, http.MethodGet, "/price/catalog", nil)
	row := priceRow(t, body, "kimi-k2")
	if status != 200 || row == nil || row["delisted"] != false || priceRow(t, body, "absent-tombstone-test") != nil {
		t.Fatalf("价格目录读取了废弃删除状态: status=%d row=%v", status, row)
	}
	listings, err := h.store.ListConfig(priceListingNS)
	if err != nil || len(listings) != 0 {
		t.Fatalf("装载迁移了废弃状态: %#v err=%v", listings, err)
	}
	removed, err := h.store.ListConfig("price_removed")
	if err != nil || len(removed) != 2 {
		t.Fatalf("装载清理了非当前存储: %#v err=%v", removed, err)
	}
}
