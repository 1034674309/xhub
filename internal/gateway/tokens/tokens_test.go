package tokens

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
)

// testHost 提供隔离鉴权和模型列表，全部状态只在本测试内使用。
type testHost struct{ authorized bool }

// RequireMixed 返回测试身份或写 401；参数为 HTTP 上下文，无存储副作用。
func (h testHost) RequireMixed(w http.ResponseWriter, r *http.Request) *auth.Principal {
	if !h.authorized {
		w.WriteHeader(401)
		return nil
	}
	return &auth.Principal{}
}

// Models 返回本地别名映射；无参数，不修改配置。
func (testHost) Models() []config.ModelEntry {
	return []config.ModelEntry{{ModelName: "public", LiteLLMParams: map[string]any{"model": "openai/gpt-4o-mini"}}}
}

// LockModels 模拟单线程夹具加锁，无参数和副作用。
func (testHost) LockModels() {}

// UnlockModels 模拟单线程夹具解锁，无参数和副作用。
func (testHost) UnlockModels() {}

// testStore 提供持久化成功及失败夹具，不连接数据库。
type testStore struct{ failed bool }

// GetKV 返回缺省记录；参数为命名空间和键，错误夹具验证读取降级。
func (s testStore) GetKV(string, string) (map[string]any, error) {
	if s.failed {
		return nil, errors.New("offline")
	}
	return map[string]any{}, nil
}

// PutKV 接收转换结果，失败夹具返回存储错误，不留下数据。
func (s testStore) PutKV(string, string, string) error {
	if s.failed {
		return errors.New("offline")
	}
	return nil
}

// TestTokensBoundary 前置最小 host/store，覆盖鉴权、缺输入、计数、参数缺失和持久化失败。
// 参数 t 为测试上下文；HTTP 处理在内存完成，无需清理外部数据。
func TestTokensBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		auth, fail bool
		op         string
		status     int
	}{
		{"unauthorized", "{}", false, false, "count", 401},
		{"missing_prompt", "{}", true, false, "count", 400},
		{"count", "{\"model\":\"public\",\"prompt\":\"Hello world\"}", true, false, "count", 200},
		{"missing_model", "{}", true, false, "params", 400},
		{"missing_transform_model", "{}", true, false, "transform", 400},
		{"transform", "{\"model\":\"public\"}", true, false, "transform", 200},
		{"store_failure", "{\"model\":\"public\"}", true, true, "transform", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := handler{host: testHost{tc.auth}, store: testStore{tc.fail}}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/utils/test", strings.NewReader(tc.body))
			switch tc.op {
			case "count":
				h.tokenCounter(w, r)
			case "params":
				h.supportedOpenAIParams(w, r)
			default:
				h.transformRequest(w, r)
			}
			if w.Code != tc.status {
				t.Fatalf("工具接口状态错误 got=%d want=%d body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
