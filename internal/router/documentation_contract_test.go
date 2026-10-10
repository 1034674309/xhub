package router

import (
	"encoding/json"
	"os"
	"testing"
)

// TestDocumentedRouteTemplateExample 验证控制台帮助复制的示例通过后台严格结构校验。
// 前置为仓库内共享 JSON；结果不访问模型供应商或数据库；只读文件，无需清理。
func TestDocumentedRouteTemplateExample(t *testing.T) {
	raw, err := os.ReadFile("../../docs/testdata/route-template-example.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRouteTemplateDocument(document); err != nil {
		t.Fatalf("documented route-template example must pass strict validation: %v", err)
	}
}
