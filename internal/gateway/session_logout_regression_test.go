package gateway

import (
	"encoding/json"
	"net/http"
	"testing"
)

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
