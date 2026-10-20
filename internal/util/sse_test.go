package util

import (
	"net/http"
	"reflect"
	"testing"
)

// TestPreventSSETransform 验证正常/空值/多值/重复/大小写缓存头，保留敏感响应的既有限制，不误改非缓存头。
// 前置纯内存响应头；调用两次结果幂等，nil 不崩溃，无外部资源与清理。
func TestPreventSSETransform(t *testing.T) {
	for _, tc := range []struct {
		name         string
		before, want []string
	}{
		{"empty", nil, []string{"no-cache, no-transform"}},
		{"blank", []string{""}, []string{"no-cache, no-transform"}},
		{"standard", []string{"no-cache"}, []string{"no-cache, no-transform"}},
		{"private", []string{"private, no-store"}, []string{"private, no-store, no-transform"}},
		{"multiple", []string{"private", "no-store"}, []string{"private, no-store, no-transform"}},
		{"already", []string{"no-cache, no-transform"}, []string{"no-cache, no-transform"}},
		{"case", []string{"NO-TRANSFORM", "no-store"}, []string{"NO-TRANSFORM", "no-store"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{"Cache-Control": tc.before, "Content-Type": []string{"text/event-stream; charset=utf-8"}}
			for range 2 {
				PreventSSETransform(header)
			}
			if !reflect.DeepEqual(header.Values("Cache-Control"), tc.want) || header.Get("Content-Type") != "text/event-stream; charset=utf-8" {
				t.Fatalf("SSE缓存限制丢失或不幂等: %v", header)
			}
		})
	}
	PreventSSETransform(nil)
}
