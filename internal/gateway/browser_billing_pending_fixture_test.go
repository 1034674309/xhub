package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/live"
)

// browserBillingPendingControl 为私有浏览器夹具注入仅限专属模型的账单约束失败，并调用生产 Flush 恢复。
// 参数 w/r 是本机控制请求，s 是无 Redis 的真实网关；返回是否匹配此控制路由，不匹配时不写响应。
// 只编入测试二进制，拒绝非 POST/共享 schema/非专属型号/Redis 模式；数据由浏览器 finally 和运行器清理。
func browserBillingPendingControl(w http.ResponseWriter, r *http.Request, s *Server) bool {
	if !strings.HasPrefix(r.URL.Path, "/billing-pending/") {
		return false
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	var body struct {
		Model string `json:"model"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || !regexp.MustCompile(`^pending-browser-[a-z0-9-]+$`).MatchString(body.Model) || s.Live != nil {
		http.Error(w, "offline model and no Redis required", 400)
		return true
	}
	schema, err := s.IAM.Engine.QueryString("SELECT current_schema() AS schema")
	if err != nil || len(schema) != 1 || !strings.HasPrefix(schema[0]["schema"], "e2e_") {
		http.Error(w, "private browser schema required", 400)
		return true
	}
	switch r.URL.Path {
	case "/billing-pending/fail":
		// 正则仅允许本场景模型标识，DDL 不接受参数；绝不覆盖非测试模型的账单。
		_, err = s.IAM.Engine.Exec("ALTER TABLE usage_events ADD CONSTRAINT browser_pending_reject CHECK (model <> '" + body.Model + "')")
	case "/billing-pending/recover":
		_, err = s.IAM.Engine.Exec("ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS browser_pending_reject")
		if err == nil {
			s.FlushSpend()
		}
	case "/billing-pending/state":
	default:
		http.NotFound(w, r)
		return true
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	n, err := s.IAM.Engine.Table("usage_pending").Where("payload->>'Model' = ?", body.Model).Count()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"pending": n})
	return true
}

// TestBrowserPendingControlBoundaries 验证控制端匹配、方法、模型和 Redis 限制；前置无网络夹具，非法请求不得操作数据库，无需资源清理。
func TestBrowserPendingControlBoundaries(t *testing.T) {
	for _, tc := range []struct {
		path, method, body string
		redis              bool
		status             int
		handled            bool
	}{
		{"/other", "POST", `{}`, false, 200, false},
		{"/billing-pending/fail", "GET", `{}`, false, 405, true},
		{"/billing-pending/fail", "POST", `{}`, false, 400, true},
		{"/billing-pending/fail", "POST", `{"model":"real-model"}`, false, 400, true},
		{"/billing-pending/fail", "POST", `{"model":"pending-browser-unit"}`, true, 400, true},
	} {
		t.Run(tc.path+tc.method+tc.body, func(t *testing.T) {
			s := &Server{}
			if tc.redis {
				s.Live = &live.Client{}
			}
			w := httptest.NewRecorder()
			handled := browserBillingPendingControl(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)), s)
			if handled != tc.handled || w.Code != tc.status {
				t.Fatalf("控制边界失效: handled=%t status=%d", handled, w.Code)
			}
		})
	}
}

// TestPersistSpendPendingSnapshot 验证真实网关直写函数在账单约束失败后保留费用/正文，移除故障并 Flush 后只入账一次。
// 前置私有库、无 Redis、本地构造的已脱敏调用；没有外部请求，连接池和 schema 由 testIdentityStore 清理。
func TestPersistSpendPendingSnapshot(t *testing.T) {
	db := testIdentityStore(t)
	s := &Server{IAM: db}
	if _, err := db.Engine.Exec("ALTER TABLE request_logs ADD CONSTRAINT persist_pending_reject CHECK(request_id <> 'pending-gateway')"); err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC()
	row := live.SpendLog{RequestID: "pending-gateway", Model: "pending-unit", Status: "success", HTTPStatus: 200, Prompt: 8, Completion: 2}
	s.persistSpend(row, 0.12, promptExchange{response: `{"text":"Hello"}`}, start, start.Add(time.Second))
	n, err := db.Engine.Table("usage_pending").Count()
	if err != nil || n != 1 {
		t.Fatalf("直写失败未保留完整账单: n=%d err=%v", n, err)
	}
	if _, err = db.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT persist_pending_reject"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		s.FlushSpend()
	}
	n, err = db.Engine.Table("usage_pending").Count()
	if err != nil || n != 0 {
		t.Fatalf("恢复未确认待结算: n=%d err=%v", n, err)
	}
	log, err := db.GetRequestLog(t.Context(), row.RequestID)
	if err != nil || log.ResponseBody != `{"text":"Hello"}` {
		t.Fatalf("恢复丢失正文: %+v err=%v", log, err)
	}
}
