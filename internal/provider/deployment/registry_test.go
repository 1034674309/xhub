package deployment_test

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providerdeployment "github.com/sunqirui1987/xhub/internal/provider/deployment"
)

// TestSelectedCapabilitiesUsesDeclaredIDs 验证部署能力只来自显式 endpoint_types。
// 参数 t（*testing.T）：当前测试上下文。
// 返回：无。失败时说明能力声明被错误推断或放宽。
func TestSelectedCapabilitiesUsesDeclaredIDs(t *testing.T) {
	newRow := config.ModelEntry{ModelInfo: map[string]any{"endpoint_types": []any{"chat", "embedding"}}}
	if got := providerdeployment.SelectedCapabilities(newRow); len(got) != 2 || got[0] != "chat" || got[1] != "embedding" {
		t.Fatalf("new spelling selected %v", got)
	}

	unknown := config.ModelEntry{ModelInfo: map[string]any{"endpoint_types": []any{"image"}}}
	if got := providerdeployment.SelectedCapabilities(unknown); len(got) != 0 {
		t.Fatalf("unknown capability selected %v", got)
	}
	modeOnly := config.ModelEntry{ModelInfo: map[string]any{"mode": "embedding"}}
	if got := providerdeployment.SelectedCapabilities(modeOnly); len(got) != 0 {
		t.Fatalf("mode-only row selected %v", got)
	}
	// 空配置不声明任何能力；新系统不从缺失字段推断对话能力。
	plain := config.ModelEntry{}
	if got := providerdeployment.SelectedCapabilities(plain); len(got) != 0 {
		t.Fatalf("an empty model info selected %v, want no capabilities", got)
	}
}

func TestTransportRequiresExplicitRegisteredID(t *testing.T) {
	row := config.ModelEntry{ModelInfo: map[string]any{"mode": "ark_contents_generation", "endpoint_types": []string{"ark_contents_generation"}}}
	if got := providerdeployment.SelectedTransport(row); got != "" {
		t.Fatalf("unselected transport=%q", got)
	}
	row.ModelInfo["transport"] = "ark_contents_generation"
	if got := providerdeployment.SelectedTransport(row); got != "ark_contents_generation" {
		t.Fatalf("explicit transport=%q", got)
	}
}

func TestOneModelCanAnswerSeveralCapabilities(t *testing.T) {
	both := config.ModelEntry{ModelInfo: map[string]any{"endpoint_types": []any{"chat", "embedding"}}}
	if !providerdeployment.IncludesCapability(both, "chat") || !providerdeployment.IncludesCapability(both, "embedding") {
		t.Fatal("a model with two capabilities did not report both")
	}
	if providerdeployment.IncludesCapability(both, "image") {
		t.Fatal("a model answered a capability it never declared")
	}
}
