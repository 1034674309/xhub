package keys

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/authz"
	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/util"
)

// strictKeyUpdateHost 只提供身份，其余宿主方法为空，确保坏JSON不能进入Key查询或写入路径。
type strictKeyUpdateHost struct{ Host }

// RequireUser 返回已登录管理员测试身份；参数为HTTP请求/响应，无数据库副作用，供Key更新解析测试使用。
func (strictKeyUpdateHost) RequireUser(http.ResponseWriter, *http.Request) *auth.Principal {
	return &auth.Principal{Kind: authz.KindSession, Role: iam.RoleAdmin, UserID: "admin"}
}

// TestKeyUpdateRejectsBrokenJSON 验证实际Key处理器拒绝解码错误、多JSON尾随和非对象；内存HTTP运行，无外部资源。
func TestKeyUpdateRejectsBrokenJSON(t *testing.T) {
	for _, raw := range []string{`{"key":"k","max_budget":1e309}`, `{"key":"k","max_budget":-1e309}`, `{"key":"k","max_budget":null} {}`, `{"key":"k","max_budget":null} broken`, `{"key":"k","max_budget":null`, `null`, `[]`, ``} {
		t.Run(raw, func(t *testing.T) {
			w := httptest.NewRecorder()
			Update(strictKeyUpdateHost{}, w, httptest.NewRequest("POST", "/key/update", strings.NewReader(raw)))
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"invalid_request"`) {
				t.Fatalf("非法Key JSON未在写前拒绝: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestKeyUpdateJSONBudgetSemantics 验证合法对象仍按Key现有patch保留、清空或设置额度；内存Key用后丢弃，无持久副作用。
func TestKeyUpdateJSONBudgetSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		clear     bool
		value     float64
	}{{"absent", `{"key_alias":"原始名称"}`, false, 10}, {"clear", `{"max_budget":null}`, true, 0}, {"zero", `{"max_budget":0}`, false, 0}, {"positive", `{"max_budget":3}`, false, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := util.ReadRequiredJSONObject(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			old := 10.0
			patch, err := patchFrom(&iam.Key{MaxBudget: &old}, body)
			if err != nil {
				t.Fatal(err)
			}
			if (patch.MaxBudget == nil) != tc.clear || patch.MaxBudget != nil && *patch.MaxBudget != tc.value {
				t.Fatalf("Key预算语义被改写: %+v", patch)
			}
		})
	}
}
