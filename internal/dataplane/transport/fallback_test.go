// 本文件验证 transport 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package transport

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

// TestFallbackNativePayload 验证网关开关剥离及大整数原样转发；前置原生 JSON，检查未知字段和模型替换，无资源清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestFallbackNativePayload(t *testing.T) {
	b, err := ParseBypassBody([]byte("{\"model\":\"a\",\"disable_fallbacks\":true,\"value\":9007199254740993}"), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Payload("model", "b", false)
	if err != nil || strings.Contains(string(raw), "disable_fallbacks") || !strings.Contains(string(raw), "9007199254740993") {
		t.Fatalf("原生正文损坏: %s %v", raw, err)
	}
	raw, err = b.Payload("", "unused", false)
	var fields map[string]json.RawMessage
	if err != nil || json.Unmarshal(raw, &fields) != nil || fields[""] != nil || fields["disable_fallbacks"] != nil || string(fields["model"]) != `"a"` {
		t.Fatalf("无模型字段时只应剥离开关: %s %v", raw, err)
	}
}

// TestFallbackPayloadMultipart 验证无模型字段的文件请求只剥离回退开关，保留文件字节与名称。
// 参数 t 为测试上下文，无返回；前置内存 multipart，不访问上游，不产生需清理的数据。
func TestFallbackPayloadMultipart(t *testing.T) {
	var raw bytes.Buffer
	writer := multipart.NewWriter(&raw)
	writer.WriteField("disable_fallbacks", "true")
	file, err := writer.CreateFormFile("file", "data.bin")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte{0, 255, 13, 10}
	file.Write(data)
	writer.Close()
	body, err := ParseBypassBody(raw.Bytes(), writer.FormDataContentType())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := body.Payload("", "unused", false)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(payload), writer.Boundary())
	part, err := reader.NextPart()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(part)
	if err != nil || part.FormName() != "file" || part.FileName() != "data.bin" || !bytes.Equal(got, data) {
		t.Fatalf("原生文件损坏: 字段%s 文件%s 正文%v 错误%v", part.FormName(), part.FileName(), got, err)
	}
	if _, err := reader.NextPart(); err != io.EOF {
		t.Fatalf("回退开关未删除: %v", err)
	}
}
