package regression

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTestdataPersistsActualCredentials 运行真实 Python 建数到隔离后台并核对密钥持久化与脱敏。
// 参数 t 为测试上下文，无返回值；前置空 schema 和测试管理员，验证建数、错误响应及脱敏编辑保留秘密。
// 不调用外部供应商；凭据和所有租户由 harness 销毁 schema，报告由 TempDir 清理。
func TestTestdataPersistsActualCredentials(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	cmd := exec.Command("python3", filepath.Join(root, "scripts/e2e.py"), "--gateway", h.server.URL, "--directory", directory, "--seed-only")
	// 子进程显式覆盖供应商环境，保证宿主凭据不会进入回归数据库。
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "FENNO_AI_") && !strings.HasPrefix(variable, "QINIU_API_") && !strings.HasPrefix(variable, "E2E_DATASET_") {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	cmd.Env = append(cmd.Env, "E2E_DATASET_ADMIN=regression-admin@example.com", "E2E_DATASET_PASSWORD=regression-admin-password", "FENNO_AI_API_KEY=regression-fenno-secret", "QINIU_API_KEY=regression-qiniu-secret")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("真实管理接口建数失败：%v\n%s", err, output)
	}
	for name, secret := range map[string]string{"fennoai": "regression-fenno-secret", "qiniu": "regression-qiniu-secret"} {
		stored, err := h.gw.RecordStore().GetKV("credentials", name)
		if err != nil {
			t.Fatal(err)
		}
		if stored["credential_values"].(map[string]any)["api_key"] != secret {
			t.Fatalf("%s 未保存实际密钥", name)
		}
		listed := h.ok(http.MethodGet, "/credentials", admin, nil)
		if strings.Contains(string(listed.body), secret) {
			t.Fatal("凭据列表泄露秘密")
		}
		var masked any
		for _, row := range listField(listed.json(), "credentials") {
			if row["credential_name"] == name {
				masked = row["credential_values"].(map[string]any)["api_key"]
			}
		}
		h.ok(http.MethodPatch, "/credentials/"+name, admin, map[string]any{"credential_values": map[string]any{"api_key": masked}})
		stored, err = h.gw.RecordStore().GetKV("credentials", name)
		if err != nil || stored["credential_values"].(map[string]any)["api_key"] != secret {
			t.Fatalf("%s 脱敏编辑覆盖真实密钥", name)
		}
	}
	if response := h.do(http.MethodPatch, "/credentials/fennoai", "invalid-session", map[string]any{}); response.status != 401 {
		t.Fatal("非法会话编辑未拒绝")
	}
	report, err := os.ReadFile(filepath.Join(directory, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal(report, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["status"] != "seeded" || len(summary["calls"].([]any)) != 0 {
		t.Fatal("建数状态或零模型调用契约错误")
	}
	for _, name := range []string{"report.json", "access.json"} {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "regression-fenno-secret") || strings.Contains(string(raw), "regression-qiniu-secret") {
			t.Fatal("建数报告泄露供应商秘密")
		}
	}
}
