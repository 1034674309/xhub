package billing

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBudgetSnapshotReconciliationSource 验证严格预占冻结本attempt连接与型号且不复制供应商凭据。
// 前置纯内存部署；配置之后变更不改已生成快照，后续人工核对不必猜最终重试连接，无外部资源。
func TestBudgetSnapshotReconciliationSource(t *testing.T) {
	dep := strictBudgetDeployment()
	dep.LiteLLMParams["litellm_credential_name"] = "synthetic-connection"
	dep.LiteLLMParams["api_key"] = "secret-not-for-snapshot"
	plan, _ := strictBudgetMustPlan(t, dep, strictBudgetBody("openai-chat"), "openai-chat")
	dep.LiteLLMParams["litellm_credential_name"] = "changed-connection"
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(plan.PriceSnapshot), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["supplier"] != "synthetic-connection" || snapshot["model"] != "private-strict-budget-test" || strings.Contains(plan.PriceSnapshot, "secret-not-for-snapshot") {
		t.Fatalf("预占来源未冻结或泄露密钥: %#v", snapshot)
	}
}
