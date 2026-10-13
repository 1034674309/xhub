package gateway

import (
	"bytes"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/util"
)

// TestWriteChatJSONCacheAdmission 验证响应缓存只写成功完整 JSON 对象及其真实授权范围。
// 前置内存缓存，无数据库；覆盖成功、错误、无效正文、空键和 Responses，私有数据随测试回收。
func TestWriteChatJSONCacheAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, op, key, body string
		status              int
		cached              bool
	}{
		{"success", "chat", "key", "{}", 200, true},
		{"error", "chat", "key", "{}", 500, false},
		{"malformed", "chat", "key", "{", 200, false},
		{"null", "chat", "key", "null", 200, false},
		{"array", "chat", "key", "[]", 200, false},
		{"empty-key", "chat", "", "{}", 200, false},
		{"responses", "responses", "key", "{}", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Cfg: &config.Config{}, Cache: util.NewCache()}
			rec := httptest.NewRecorder()
			s.writeChatJSON(rec, nil, "call", "cache-test-unknown", tc.key, tc.op, "", []byte(tc.body), tc.status, time.Now(), "", "pool", "member")
			hit, ok := s.Cache.Get(tc.key)
			if rec.Code != tc.status || ok != tc.cached {
				t.Fatalf("响应/缓存写入契约错误: status=%d cached=%v", rec.Code, ok)
			}
			if ok && (!bytes.Equal(hit.Body, rec.Body.Bytes()) || !reflect.DeepEqual(hit.Models, []string{"pool", "member"})) {
				t.Fatalf("缓存正文及权限范围错误: %+v", hit)
			}
		})
	}
}
