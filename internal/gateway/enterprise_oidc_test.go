package gateway

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/sunqirui1987/xhub/internal/iam"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestEnterpriseOIDCFlow 使用真实 TLS discovery、JWKS 和 token 路由验证授权码完整回调。
// 前置隔离数据库及 RSA 身份提供方；覆盖 PKCE、nonce、签名、audience、未同步身份与重放；夹具清理服务器和 schema。
func TestEnterpriseOIDCFlow(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	cfg.GeneralSettings.MasterKey = strings.Repeat("k", 32)
	ctx := context.Background()
	org, err := db.CreateOrg(ctx, testActor, "OIDC", nil)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := db.CreateDirectoryConnection(ctx, testActor, org.ID, "OIDC")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ConfigureDirectory(ctx, testActor, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	_, _, err = db.ProvisionDirectoryUser(ctx, c.ID, "employee-sub", "oidc@example.com", "Employee", true)
	if err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := ""
	nonce, challenge, mode := "", "", "valid"
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/token":
			if err := r.ParseForm(); err != nil {
				w.WriteHeader(400)
				return
			}
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != challenge || r.Form.Get("code") != "test-code" {
				w.WriteHeader(400)
				return
			}
			claims := map[string]any{"iss": issuer, "sub": "employee-sub", "aud": "xhub", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "nonce": nonce}
			switch mode {
			case "nonce":
				claims["nonce"] = "wrong"
			case "audience":
				claims["aud"] = "wrong"
			case "unknown":
				claims["sub"] = "unknown"
			case "expired":
				claims["exp"] = time.Now().Add(-time.Minute).Unix()
			case "issuer":
				claims["iss"] = "https://wrong.example"
			}
			signing := key
			if mode == "signature" {
				signing, _ = rsa.GenerateKey(rand.Reader, 2048)
			}
			token := signedEnterpriseToken(t, signing, claims)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "token_type": "Bearer", "expires_in": 60, "id_token": token})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fixture.Close)
	issuer = fixture.URL
	secret, err := enterpriseSecret(cfg.GeneralSettings.MasterKey, "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	oc := iam.EnterpriseOIDC{Name: "TLS login", Issuer: issuer, ClientID: "xhub", Secret: secret, RedirectURI: "https://console.example/auth/enterprise/callback", ReturnURI: "https://console.example/ui/login", IdentityClaim: "sub", Enabled: true}
	if _, err = db.CreateEnterpriseOIDC(ctx, testActor, org.ID, oc); err != nil {
		t.Fatal(err)
	}
	oc.ConnectionID = c.ID
	// 删除同步配置后仍保留稳定身份，完整 TLS 回调必须可以独立工作。
	if err = db.DeleteDirectoryConnection(ctx, testActor, c.ID); err != nil {
		t.Fatal(err)
	}
	s := New(cfg, st, db)
	s.enterpriseClient = fixture.Client()
	for _, testMode := range []string{"valid", "nonce", "audience", "unknown", "expired", "issuer", "signature"} {
		t.Run(testMode, func(t *testing.T) {
			mode = testMode
			req := httptest.NewRequest("GET", "https://console.example/auth/enterprise/"+c.ID+"/start", nil)
			req.SetPathValue("connection_id", c.ID)
			start := httptest.NewRecorder()
			s.enterpriseOIDCStart(start, req)
			if start.Code != 302 {
				t.Fatalf("SSO 启动失败 %d %s", start.Code, start.Body.String())
			}
			location, err := url.Parse(start.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			nonce = location.Query().Get("nonce")
			challenge = location.Query().Get("code_challenge")
			if nonce == "" || challenge == "" || location.Query().Get("code_challenge_method") != "S256" {
				t.Fatal("nonce/PKCE 缺失")
			}
			cookies := start.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly {
				t.Fatal("浏览器绑定 cookie 不安全")
			}
			callback := httptest.NewRequest("GET", "https://console.example/auth/enterprise/callback?state="+url.QueryEscape(location.Query().Get("state"))+"&code=test-code", nil)
			callback.AddCookie(cookies[0])
			out := httptest.NewRecorder()
			s.enterpriseOIDCCallback(out, callback)
			if mode == "valid" {
				if out.Code != 302 || out.Header().Get("Location") != oc.ReturnURI {
					t.Fatalf("完整登录失败 %d %s", out.Code, out.Body.String())
				}
				found := false
				for _, cookie := range out.Result().Cookies() {
					if cookie.Name == "token" && cookie.Value != "" && cookie.Secure {
						found = true
					}
				}
				if !found {
					t.Fatal("会话 cookie 未签发")
				}
			} else if out.Code != 401 {
				t.Fatalf("非法 %s 未拒绝 %d %s", mode, out.Code, out.Body.String())
			}
			replay := httptest.NewRecorder()
			s.enterpriseOIDCCallback(replay, callback)
			if replay.Code != 401 {
				t.Fatal("授权回调重放被接受")
			}
		})
	}
}

// signedEnterpriseToken 为本地身份 fixture 签发 RSA ID token；参数为密钥与 claims，返回紧凑 JWT。
// 仅测试使用，不写日志或数据库，签名失败终止当前测试。
func signedEnterpriseToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	raw := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(raw))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return raw + "." + base64.RawURLEncoding.EncodeToString(sig)
}
