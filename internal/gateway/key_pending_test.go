package gateway

import (
	"math"
	"net/http/httptest"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/live"
)

// TestKeyPendingHeaders 验证流式预发送头和热费用回调按固定 KeyID 读取，而不是当前凭据哈希。
// 前置专用 Redis，覆盖无 Redis、正常累计和失联；故障省略累计头并产生 503，不保留测试状态。
func TestKeyPendingHeaders(t *testing.T) {
	p := &auth.Principal{KeyID: "stable", Hash: "new-hash", Key: &iam.Key{ID: "stable", Spend: 3}}
	s := &Server{}
	w := httptest.NewRecorder()
	s.setChatHeaders(w, p, "model", "base")
	if w.Header().Get("x-litellm-key-spend") != "3" {
		t.Fatalf("无 Redis 累计错误: %s", w.Header().Get("x-litellm-key-spend"))
	}
	client, err := live.Open(browserPrivateRedis(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	s.Live = client
	if err := client.EnqueueSpend(live.SpendLog{RequestID: "old-request", KeyID: "stable", APIKey: "old-hash", Spend: 2, SpendValid: true}); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.setChatHeaders(w, p, "model", "base")
	if w.Header().Get("x-litellm-key-spend") != "5" || s.hotSpendRef("key", "stable") != 2 {
		t.Fatalf("旧凭据消费未带入: %s", w.Header().Get("x-litellm-key-spend"))
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	s.setChatHeaders(w, p, "model", "base")
	if w.Header().Get("x-litellm-key-spend") != "" || !math.IsNaN(s.hotSpendRef("key", "stable")) {
		t.Fatal("不可用费用被静默当作零")
	}
	budgetRefusal(w, "/v1/chat/completions", iam.ErrQuotaUnavailable)
	if w.Code != 503 {
		t.Fatalf("待结算不可读错误状态=%d", w.Code)
	}
}
