package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/qiniu/go-sdk/v7/auth"
	qclient "github.com/qiniu/go-sdk/v7/client"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/qiniu/go-sdk/v7/storagev2/uplog"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/store"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// TestBrowserEnterpriseFixture 启动仅编入测试二进制的 HTTPS 身份/对象服务和控制台代理。
// 参数 t 为测试运行器；仅显式开启时启动，要求私有 e2e schema 和禁用 Redis。
// 浏览器操作真实页面、网关和数据库，归档通过官方 SDK；控制端仅调度 worker/故障。
// 退出关闭服务和数据库，外层脚本删除私有 schema，不使用外部凭据，不改变生产 SSRF 限制。
func TestBrowserEnterpriseFixture(t *testing.T) {
	if os.Getenv("E2E_ENTERPRISE_FIXTURE") != "1" {
		t.Skip("仅供隔离浏览器运行")
	}
	dir := os.Getenv("E2E_RUN_DIR")
	cfg, err := config.Load(filepath.Join(dir, "c.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := url.Parse(cfg.GeneralSettings.DatabaseURL)
	if err != nil || !strings.HasPrefix(dsn.Query().Get("search_path"), "e2e_") || cfg.GeneralSettings.RedisURL != "" {
		t.Fatal("夹具要求私有 e2e schema 且禁用 Redis")
	}
	ctx := context.Background()
	db, err := iam.Open(ctx, cfg.GeneralSettings.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(cfg.GeneralSettings.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Engine.Close()
	s := New(cfg, st, db)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	mode := "valid"
	issuer := ""
	objects := map[string][]byte{}
	codes := map[string]url.Values{}
	// 一次性授权码保存 nonce、PKCE 和用户选择；表单由浏览器提交，服务端不替浏览器触发回调。
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
		case "/authorize":
			w.Header().Set("Content-Type", "text/html")
			if r.Method == "GET" {
				_, _ = fmt.Fprint(w, `<form method="post"><label>Employee<input name="employee" value="employee-sub"></label><button>Sign in to identity provider</button></form>`)
				return
			}
			_ = r.ParseForm()
			code, _ := enterpriseRandom()
			q := r.URL.Query()
			q.Set("sub", r.Form.Get("employee"))
			codes[code] = q
			http.Redirect(w, r, q.Get("redirect_uri")+"?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(q.Get("state")), 302)
		case "/token":
			_ = r.ParseForm()
			// 本地 IdP 拒绝授权码兑换；浏览器必须看到失败，不能建立 XHub 会话。
			if mode == "rejected" {
				http.Error(w, "invalid grant", http.StatusBadRequest)
				return
			}
			q, ok := codes[r.Form.Get("code")]
			delete(codes, r.Form.Get("code"))
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || q.Get("code_challenge_method") != "S256" || base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") || r.Form.Get("redirect_uri") != q.Get("redirect_uri") {
				http.Error(w, "invalid grant", 400)
				return
			}
			claims := map[string]any{"iss": issuer, "sub": q.Get("sub"), "aud": "xhub-browser", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "nonce": q.Get("nonce")}
			if mode == "nonce" {
				claims["nonce"] = "invalid"
			}
			if mode == "unknown" {
				claims["sub"] = "unprovisioned"
			}
			// 签名和身份正确但令牌已过期，验证回调实际校验时间声明。
			if mode == "expired" {
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "local-access", "token_type": "Bearer", "id_token": signedEnterpriseToken(t, key, claims)})
		default:
			w.WriteHeader(404)
		}
	}))
	defer idp.Close()
	issuer = idp.URL
	s.enterpriseClient = idp.Client()
	archiveSDKOnce.Do(uplog.DisableUplog)
	// 对象服务实现七牛 multipart、限定 token、stat 和不可覆盖语义；不以 mock Put 代替 SDK。
	objectServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reqid", "browser-kodo")
		if mode == "archive-fail" {
			http.Error(w, `{"error":"fixture unavailable"}`, 503)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/":
			if err := r.ParseMultipartForm(2 << 20); err != nil {
				http.Error(w, "multipart required", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			objectKey := r.FormValue("key")
			parts := strings.Split(r.FormValue("token"), ":")
			var policy storage.PutPolicy
			if len(parts) != 3 || parts[0] != "fake-ak" {
				w.WriteHeader(401)
				return
			}
			raw, _ := base64.URLEncoding.DecodeString(parts[2])
			if auth.New("fake-ak", "fake-sk").Sign([]byte(parts[2])) != parts[0]+":"+parts[1] || json.Unmarshal(raw, &policy) != nil || policy.Scope != "xhub-test:"+objectKey || policy.InsertOnly != 1 {
				w.WriteHeader(401)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(400)
				return
			}
			defer f.Close()
			body, err := io.ReadAll(f)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			if _, found := objects[objectKey]; found {
				w.WriteHeader(614)
				_, _ = w.Write([]byte(`{"error":"file exists"}`))
				return
			}
			objects[objectKey] = body
			_ = json.NewEncoder(w).Encode(map[string]any{"key": objectKey, "hash": kodoContentHash(body)})
		case strings.HasPrefix(r.URL.Path, "/stat/") || strings.HasPrefix(r.URL.Path, "/delete/"):
			valid, err := auth.New("fake-ak", "fake-sk").VerifyCallback(r)
			if !valid || err != nil {
				w.WriteHeader(401)
				return
			}
			raw, _ := base64.URLEncoding.DecodeString(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			objectKey := strings.TrimPrefix(string(raw), "xhub-test:")
			body, found := objects[objectKey]
			if !found {
				w.WriteHeader(612)
				_, _ = w.Write([]byte(`{"error":"missing"}`))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/delete/") {
				delete(objects, objectKey)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"hash": kodoContentHash(body), "fsize": len(body)})
		default:
			w.WriteHeader(404)
		}
	}))
	defer objectServer.Close()
	host := strings.TrimPrefix(objectServer.URL, "https://")
	// 工厂仅在本测试作用域注入 TLS 端点，秘密仍从真实版本配置解密，生产构造保持原样。
	factory := func(c *iam.ArchiveConfig) (archiveObjectStore, error) {
		actual, err := newKodoArchive(c, cfg.GeneralSettings.MasterKey)
		if err != nil {
			return nil, err
		}
		k := actual.(*kodoArchive)
		if k.bucket != "xhub-test" {
			return nil, fmt.Errorf("fixture bucket required")
		}
		k.cfg = &storage.Config{UseHTTPS: true, Region: &storage.Region{SrcUpHosts: []string{host}, RsHost: host}}
		k.client = &qclient.Client{Client: objectServer.Client()}
		return k, nil
	}
	ui, _ := url.Parse("http://127.0.0.1:" + os.Getenv("E2E_UI_PORT"))
	proxy := httputil.NewSingleHostReverseProxy(ui)
	consoleServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/gw/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/gw")
			s.Handler().ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			s.Handler().ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer consoleServer.Close()
	identities := map[string]string{}
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/session/ttl":
			// 仅缩短当前测试会话的存储 TTL；不删除或撤销凭据，让真实鉴权链等待到期后拒绝。
			var body struct {
				Session string `json:"session"`
			}
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "session required", 400)
				return
			}
			record, err := st.GetKV("ui_session", body.Session)
			if err != nil || record == nil {
				http.Error(w, "fixture session not found", 404)
				return
			}
			expires := time.Now().Add(2 * time.Second)
			record["expires_at"] = expires.UTC().Format(time.RFC3339Nano)
			encoded, err := json.Marshal(record)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err = st.PutKV("ui_session", body.Session, string(encoded)); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"expires_at": expires})
		case "/identity":
			org, err := db.CreateOrg(ctx, testActor, fmt.Sprintf("Browser identity %d", time.Now().UnixNano()), nil)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			c, _, err := db.CreateDirectoryConnection(ctx, testActor, org.ID, "Browser identity")
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err = db.ConfigureDirectory(ctx, testActor, c.ID, true, ""); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			_, user, err := db.ProvisionDirectoryUser(ctx, c.ID, "employee-sub", "browser-"+c.ID+"@example.com", "Browser Employee", true)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			identities[org.ID] = c.ID
			_ = json.NewEncoder(w).Encode(map[string]any{"organization_id": org.ID, "connection_id": c.ID, "user": user})
		case "/identity/cleanup":
			// 仅回收控制端创建的租户；保留锚点是产品语义，测试清理由私有 schema 内事务执行。
			orgID := r.URL.Query().Get("organization_id")
			connectionID, found := identities[orgID]
			if r.Method != "POST" || !found {
				http.Error(w, "fixture identity required", 400)
				return
			}
			tx := db.Engine.NewSession()
			defer tx.Close()
			if err := tx.Begin(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			statements := []string{
				"DELETE FROM enterprise_login_transactions WHERE connection_id=?",
				"DELETE FROM enterprise_login_identities WHERE connection_id=?",
				"DELETE FROM enterprise_oidc WHERE connection_id=?",
				"DELETE FROM directory_team_mappings WHERE connection_id=?",
				"DELETE FROM directory_group_members WHERE connection_id=?",
				"DELETE FROM directory_groups WHERE connection_id=?",
				"DELETE FROM users WHERE id IN (SELECT user_id FROM directory_users WHERE connection_id=?)",
			}
			// 用户外键为 RESTRICT，先读 ID 再清除锚点，最后删除用户和组织。
			var rows []struct{ UserID string }
			if err := tx.SQL("SELECT user_id FROM directory_users WHERE connection_id=?", connectionID).Find(&rows); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			statements = append(statements[:6], "DELETE FROM directory_users WHERE connection_id=?", "DELETE FROM directory_connections WHERE id=?")
			for _, statement := range statements {
				if _, err := tx.Exec(statement, connectionID); err != nil {
					_ = tx.Rollback()
					http.Error(w, err.Error(), 500)
					return
				}
			}
			for _, row := range rows {
				if _, err := tx.Exec("DELETE FROM users WHERE id=?", row.UserID); err != nil {
					_ = tx.Rollback()
					http.Error(w, err.Error(), 500)
					return
				}
			}
			if _, err := tx.Exec("DELETE FROM organizations WHERE id=?", orgID); err != nil {
				_ = tx.Rollback()
				http.Error(w, err.Error(), 500)
				return
			}
			if err := tx.Commit(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			delete(identities, orgID)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/mode":
			mu.Lock()
			mode = r.URL.Query().Get("value")
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/archive/reset":
			// 仅测试二进制且启动时已验证独占 e2e schema；单 worker 场景回收前条测试的队列与内存对象。
			// 不修改产品处理链或业务状态，也不能从生产网关访问，避免伪凭据积压遮蔽当前请求。
			if _, err := db.Engine.Context(ctx).Exec("DELETE FROM archive_events"); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			mu.Lock()
			clear(objects)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/archive/process":
			count, err := s.runArchiveBatch(ctx, 200, factory)
			message := ""
			if err != nil {
				message = err.Error()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"processed": count, "error": message})
		case "/objects":
			mu.Lock()
			defer mu.Unlock()
			_ = json.NewEncoder(w).Encode(objects)
		default:
			w.WriteHeader(404)
		}
	}))
	defer control.Close()
	metadata, _ := json.Marshal(map[string]string{"issuer": issuer, "console": consoleServer.URL, "control": control.URL})
	if err = os.WriteFile(filepath.Join(dir, "enterprise-fixture.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Addr: "127.0.0.1:" + os.Getenv("E2E_GW_PORT"), Handler: s.Handler()}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	<-stop
	shutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
}
