package identity

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

// strictUserUpdateGate 仅提供已登录身份；其他宿主方法故意为空，保证坏JSON在任何目标读取或副作用前返回。
type strictUserUpdateGate struct{ Gate }

// RequireUser 返回测试管理员；参数为HTTP请求/响应，结果无数据库状态，用于孤立UserUpdate的解析边界。
func (strictUserUpdateGate) RequireUser(http.ResponseWriter, *http.Request) *auth.Principal {
	return &auth.Principal{Kind: authz.KindSession, Role: iam.RoleAdmin, UserID: "admin"}
}

// TestUserUpdateRejectsBrokenJSON 验证完整路由处理器拒绝溢出、尾随和非对象；仅内存HTTP，无数据库可供错误路径调用。
func TestUserUpdateRejectsBrokenJSON(t *testing.T) {
	for _, raw := range []string{`{"user_id":"u","max_budget":1e309}`, `{"user_id":"u","max_budget":-1e309}`, `{"user_id":"u","max_budget":null} {}`, `{"user_id":"u","max_budget":null} broken`, `{"user_id":"u","max_budget":null`, `null`, `[]`, ``} {
		t.Run(raw, func(t *testing.T) {
			w := httptest.NewRecorder()
			UserUpdate(strictUserUpdateGate{}, w, httptest.NewRequest("POST", "/user/update", strings.NewReader(raw)))
			if w.Code != 400 || !strings.Contains(w.Body.String(), `"invalid_request"`) {
				t.Fatalf("非法用户JSON未在写前拒绝: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestUserUpdateJSONBudgetSemantics 验证正常解析后用户patch的缺省、明确null、零和正数；纯函数不写资源。
func TestUserUpdateJSONBudgetSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, raw      string
		present, clear bool
		value          float64
	}{{"absent", `{"user_alias":"用户"}`, false, false, 0}, {"clear", `{"max_budget":null}`, true, true, 0}, {"zero", `{"max_budget":0}`, true, false, 0}, {"positive", `{"max_budget":10}`, true, false, 10}} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := util.ReadRequiredJSONObject(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			v := optionalFloat(body, "max_budget")
			if (v != nil) != tc.present || v != nil && ((*v == nil) != tc.clear || *v != nil && **v != tc.value) {
				t.Fatalf("用户预算语义被改写: %v", v)
			}
		})
	}
}
