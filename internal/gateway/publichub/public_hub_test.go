package publichub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
)

// testHost 提供管理会话夹具，拒绝时写 401，不访问身份数据库。
type testHost bool

// RequireManage 以夹具状态返回管理身份或拒绝，参数为 HTTP 上下文，无持久化副作用。
func (h testHost) RequireManage(w http.ResponseWriter, r *http.Request) *auth.Principal {
	if !h {
		w.WriteHeader(401)
		return nil
	}
	return &auth.Principal{}
}

// testStore 模拟链接存储成功或失败，不留下数据库数据。
type testStore bool

// GetKV 读取测试链接；参数为命名空间和键，失败时返回错误。
func (s testStore) GetKV(string, string) (map[string]any, error) {
	if !s {
		return nil, errors.New("offline")
	}
	return map[string]any{}, nil
}

// PutKV 保存测试链接；参数为命名空间、键和正文，失败时返回错误，无外部写入。
func (s testStore) PutKV(string, string, string) error {
	if !s {
		return errors.New("offline")
	}
	return nil
}

// TestPublicHubBoundaries 前置本地链接和目录行，验证鉴权、存储失败、分页上下界、排序及大小写筛选。
// 参数 t 为测试上下文；全程内存测试，无外部数据清理。
func TestPublicHubBoundaries(t *testing.T) {
	for _, tc := range []struct {
		auth, store bool
		status      int
	}{{false, true, 401}, {true, false, 500}, {true, true, 200}} {
		h := handler{host: testHost(tc.auth), store: testStore(tc.store)}
		w := httptest.NewRecorder()
		h.updateUsefulLinks(w, httptest.NewRequest("POST", "/model_hub/update_useful_links", strings.NewReader("{\"useful_links\":{}}")))
		if w.Code != tc.status {
			t.Fatalf("链接更新状态错误: got=%d want=%d", w.Code, tc.status)
		}
	}
	for _, tc := range []struct {
		query      string
		page, size int
	}{{"?page=-1&page_size=0", 1, 20}, {"?page=2&page_size=9999", 2, 100}, {"?page=bad&page_size=bad", 1, 20}} {
		page, size := pageQuery(httptest.NewRequest("GET", "/"+tc.query, nil), 20, 100)
		if page != tc.page || size != tc.size {
			t.Fatalf("分页边界错误: %d %d", page, size)
		}
	}
	rows := []publicModelRow{{ModelGroup: "z", InputCostPerToken: 1.0}, {ModelGroup: "a", InputCostPerToken: 1.0}, {ModelGroup: "b", InputCostPerToken: 2.0}}
	sortPublicModels(rows, "-input_cost_per_token")
	if rows[0].ModelGroup != "b" || rows[1].ModelGroup != "z" {
		t.Fatalf("价格排序或同价顺序错误: %+v", rows)
	}
	if !overlapsFold([]string{"OPENAI"}, []string{"openai"}) || overlapsFold([]string{"anthropic"}, []string{"google"}) {
		t.Fatal("目录筛选大小写处理错误")
	}
}
