// 本文件验证 transport 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"testing"

	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestNativeSupplierURLs 验证任何域名都只遵循配置地址与显式传输，模型前缀由所选传输处理。
// 参数 t：测试上下文。返回：无；只组合本地声明，不访问供应商。
// 测试只操作内存配置或本地假上游，HTTP 服务及正文由本例关闭，不创建外部付费任务或持久化数据。
func TestNativeSupplierURLs(t *testing.T) {
	for _, spec := range []struct{ base, path, wantURL, model, wantModel string }{
		{"https://api.qnaigc.com/v1", "/bypass/openai/v1/responses", "https://api.qnaigc.com/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://api.modelink.ai/custom/v1", "/bypass/openai/v1/responses", "https://api.modelink.ai/custom/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://api.openai.com/v1", "/bypass/openai/v1/responses", "https://api.openai.com/v1/responses", "openai/gpt-test", "openai/gpt-test"},
		{"https://configured.example/bypass/openai/v1", "/bypass/openai/v1/responses", "https://configured.example/bypass/openai/v1/responses", "openai/gpt-test", "openai/gpt-test"},
	} {
		t.Run(spec.wantURL, func(t *testing.T) {
			hit, ok := providerregistry.Match("POST", spec.path)
			if !ok {
				t.Fatal("unregistered native path", spec.path)
			}
			if got := BypassDeploymentURL(spec.base, hit); got != spec.wantURL {
				t.Errorf("URL = %s, want %s", got, spec.wantURL)
			}
			if got := providercontract.OfficialID(hit.Transport.StripPrefix, spec.model); got != spec.wantModel {
				t.Errorf("model = %s, want %s", got, spec.wantModel)
			}
		})
	}
}

// TestBypassSessionFields 验证原生 JSON 和 multipart 在选路后移除 session_id，保留原生缓存键和文件。
// 前置内存载荷，检查解析投影仍含会话信号、重建正文不泄露网关字段；非法 JSON 返回错误，无外部数据。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
// 测试只操作内存配置或本地假上游，HTTP 服务及正文由本例关闭，不创建外部付费任务或持久化数据。
func TestBypassSessionFields(t *testing.T) {
	raw := []byte(`{"model":"public","session_id":"session","prompt_cache_key":"native-cache","extra":true}`)
	body, err := ParseBypassBody(raw, "application/json")
	if err != nil || body.Fields["session_id"] != "session" {
		t.Fatalf("JSON 会话投影丢失: %v", err)
	}
	encoded, err := body.Payload("model", "upstream", false)
	var doc map[string]any
	if err != nil || json.Unmarshal(encoded, &doc) != nil {
		t.Fatalf("JSON 重建失败: %v", err)
	}
	if _, exists := doc["session_id"]; exists {
		t.Fatal("JSON 泄露网关会话字段")
	}
	if doc["model"] != "upstream" || doc["prompt_cache_key"] != "native-cache" || doc["extra"] != true {
		t.Fatalf("原生字段被改写: %v", doc)
	}
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	_ = writer.WriteField("model", "public")
	_ = writer.WriteField("session_id", "session")
	_ = writer.WriteField("prompt_cache_key", "native-cache")
	file, _ := writer.CreateFormFile("file", "sample.bin")
	_, _ = file.Write([]byte{0, 1, 255})
	_ = writer.Close()
	body, err = ParseBypassBody(payload.Bytes(), writer.FormDataContentType())
	if err != nil || body.Fields["session_id"] != "session" {
		t.Fatalf("表单会话投影丢失: %v", err)
	}
	encoded, err = body.Payload("model", "upstream", false)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(encoded), body.Boundary)
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(part)
		fields[part.FormName()] = string(data)
	}
	if _, exists := fields["session_id"]; exists {
		t.Fatal("表单泄露网关会话字段")
	}
	if fields["model"] != "upstream" || fields["prompt_cache_key"] != "native-cache" || fields["file"] != string([]byte{0, 1, 255}) {
		t.Fatalf("表单原生字段或文件被改写: %v", fields)
	}
	if _, err := ParseBypassBody([]byte("{"), "application/json"); err == nil {
		t.Fatal("非法 JSON 未拒绝")
	}
}
