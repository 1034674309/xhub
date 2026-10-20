package identity

import (
	"errors"
	"math"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestBulkUserBudgetFields 验证正常/零/null、缺省和错误目标契约；前置内存JSON对象，解析不写数据，无需清理。
func TestBulkUserBudgetFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  map[string]any
		want  error
		all   bool
		clear bool
		value float64
	}{
		{name: "selected", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": 12.5}}}, value: 12.5},
		{name: "zero", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": float64(0)}}}},
		{name: "clear", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": nil}}}, clear: true},
		{name: "all", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": float64(3)}}, all: true, value: 3},
		{name: "all clear", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": nil}}, all: true, clear: true},
		{name: "empty", body: map[string]any{}, want: iam.ErrInvalid},
		{name: "empty list", body: map[string]any{"users": []any{}}, want: iam.ErrInvalid},
		{name: "all string", body: map[string]any{"all_users": "true", "user_updates": map[string]any{"max_budget": float64(1)}}, want: iam.ErrInvalid},
		{name: "ambiguous scopes", body: map[string]any{"all_users": true, "users": []any{}, "user_updates": map[string]any{"max_budget": float64(1)}}, want: iam.ErrInvalid},
		{name: "updates without all", body: map[string]any{"user_updates": map[string]any{"max_budget": float64(1)}}, want: iam.ErrInvalid},
		{name: "missing budget", body: map[string]any{"users": []any{map[string]any{"user_id": "a"}}}, want: iam.ErrInvalid},
		{name: "blank budget", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": ""}}}, want: iam.ErrInvalid},
		{name: "numeric string", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": "1"}}}, want: iam.ErrInvalid},
		{name: "boolean budget", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": true}}}, want: iam.ErrInvalid},
		{name: "negative", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": float64(-1)}}, want: iam.ErrInvalid},
		{name: "NaN", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": math.NaN()}}, want: iam.ErrInvalid},
		{name: "infinity", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": math.Inf(1)}}, want: iam.ErrInvalid},
		{name: "unsupported role", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": float64(1), "user_role": "admin"}}}, want: iam.ErrInvalid},
		{name: "unsupported metadata", body: map[string]any{"all_users": true, "user_updates": map[string]any{"max_budget": nil, "metadata": nil}}, want: iam.ErrInvalid},
		{name: "unknown envelope", body: map[string]any{"users": []any{}, "unexpected": nil}, want: iam.ErrInvalid},
		{name: "duplicate IDs", body: map[string]any{"users": []any{map[string]any{"user_id": "a", "max_budget": float64(1)}, map[string]any{"user_id": "a", "max_budget": float64(2)}}}, want: iam.ErrInvalid},
		{name: "empty ID", body: map[string]any{"users": []any{map[string]any{"user_id": "", "max_budget": nil}}}, want: iam.ErrInvalid},
		{name: "nonobject user", body: map[string]any{"users": []any{"a"}}, want: iam.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bulkUserBudgetFields(tc.body)
			if !errors.Is(err, tc.want) {
				t.Fatalf("契约错误应为%v，得到%v", tc.want, err)
			}
			if err != nil {
				return
			}
			if got.AllUsers != tc.all {
				t.Fatal("目标范围错误")
			}
			value := got.MaxBudget
			if !tc.all {
				value = got.Users[0].MaxBudget
			}
			if (value == nil) != tc.clear || value != nil && *value != tc.value {
				t.Fatalf("零/清空语义错误: %+v", got)
			}
		})
	}
	t.Run("too many users", func(t *testing.T) {
		users := make([]any, iam.MaxBulkUserBudgets+1)
		_, err := bulkUserBudgetFields(map[string]any{"users": users})
		if !errors.Is(err, iam.ErrBulkUserBudgetTooLarge) {
			t.Fatal(err)
		}
	})
}

// TestReadBulkUserBudgetRequest 验证原始JSON严格解码；前置合法/损坏/溢出/超大正文，部分解码绝不能成为取消上限命令。
// 仅内存输入无外部资源；读取异常和所有JSON异常返回ErrInvalid，合法null/零仍保留契约。
func TestReadBulkUserBudgetRequest(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"selected zero", `{"users":[{"user_id":"a","max_budget":0}]}`, true},
		{"all clear", `{"all_users":true,"user_updates":{"max_budget":null}}`, true},
		{"selected overflow", `{"users":[{"user_id":"a","max_budget":1e309}]}`, false},
		{"all negative overflow", `{"all_users":true,"user_updates":{"max_budget":-1e309}}`, false},
		{"mixed overflow", `{"users":[{"user_id":"a","max_budget":3},{"user_id":"b","max_budget":1e309}]}`, false},
		{"truncated", `{"users":[{"user_id":"a","max_budget":null}`, false},
		{"trailing", `{"all_users":true,"user_updates":{"max_budget":null}} {}`, false},
		{"array", `[]`, false}, {"null", `null`, false}, {"empty", "", false},
		{"oversized", strings.Repeat(" ", 1<<20+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readBulkUserBudgetRequest(strings.NewReader(tc.raw))
			if (err == nil) != tc.valid {
				t.Fatalf("原始正文未严格拒绝/接受: %v", err)
			}
		})
	}
	t.Run("read failure", func(t *testing.T) {
		if _, err := readBulkUserBudgetRequest(iotest.ErrReader(errors.New("fixture read failure"))); !errors.Is(err, iam.ErrInvalid) {
			t.Fatalf("读取异常未拒绝: %v", err)
		}
	})
}
