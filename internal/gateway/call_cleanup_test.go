package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestReleaseCall 验证退出清理只删除指定调用的临时事实，已消费、空值和重复清理均安全。
// 参数 t 为上下文；前置纯内存备注、交互与护栏，断言其他调用不受影响；无数据库或文件副作用。
func TestReleaseCall(t *testing.T) {
	s := &Server{exchanges: map[string]promptExchange{"a": {}, "b": {}}, guardrailNotes: map[string]string{"a": "guard", "b": "other"}}
	s.AnnotateCall("a", dpcontract.CallNote{SessionID: "session-a"})
	s.AnnotateCall("b", dpcontract.CallNote{SessionID: "session-b"})
	s.ReleaseCall("")
	s.ReleaseCall("missing")
	s.ReleaseCall("a")
	s.ReleaseCall("a")
	if len(s.callNotes) != 1 || len(s.exchanges) != 1 || len(s.guardrailNotes) != 1 || s.takeNote("b").SessionID != "session-b" {
		t.Fatal("退出清理遗漏临时数据或误删其他调用")
	}
	s.ReleaseCall("b")
	if len(s.callNotes)+len(s.exchanges)+len(s.guardrailNotes) != 0 {
		t.Fatal("已消费调用清理后仍有数据")
	}
	var absent *Server
	absent.ReleaseCall("a")
}

// TestRejectedRequestsReleaseCall 验证真实 HTTP 拒绝路径不会按请求永久保留临时备注。
// 参数 t 为上下文；前置隔离 PostgreSQL、真实用户和 Key，无模型目录，覆盖统一/原生入口的非法会话及无候选错误。
// 每次响应完成后检查实际 Server 内存，不模拟清理；账号、配置和连接由 testGatewayStores 回收，不访问供应商。
func TestRejectedRequestsReleaseCall(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	cfg.ModelList = nil
	user, err := db.CreateUser(context.Background(), testActor, iam.UserInput{Email: "call-cleanup@example.com", Name: "Cleanup", Password: "password123", Role: iam.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := db.CreateKey(context.Background(), iam.Actor{ID: user.ID, Kind: "session"}, iam.KeyInput{UserID: user.ID, Name: "cleanup", TeamID: testTeam(t, db, user.ID), OwnerType: iam.OwnerPersonal})
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, st, db)
	handler := s.Handler()
	for _, path := range []string{"/v1/responses", "/bypass/openai/v1/responses"} {
		for _, body := range []string{`{"model":"missing","input":"Hello","session_id":" "}`, `{"model":"missing","input":"Hello","session_id":42}`, `{"model":"missing","input":"Hello"}`} {
			for i := 0; i < 10; i++ {
				req := httptest.NewRequest("POST", path, strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+key)
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				if w.Code < http.StatusBadRequest || w.Header().Get("x-litellm-call-id") == "" {
					t.Fatalf("未进入真实执行器拒绝路径: %s %d %s", path, w.Code, w.Body.String())
				}
				if strings.Contains(body, "session_id") && (w.Code != 400 || !strings.Contains(w.Body.String(), "invalid session_id")) {
					t.Fatalf("非法会话未在准入前拒绝: %s %d %s", path, w.Code, w.Body.String())
				}
				s.mu.Lock()
				remaining := len(s.callNotes) + len(s.exchanges) + len(s.guardrailNotes)
				s.mu.Unlock()
				if remaining != 0 {
					t.Fatalf("请求退出仍保留 %d 条临时数据: %s", remaining, path)
				}
			}
		}
	}
}
