package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSessionExpiryBoundary 验证存储 TTL 到期边界；前置私有数据库和真实登录会话。
// 到期前返回 200，过期后 bearer/JWT 均明确 401，过期存储被清理，新登录恢复；夹具回收 schema。
func TestSessionExpiryBoundary(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	server := httptest.NewServer(New(cfg, st, db).Handler())
	defer server.Close()
	session := loginAdmin(t, server.URL, db)
	record, err := st.GetKV("ui_session", session)
	if err != nil {
		t.Fatal(err)
	}
	record["expires_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.PutKV("ui_session", session, string(encoded)); err != nil {
		t.Fatal(err)
	}
	status, body := authed(t, server.URL, session, http.MethodGet, "/auth/me", nil)
	if status != 200 {
		t.Fatalf("到期前会话应可用：%d %s", status, trim(body))
	}
	userID := record["user_id"].(string)
	jwt := signSessionJWT(session, userID, int(record["version"].(float64)), "admin", "chain-admin@example.com", cfg.GeneralSettings.MasterKey)
	// 以已过去的存储期限验证边界，避免后台回归依赖真实等待；浏览器层验证真实时间经过 TTL。
	record["expires_at"] = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	encoded, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.PutKV("ui_session", session, string(encoded)); err != nil {
		t.Fatal(err)
	}
	for _, credential := range []string{session, jwt} {
		status, body = authed(t, server.URL, credential, http.MethodGet, "/auth/me", nil)
		var out struct {
			Error struct{ Type, Code, Message string }
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if status != 401 || out.Error.Type != "invalid_api_key" || out.Error.Code != "401" || out.Error.Message == "" {
			t.Fatalf("到期必须明确拒绝：%d %s", status, trim(body))
		}
	}
	fresh := loginAdmin(t, server.URL, db)
	if fresh == session {
		t.Fatal("重新登录必须创建新会话")
	}
	status, body = authed(t, server.URL, fresh, http.MethodGet, "/auth/me", nil)
	if status != 200 {
		t.Fatalf("重新登录应恢复：%d %s", status, trim(body))
	}
}

// TestLogoutRevokesReplay 验证真实退出路由撤销 bearer 与 JWT，重复退出幂等。
// 前置隔离数据库及成员会话；断言旧凭据明确 401、另一会话可用、重新登录恢复；夹具回收 schema。
func TestLogoutRevokesReplay(t *testing.T) {
	for _, credential := range []string{"bearer", "jwt"} {
		t.Run(credential, func(t *testing.T) {
			c := newChain(t)
			member := c.memberA1
			fresh := loginAs(t, c.f.base, member.user.Email, "password123")
			jwt := signSessionJWT(member.key, member.user.ID, member.user.SessionVersion, member.user.Role, member.user.Email, c.f.masterKey)
			token := member.key
			if credential == "jwt" {
				token = jwt
			}
			for attempt := 0; attempt < 2; attempt++ {
				status, body := authed(t, c.f.base, token, http.MethodPost, "/auth/logout", nil)
				if status != 200 {
					t.Fatalf("退出应幂等成功：%d %s", status, trim(body))
				}
			}
			for _, old := range []string{member.key, jwt} {
				status, body := authed(t, c.f.base, old, http.MethodGet, "/auth/me", nil)
				var out struct {
					Error struct {
						Type    string
						Code    string
						Message string
					}
				}
				if err := json.Unmarshal(body, &out); err != nil {
					t.Fatal(err)
				}
				if status != 401 || out.Error.Type != "invalid_api_key" || out.Error.Code != "401" || out.Error.Message == "" {
					t.Fatalf("已撤销会话必须明确拒绝：%d %s", status, trim(body))
				}
			}
			status, body := authed(t, c.f.base, fresh, http.MethodGet, "/auth/me", nil)
			if status != 200 {
				t.Fatalf("另一独立会话应保持可用：%d %s", status, trim(body))
			}
			recovered := loginAs(t, c.f.base, member.user.Email, "password123")
			if recovered == member.key {
				t.Fatal("重新登录不能恢复被撤销的会话身份")
			}
		})
	}
}
