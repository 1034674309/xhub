package gateway

import (
	"github.com/sunqirui1987/xhub/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUsageSpoolConfiguration 验证省略配置保持兼容，显式错误容量不静默关闭；环境由t.Setenv恢复，无外部依赖。
func TestUsageSpoolConfiguration(t *testing.T) {
	for _, name := range []string{"XHUB_USAGE_SPOOL_DIR", "XHUB_USAGE_SPOOL_MAX_BYTES", "XHUB_USAGE_SPOOL_RECORD_BYTES", "XHUB_USAGE_SPOOL_MAX_ENTRIES"} {
		t.Setenv(name, "")
	}
	if _, enabled, err := usageSpoolConfig(); enabled || err != nil {
		t.Fatal("默认不应启用磁盘队列")
	}
	t.Setenv("XHUB_USAGE_SPOOL_DIR", t.TempDir())
	for _, name := range []string{"XHUB_USAGE_SPOOL_MAX_BYTES", "XHUB_USAGE_SPOOL_RECORD_BYTES", "XHUB_USAGE_SPOOL_MAX_ENTRIES"} {
		t.Run(name, func(t *testing.T) {
			for _, bad := range []string{"0", "-1", "abc", "999999999999999999999999"} {
				t.Setenv(name, bad)
				if _, enabled, err := usageSpoolConfig(); !enabled || err == nil {
					t.Fatalf("非法容量被接受: %s", bad)
				}
			}
			t.Setenv(name, "1024")
			if _, enabled, err := usageSpoolConfig(); !enabled || err != nil {
				t.Fatal("合法容量被拒绝")
			}
		})
	}
}

// TestUsageSpoolAdmissionFailure 验证初始化失败明确阻止新付费请求，关闭安全幂等；前置临时目录，结束自动删除。
func TestUsageSpoolAdmissionFailure(t *testing.T) {
	t.Setenv("XHUB_USAGE_SPOOL_DIR", t.TempDir())
	t.Setenv("XHUB_USAGE_SPOOL_MAX_ENTRIES", "bad")
	s := &Server{}
	s.initializeUsageSpool()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/responses", nil)
	if s.admitUsageSpool(w, r, "call") || w.Code != 503 || !strings.Contains(w.Body.String(), "billing_storage_unavailable") {
		t.Fatal("初始化失败静默绕过持久化准入")
	}
	if err := s.CloseUsageSpool(); err != nil {
		t.Fatal(err)
	}
	s.usageSpoolEnabled = false
	if !s.admitUsageSpool(httptest.NewRecorder(), r, "call") {
		t.Fatal("未配置队列不应改变兼容路径")
	}
}

// TestBrowserUsageSpoolControlBoundaries 验证故障夹具仅接受专属路由/POST/私有schema/专属型号，无数据库或网络副作用。
func TestBrowserUsageSpoolControlBoundaries(t *testing.T) {
	for _, tc := range []struct {
		path, method, body, dsn string
		handled                 bool
		status                  int
	}{
		{"/other", "POST", `{}`, "", false, 200},
		{"/usage-spool/fail", "GET", `{}`, "postgres://unused/?search_path=e2e_test", true, 400},
		{"/usage-spool/fail", "POST", `{"model":"real-model"}`, "postgres://unused/?search_path=e2e_test", true, 400},
		{"/usage-spool/fail", "POST", `{"model":"spool-browser-test"}`, "postgres://unused/?search_path=public", true, 400},
	} {
		t.Run(tc.path+tc.method+tc.dsn, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{GeneralSettings: config.GeneralSettings{DatabaseURL: tc.dsn}}, Client: &http.Client{}}
			handler, cleanup := newBrowserUsageSpoolControl(s, t.TempDir())
			defer cleanup()
			w := httptest.NewRecorder()
			handled := handler(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if handled != tc.handled || w.Code != tc.status {
				t.Fatalf("夹具边界错误: handled=%v status=%d", handled, w.Code)
			}
		})
	}
}
