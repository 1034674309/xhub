package iam

import "testing"

// TestArchiveSchedule 验证系统默认、调度边界及非法参数；无外部前置、写入或清理需求。
func TestArchiveSchedule(t *testing.T) {
	for _, tc := range []struct {
		interval, batch, wantInterval, wantBatch int
		valid                                    bool
	}{
		{0, 0, 2, 20, true}, {2, 1, 2, 1, true}, {86400, 200, 86400, 200, true},
		{1, 20, 1, 20, false}, {-1, 20, -1, 20, false}, {86401, 20, 86401, 20, false},
		{2, -1, 2, -1, false}, {2, 201, 2, 201, false},
	} {
		interval, batch, valid := ArchiveSchedule(tc.interval, tc.batch)
		if interval != tc.wantInterval || batch != tc.wantBatch || valid != tc.valid {
			t.Fatalf("调度参数归一化失败 %+v: %d %d %v", tc, interval, batch, valid)
		}
	}
}
