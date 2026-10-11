package usage

import (
	"reflect"
	"testing"
)

func TestCompleteUsagePreservesReportedZeroAndCacheInput(t *testing.T) {
	usage := map[string]any{
		"input_tokens":                float64(0),
		"output_tokens":               float64(0),
		"cache_read_input_tokens":     float64(19),
		"cache_creation_input_tokens": float64(3),
	}
	got := CompleteUsage(usage, map[string]any{"messages": []any{map[string]any{"content": "estimate me"}}}, []byte("data: {\"choices\":[{\"delta\":{\"content\":\"text\"}}]}\n"))
	if !reflect.DeepEqual(got, usage) {
		t.Fatalf("explicit usage changed: got %#v want %#v", got, usage)
	}
}

func TestEstimateTokensDoesNotCountOutputLimitAsInput(t *testing.T) {
	without := EstimateTokens(map[string]any{"prompt": "abcd"})
	with := EstimateTokens(map[string]any{"prompt": "abcd", "max_tokens": 100000})
	if with != without {
		t.Fatalf("max_tokens changed input estimate: with=%d without=%d", with, without)
	}
}
