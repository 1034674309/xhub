package execution

import (
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	dptransport "github.com/sunqirui1987/xhub/internal/dataplane/transport"
	"testing"
	"time"
)

// TestApplyStreamTiming 使用固定时刻验证原生流首数据、无数据及同步边界；只修改内存备注，无清理需求。
func TestApplyStreamTiming(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		response dptransport.ForwardResponse
		want     *int
	}{
		{"stream", dptransport.ForwardResponse{Streamed: true, FirstByteAt: start.Add(125 * time.Millisecond)}, ptrMonitor(125)},
		{"no-data", dptransport.ForwardResponse{Streamed: true}, nil},
		{"sync", dptransport.ForwardResponse{}, ptrMonitor(999)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			note := dpcontract.CallNote{TTFTMs: ptrMonitor(999)}
			applyStreamTiming(&note, tc.response, start)
			if note.Streamed != tc.response.Streamed || (note.TTFTMs == nil) != (tc.want == nil) || tc.want != nil && *note.TTFTMs != *tc.want {
				t.Fatalf("实测计时错误: %+v", note)
			}
		})
	}
}

// ptrMonitor 为计时测试建立整数指针，参数为毫秒，返回独立地址，无外部副作用。
func ptrMonitor(n int) *int { return &n }
