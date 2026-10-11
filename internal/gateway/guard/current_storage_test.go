package guard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGuardrailsUseOnlyCurrentNamespace 验证废弃单数空间不参与管理或默认执行。
// 前置隔离库中的负向规则，验证列表为空、查询和删除404、默认请求放行；当前空间规则仍拦截，schema 自动清理。
func TestGuardrailsUseOnlyCurrentNamespace(t *testing.T) {
	st := openGuardStore(t)
	h := &savedHost{store: st, allow: true}
	raw := `{"guardrail_id":"retired","guardrail_name":"same-name","litellm_params":{"default_on":true,"guardrail":"always_block"}}`
	if err := st.PutKV("guardrail", "retired", raw); err != nil {
		t.Fatal(err)
	}
	rows, err := storedRules(h)
	if err != nil || len(rows) != 0 {
		t.Fatalf("读取废弃空间: rows=%v err=%v", rows, err)
	}
	if blocked, reason := PreCall(h, map[string]any{"text": "hello"}); blocked {
		t.Fatalf("废弃规则被执行: %s", reason)
	}
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		w := httptest.NewRecorder()
		Manage(h, w, httptest.NewRequest(method, "/guardrails/retired", nil))
		if w.Code != 404 {
			t.Fatalf("%s 读取废弃规则: status=%d body=%s", method, w.Code, w.Body.String())
		}
	}
	putGuardrail(t, st, "current", map[string]any{"guardrail_id": "current", "guardrail_name": "same-name", "litellm_params": map[string]any{"default_on": true, "guardrail": "always_block"}})
	if blocked, _ := PreCall(h, map[string]any{"text": "hello"}); !blocked {
		t.Fatal("当前规则未执行")
	}
}
