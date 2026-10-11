package contract_test

import (
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

func TestReadTaskID(t *testing.T) {
	doc := map[string]any{"data": map[string]any{"taskId": "suno-1"}}
	if got := providercontract.ReadTaskID(doc, "data.taskId"); got != "suno-1" {
		t.Fatalf("task %s", got)
	}
}
