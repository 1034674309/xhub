package runtime

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/live"
)

func TestHotDeltasIncludesProject(t *testing.T) {
	rows := []live.SpendLog{{APIKey: "k", TeamID: "t", UserID: "u", OrgID: "o", ProjectID: "p", Spend: 0.25, SpendValid: true}, {ProjectID: "p", Spend: 100}}
	got := hotDeltas(rows)
	for _, id := range []string{"key/k", "team/t", "user/u", "org/o", "project/p"} {
		if got[id] != 0.25 {
			t.Fatalf("%s delta = %v", id, got[id])
		}
	}
}

// TestQueuedUsageRecordEndpoints 前置内存队列日志，验证成功、失败、未配置端点与非法时间的转换。
// 参数 t 为测试上下文；端点、计时和错误均不得丢失，测试无外部数据，无需清理。
func TestQueuedUsageRecordEndpoints(t *testing.T) {
	for _, status := range []string{"success", "error"} {
		t.Run(status, func(t *testing.T) {
			ttft := 100
			row := live.SpendLog{RequestID: "endpoint", InboundEndpoint: "/v1/messages", OutboundEndpoint: "/v1/responses", Status: status, Error: "upstream error", Start: "2026-10-12T00:00:00Z", End: "2026-10-12T00:00:01Z", TTFTMs: &ttft, Spend: 0.2, SpendValid: true}
			got := queuedUsageRecord(row)
			if got.InboundEndpoint != row.InboundEndpoint || got.OutboundEndpoint != row.OutboundEndpoint || got.DurationMS != 1000 || got.TTFTMs != &ttft || got.Status != status || got.Error != row.Error || got.Cost != 0.2 {
				t.Fatalf("异步落库丢失端点或计量事实: %+v", got)
			}
		})
	}
	got := queuedUsageRecord(live.SpendLog{Start: "invalid", End: "invalid", Spend: 100})
	if got.InboundEndpoint != "" || got.OutboundEndpoint != "" || got.TS.IsZero() || got.EndedAt.IsZero() || got.Cost != 0 {
		t.Fatalf("缺失端点或非法计量不能伪造事实: %+v", got)
	}
}
