package prefs

import "testing"

// TestPromptStorageCatalog 验证日志页面依赖的字段只登记一次，默认关闭且为布尔类型。
// 前置纯内存目录；缺字段、重复字段或错误默认值均失败，无外部数据需要清理。
func TestPromptStorageCatalog(t *testing.T) {
	count := 0
	for _, field := range generalFieldCatalog() {
		if field.name == "store_prompts_in_spend_logs" {
			count++
			if field.typ != "Boolean" || field.def != false || field.desc == "" {
				t.Fatalf("日志开关的目录契约错误：%+v", field)
			}
		}
	}
	if count != 1 {
		t.Fatalf("日志开关必须唯一登记，实际 %d", count)
	}
}
