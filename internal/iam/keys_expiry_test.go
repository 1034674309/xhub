package iam

import (
	"testing"
	"time"
)

// TestParseExpiryBoundary 验证密钥相对期限只接受可表示的正整数时长。
// 前置条件为纯字符串输入；验证空值、正常未来时间、非正数、格式错误和乘法溢出，测试不写数据库，无需清理。
func TestParseExpiryBoundary(t *testing.T) {
	if got, err := ParseExpiry(""); err != nil || got != nil {
		t.Fatalf("空期限应表示永不过期: got=%v err=%v", got, err)
	}
	before := time.Now().UTC()
	got, err := ParseExpiry("2h")
	if err != nil || got == nil || !got.After(before.Add(119*time.Minute)) || got.After(before.Add(121*time.Minute)) {
		t.Fatalf("正常期限未转换到未来: got=%v err=%v", got, err)
	}
	for _, raw := range []string{"0s", "-1m", "s", "1x", "9223372036854775807h", "999999999999999999999d"} {
		if got, err := ParseExpiry(raw); err == nil || got != nil {
			t.Fatalf("非法期限未拒绝: raw=%q got=%v err=%v", raw, got, err)
		}
	}
}
