package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/sunqirui1987/xhub/internal/gateway/usagespool"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// browserSpoolTransport 仅在隔离夹具中包装真实本地上游；首次目标推理获得响应头后关闭完整IAM连接池。
// 不改变响应内容或usage，故障发生在准入之后、实际账单落库之前，模拟PG全库无法接受写入。
type browserSpoolTransport struct {
	base http.RoundTripper
	db   *iam.DB
	once sync.Once
}

// RoundTrip 执行真实HTTP后一次性关闭IAM池；错误原样返回，不把网络失败冒充已付费响应。
func (b *browserSpoolTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := b.base.RoundTrip(r)
	if err == nil && strings.HasSuffix(r.URL.Path, "/chat/completions") {
		b.once.Do(func() { _ = b.db.Engine.Close() })
	}
	return resp, err
}

// newBrowserUsageSpoolControl 返回仅测试二进制拥有的本机控制处理器；参数为专属网关和运行目录。
// 只允许私有e2e schema、无Redis和专属模型，关闭真实连接后可重建连接与队列，退出必须调用cleanup。
func newBrowserUsageSpoolControl(s *Server, dir string) (func(http.ResponseWriter, *http.Request) bool, func()) {
	var mu sync.Mutex
	active := false
	original := s.Client.Transport
	spoolDir := filepath.Join(dir, "usage-spool")
	recoverDB := func() error {
		if !active {
			return nil
		}
		db, err := iam.Open(context.Background(), s.Cfg.GeneralSettings.DatabaseURL)
		if err != nil {
			return err
		}
		s.IAM.Engine = db.Engine
		s.Client.Transport = original
		_ = s.CloseUsageSpool()
		s.usageSpool, err = usagespool.Open(usagespool.Config{Dir: spoolDir})
		if err != nil {
			return err
		}
		s.FlushSpend()
		active = false
		return nil
	}
	cleanup := func() { mu.Lock(); defer mu.Unlock(); _ = recoverDB(); _ = s.CloseUsageSpool() }
	handler := func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasPrefix(r.URL.Path, "/usage-spool/") {
			return false
		}
		mu.Lock()
		defer mu.Unlock()
		dsn, err := url.Parse(s.Cfg.GeneralSettings.DatabaseURL)
		var body struct {
			Model string `json:"model"`
		}
		if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&body) != nil || !regexp.MustCompile(`^spool-browser-[a-z0-9-]+$`).MatchString(body.Model) || s.Live != nil || err != nil || !strings.HasPrefix(dsn.Query().Get("search_path"), "e2e_") {
			http.Error(w, "private offline spool fixture required", 400)
			return true
		}
		switch r.URL.Path {
		case "/usage-spool/fail":
			if active {
				http.Error(w, "already armed", 409)
				return true
			}
			if s.usageSpool == nil {
				s.usageSpool, err = usagespool.Open(usagespool.Config{Dir: spoolDir})
				s.usageSpoolEnabled = true
			}
			if err == nil {
				base := original
				if base == nil {
					base = http.DefaultTransport
				}
				s.Client.Transport = &browserSpoolTransport{base: base, db: s.IAM}
				active = true
			}
		case "/usage-spool/recover":
			err = recoverDB()
			if err == nil {
				s.FlushSpend()
			}
		case "/usage-spool/state":
		default:
			http.NotFound(w, r)
			return true
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return true
		}
		if s.usageSpool == nil {
			http.Error(w, "spool not initialized", 409)
			return true
		}
		status, err := s.usageSpool.Status()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"pending": status.Pending, "quarantined": status.Quarantined})
		return true
	}
	return handler, cleanup
}
