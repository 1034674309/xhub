package util

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// TestReadRequiredJSONObject 验证管理更新JSON的正常、零/null/缺省、错误与尾部读取边界；仅内存输入，不访问外部资源。
// 非完整对象或数字溢出必须返回nil和错误，防止调用方使用部分字段清空额度；合法字段保留原值。
func TestReadRequiredJSONObject(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"positive", `{"max_budget":10}`, true}, {"zero", `{"max_budget":0}`, true}, {"null field", `{"max_budget":null}`, true},
		{"absent", `{"user_alias":"Original 用户"}`, true}, {"empty object", `{}`, true}, {"whitespace", " \n{\"max_budget\":0}\t\n", true},
		{"positive overflow", `{"max_budget":1e309}`, false}, {"negative overflow", `{"max_budget":-1e309}`, false},
		{"partial overflow", `{"user_alias":"changed","max_budget":1e309}`, false}, {"nested overflow", `{"metadata":{"number":1e309}}`, false},
		{"empty", "", false}, {"null root", `null`, false}, {"array", `[]`, false}, {"number", `3`, false}, {"string", `"text"`, false},
		{"truncated", `{"max_budget":null`, false}, {"second object", `{"max_budget":null} {}`, false}, {"second null", `{"max_budget":null} null`, false}, {"trailing garbage", `{"max_budget":null} broken`, false},
		{"exact size", `{}` + strings.Repeat(" ", (1<<20)-2), true}, {"oversized", `{"max_budget":null}` + strings.Repeat(" ", 1<<20), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadRequiredJSONObject(strings.NewReader(tc.raw))
			if (err == nil) != tc.valid || !tc.valid && got != nil {
				t.Fatalf("严格对象判定错误: body=%v err=%v", got, err)
			}
			if tc.name == "zero" && got["max_budget"] != float64(0) {
				t.Fatal("零被改写")
			}
			if tc.name == "null field" {
				if v, ok := got["max_budget"]; !ok || v != nil {
					t.Fatal("明确null未保留")
				}
			}
			if tc.name == "absent" {
				if _, ok := got["max_budget"]; ok {
					t.Fatal("缺省字段被补写")
				}
			}
		})
	}
	t.Run("reader failure", func(t *testing.T) {
		if body, err := ReadRequiredJSONObject(iotest.ErrReader(errors.New("private reader error"))); err == nil || body != nil {
			t.Fatalf("读取失败留下正文: %v %v", body, err)
		}
	})
	t.Run("nil reader", func(t *testing.T) {
		if body, err := ReadRequiredJSONObject(nil); err == nil || body != nil {
			t.Fatal("nil正文未拒绝")
		}
	})
	t.Run("late reader failure", func(t *testing.T) {
		reader := io.MultiReader(strings.NewReader(`{"max_budget":null}`), iotest.ErrReader(errors.New("late private reader error")))
		if body, err := ReadRequiredJSONObject(reader); err == nil || body != nil {
			t.Fatalf("尾部读取失败返回部分对象: %v %v", body, err)
		}
	})

}
