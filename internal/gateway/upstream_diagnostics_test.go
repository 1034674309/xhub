package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	dpdiagnostics "github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
	"github.com/sunqirui1987/xhub/internal/gateway/requestlog"
	"github.com/sunqirui1987/xhub/internal/live"
)

// TestUpstreamDiagnosticsPersistence 前置原始多值头与计费用量，验证脱敏、附注覆盖及持久化转换。
// 参数 t 为上下文；覆盖无诊断、重试替换、消费清理，不访问数据库或留下数据。
func TestUpstreamDiagnosticsPersistence(t *testing.T) {
	if requestlog.DiagnosticJSON(nil, nil) != "" {
		t.Fatal("旧日志不得伪造上游统计")
	}
	diagnostic := &dpdiagnostics.UpstreamDiagnostics{StatusCode: 200, Headers: http.Header{"Set-Cookie": {"a=secret", "b=secret"}, "X-Request-Id": {"one", "two"}, "X-Debug": {"sk-private"}}, UsageReported: true, Usage: map[string]any{"prompt_tokens_details": map[string]any{"cached_tokens": 12}, "custom": true}}
	s := &Server{}
	s.AnnotateCall("call", dpcontract.CallNote{Upstream: diagnostic})
	s.AnnotateCall("call", dpcontract.CallNote{Provider: "openai"})
	note := s.takeNote("call")
	if note.Upstream != diagnostic || note.Provider != "openai" || s.takeNote("call").Upstream != nil {
		t.Fatal("业务附注覆盖诊断或未清理")
	}
	next := &dpdiagnostics.UpstreamDiagnostics{StatusCode: 502}
	s.AnnotateCall("retry", dpcontract.CallNote{Upstream: diagnostic})
	s.AnnotateCall("retry", dpcontract.CallNote{Upstream: next})
	if s.takeNote("retry").Upstream != next {
		t.Fatal("重试必须保存最后响应")
	}
	raw := requestlog.DiagnosticJSON(diagnostic, map[string]any{"prompt_tokens": 20})
	if strings.Contains(raw, "secret") || strings.Contains(raw, "sk-private") {
		t.Fatalf("凭据未脱敏: %s", raw)
	}
	if diagnostic.Headers.Get("Set-Cookie") != "a=secret" {
		t.Fatal("脱敏修改了上游对象")
	}
	var saved dpdiagnostics.UpstreamDiagnostics
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || len(saved.Headers["Set-Cookie"]) != 2 || saved.Headers["X-Request-Id"][1] != "two" || saved.BillingUsage["prompt_tokens"] != float64(20) {
		t.Fatalf("诊断字段丢失: %s %v", raw, err)
	}
	row := usageFromSpend(live.SpendLog{UpstreamResponse: raw}, 0, "", "", "", time.Now(), time.Now())
	if row.UpstreamResponse != raw {
		t.Fatal("持久化转换丢失诊断")
	}
}

// TestUsageMonitorSnapshot 验证直接写库的队列转换保存供应商、实际模型与流计时；正常和缺失身份均无外部数据。
func TestUsageMonitorSnapshot(t *testing.T) {
	for _, supplier := range []string{"supplier-a", ""} {
		ttft := 15
		row := usageFromSpend(live.SpendLog{Supplier: supplier, UpstreamModel: "actual", Streamed: true, TTFTMs: &ttft}, 0.25, "", "", "", time.Now(), time.Now())
		if row.Supplier != supplier || row.UpstreamModel != "actual" || !row.Streamed || row.TTFTMs == nil || *row.TTFTMs != 15 {
			t.Fatalf("监控快照转换丢失: %+v", row)
		}
	}
}
