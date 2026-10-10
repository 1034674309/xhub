package regression

import (
	"net/http"
	"testing"
	"time"
)

// TestKeyExpiryHTTPContract 通过真实密钥接口验证期限创建、单项更新和批量更新的校验及原子性。
// 前置隔离 schema、管理员和两个个人密钥；非法期限必须返回 400 且不创建或修改记录，正期限必须落在请求时刻后的预期窗口；harness 删除整个私有 schema。
func TestKeyExpiryHTTPContract(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()

	beforeInvalid := len(rowsOf(h.ok(http.MethodGet, "/key/list", admin, nil), "keys"))
	for _, duration := range []string{"0s", "-1h", "9223372036854775807d"} {
		r := h.do(http.MethodPost, "/key/generate", admin, map[string]any{
			"key_alias": "invalid-create-" + duration, "duration": duration,
		})
		if r.status != http.StatusBadRequest {
			t.Fatalf("非法创建期限 %q 未返回 400: %s", duration, r.describe())
		}
	}
	if got := len(rowsOf(h.ok(http.MethodGet, "/key/list", admin, nil), "keys")); got != beforeInvalid {
		t.Fatalf("非法创建期限仍写入密钥: before=%d after=%d", beforeInvalid, got)
	}

	first := createKeyWithExpiry(t, h, admin, "expiry-first", "2h", 2*time.Hour)
	second := createKeyWithExpiry(t, h, admin, "expiry-second", "3h", 3*time.Hour)
	firstID, secondID := firstString(first, "token_id"), firstString(second, "token_id")
	firstExpiry, secondExpiry := expiryOf(t, first), expiryOf(t, second)

	badUpdate := h.do(http.MethodPost, "/key/update", admin, map[string]any{
		"key": firstID, "key_alias": "should-not-land", "duration": "0s",
	})
	if badUpdate.status != http.StatusBadRequest {
		t.Fatalf("非法单项期限未返回 400: %s", badUpdate.describe())
	}
	firstAfterBad := keyInfo(t, h, admin, firstID)
	if expiryOf(t, firstAfterBad) != firstExpiry || firstString(firstAfterBad, "key_alias", "key_name") != "expiry-first" {
		t.Fatalf("非法单项期限产生部分更新: %v", firstAfterBad)
	}

	badBulk := h.do(http.MethodPost, "/key/bulk_update", admin, map[string]any{
		"keys": []string{firstID, secondID}, "key_alias": "bulk-should-not-land", "duration": "-2m",
	})
	if badBulk.status != http.StatusBadRequest {
		t.Fatalf("非法批量期限未返回 400: %s", badBulk.describe())
	}
	firstAfterBulk := keyInfo(t, h, admin, firstID)
	secondAfterBulk := keyInfo(t, h, admin, secondID)
	if expiryOf(t, firstAfterBulk) != firstExpiry || expiryOf(t, secondAfterBulk) != secondExpiry ||
		firstString(firstAfterBulk, "key_alias", "key_name") != "expiry-first" || firstString(secondAfterBulk, "key_alias", "key_name") != "expiry-second" {
		t.Fatalf("非法批量期限产生部分更新: first=%v second=%v", firstAfterBulk, secondAfterBulk)
	}

	updateStarted := time.Now().UTC()
	updated := h.ok(http.MethodPost, "/key/update", admin, map[string]any{"key": firstID, "duration": "4h"}).json()
	assertExpiryWindow(t, expiryOf(t, updated), updateStarted, 4*time.Hour)
	bulkStarted := time.Now().UTC()
	bulk := h.ok(http.MethodPost, "/key/bulk_update", admin, map[string]any{"keys": []string{firstID, secondID}, "duration": "5h"}).json()
	if bulk["updated"] != float64(2) {
		t.Fatalf("合法批量期限未更新两把密钥: %v", bulk)
	}
	assertExpiryWindow(t, expiryOf(t, keyInfo(t, h, admin, firstID)), bulkStarted, 5*time.Hour)
	assertExpiryWindow(t, expiryOf(t, keyInfo(t, h, admin, secondID)), bulkStarted, 5*time.Hour)
}

// createKeyWithExpiry 通过 HTTP 创建带期限的密钥并验证响应期限窗口；参数包括别名、期限文本和预期时长，返回创建响应。
func createKeyWithExpiry(t *testing.T, h *harness, admin, alias, duration string, want time.Duration) map[string]any {
	t.Helper()
	started := time.Now().UTC()
	created := h.ok(http.MethodPost, "/key/generate", admin, map[string]any{"key_alias": alias, "duration": duration}).json()
	assertExpiryWindow(t, expiryOf(t, created), started, want)
	return created
}

// keyInfo 通过真实详情接口读取密钥；参数为 harness、管理员会话和公开记录 ID，返回响应对象，私有 schema 由 harness 清理。
func keyInfo(t *testing.T, h *harness, admin, id string) map[string]any {
	t.Helper()
	body := h.ok(http.MethodGet, "/key/info?key="+id, admin, nil).json()
	info, ok := body["info"].(map[string]any)
	if !ok {
		t.Fatalf("密钥详情缺少 info 对象: %v", body)
	}
	return info
}

// expiryOf 解析接口返回的 RFC3339 期限；参数为响应对象，返回 UTC 时间，缺失或格式错误会立即使测试失败。
func expiryOf(t *testing.T, body map[string]any) time.Time {
	t.Helper()
	raw, _ := body["expires"].(string)
	got, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("期限不是 RFC3339: value=%q body=%v", raw, body)
	}
	return got.UTC()
}

// assertExpiryWindow 验证绝对期限落在请求开始后的预期时长附近；允许五秒执行抖动，不依赖数据库时钟。
func assertExpiryWindow(t *testing.T, got, started time.Time, want time.Duration) {
	t.Helper()
	min, max := started.Add(want-2*time.Second), time.Now().UTC().Add(want+2*time.Second)
	if got.Before(min) || got.After(max) {
		t.Fatalf("期限窗口错误: got=%s want=[%s,%s]", got.Format(time.RFC3339), min.Format(time.RFC3339), max.Format(time.RFC3339))
	}
}
