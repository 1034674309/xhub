package identity

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestOrganizationConfigFields 验证 HTTP 字段解析；前置内存 JSON 对象，覆盖缺省、清空、正常与非法类型。
// 参数 t 为测试上下文，无返回；确认非法值不会被解释为清空，不写数据，无需清理。
func TestOrganizationConfigFields(t *testing.T) {
	models, meta, err := organizationConfigFields(map[string]any{})
	if err != nil || models != nil || meta != nil {
		t.Fatalf("缺省不应清空: %v %v %v", models, meta, err)
	}
	models, meta, err = organizationConfigFields(map[string]any{"models": nil, "metadata": nil})
	if err != nil || models == nil || len(*models) != 0 || meta == nil || *meta == nil || len(*meta) != 0 {
		t.Fatalf("null 未清空: %v %v %v", models, meta, err)
	}
	models, meta, err = organizationConfigFields(map[string]any{"models": []any{"b", "a", "a"}, "metadata": map[string]any{"department": "研发"}})
	if err != nil || !reflect.DeepEqual(*models, []string{"a", "b"}) || (*meta)["department"] != "研发" {
		t.Fatalf("正常字段解析失败: %v %v %v", models, meta, err)
	}
	for _, body := range []map[string]any{
		{"models": "a"}, {"models": []any{1}}, {"models": []string{" a"}}, {"models": []string{"no-default-models", "a"}},
		{"models": math.NaN()}, {"metadata": "bad"}, {"metadata": []any{}}, {"metadata": true}, {"metadata": math.Inf(1)},
	} {
		if _, _, err := organizationConfigFields(body); !errors.Is(err, iam.ErrInvalid) {
			t.Fatalf("非法配置未拒绝: %v %v", body, err)
		}
	}
}
