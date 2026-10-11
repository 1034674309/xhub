package routing

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

func TestPreferDeploymentMovesThePinnedOneFirst(t *testing.T) {
	pool := []config.ModelEntry{
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://a.example", "model": "m"}},
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://b.example", "model": "m"}},
	}
	got := PreferDeployment(pool, "https://b.example|m")
	if got[0].ParamString("api_base", "") != "https://b.example" {
		t.Fatalf("first deployment %s", got[0].ParamString("api_base", ""))
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
}

func TestPreferDeploymentLeavesAnUnknownPinAlone(t *testing.T) {
	pool := []config.ModelEntry{
		{ModelName: "m", LiteLLMParams: map[string]any{"api_base": "https://a.example", "model": "m"}},
	}
	got := PreferDeployment(pool, "missing")
	if got[0].ParamString("api_base", "") != "https://a.example" {
		t.Fatalf("pool changed: %+v", got)
	}
}
