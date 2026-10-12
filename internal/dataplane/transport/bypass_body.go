package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
)

// bypassPart 保存 multipart 原始分段，模型替换不改变文件字节与 MIME 头。
// Header 是完整分段头；Name 是字段名；Data 是未经编码转换的正文。
type bypassPart struct {
	// 原始分段 MIME 头，重建时保留文件名称、类型及扩展头，不包含整个 HTTP 请求头。
	Header textproto.MIMEHeader
	// 表单字段名，用于识别 model 和网关控制字段；文件字段也保留名称。
	Name string
	// 原始分段字节；文件不做文本转换，session_id 控制字段不转发给上游。
	Data []byte
}

// bypassBody 同时保存透传字节和用于路由、用量提取的字段投影。
// Raw 是原始载荷；Fields 是非文件字段；Media、Boundary 保留协议类型与原始边界。
// Parts 保存原始文件和字段顺序，替换模型时只修改对应字段。
type bypassBody struct {
	// 客户端原始正文；模型替换或控制字段过滤时构建新的正文，不原地改写观测内容。
	Raw []byte
	// 路由、会话识别与用量检查的字段投影；JSON 保留类型，multipart 非文件字段为字符串。
	Fields map[string]any
	// 解析后的媒体类型，支持 JSON 和 multipart；空 Content-Type 按 JSON 处理。
	Media string
	// multipart 原始边界，重建分段时复用；JSON 时为空。
	Boundary string
	// 按原始顺序保存的分段；网关字段可移除，其余字段和文件保持原协议。
	Parts []bypassPart
}

// parseBypassBody 解析路由和计价需要的字段，同时保留原始 JSON 与 multipart 文件。
// 参数 raw（[]byte）：已受大小限制的请求体；contentType（string）：原始媒体类型。
// 返回 bypassBody、error：请求字段、原始分段和边界；非法 JSON、边界或重复模型字段返回错误。
// 调用：serveBypassCreate、forwardOfficial。测试：native_bypass_test.go。
func ParseBypassBody(raw []byte, contentType string) (bypassBody, error) {
	b := bypassBody{Raw: raw, Fields: map[string]any{}}
	b.Media, _, _ = mime.ParseMediaType(contentType)
	if b.Media == "" {
		b.Media = "application/json"
	}
	switch b.Media {
	case "application/json":
		if len(bytes.TrimSpace(raw)) > 0 && (json.Unmarshal(raw, &b.Fields) != nil || b.Fields == nil) {
			return b, fmt.Errorf("JSON object required")
		}
	case "multipart/form-data":
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil || params["boundary"] == "" {
			return b, fmt.Errorf("multipart boundary required")
		}
		b.Boundary = params["boundary"]
		reader := multipart.NewReader(bytes.NewReader(raw), b.Boundary)
		for {
			part, err := reader.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return b, fmt.Errorf("invalid multipart body")
			}
			data, err := io.ReadAll(part)
			if err != nil {
				return b, err
			}
			if part.FileName() == "" && part.FormName() != "" {
				if _, exists := b.Fields[part.FormName()]; exists && part.FormName() == "model" {
					return b, fmt.Errorf("duplicate model field")
				}
				b.Fields[part.FormName()] = string(data)
			}
			b.Parts = append(b.Parts, bypassPart{part.Header, part.FormName(), data})
		}
	}
	return b, nil
}

// Payload 替换用于路由的模型名并剥离网关回退开关；路径固定模型则移除请求体中的模型字段。
// 参数 field、model（string）：模型字段名和上游 ID；pathModel（bool）：模型是否已经编码在固定路径中。
// 返回 []byte、error：可发送的原生请求体或序列化错误。JSON 大整数、未知参数、文件字节与边界均保留。
// 调用：serveBypassCreate。测试：TestNativeBypassJSONAndUsage、TestNativeImageEditMultipart。
func (b bypassBody) Payload(field, model string, pathModel bool) ([]byte, error) {
	if field == "" && !pathModel && b.Fields["disable_fallbacks"] == nil && b.Fields["session_id"] == nil {
		return b.Raw, nil
	}
	if pathModel {
		field = "model"
	}
	if b.Media == "application/json" {
		// 用 RawMessage 只替换路由字段，避免 float64 解码后损失大整数精度。
		fields := map[string]json.RawMessage{}
		if len(bytes.TrimSpace(b.Raw)) > 0 {
			if err := json.Unmarshal(b.Raw, &fields); err != nil {
				return nil, err
			}
		}
		// 回退开关属于网关，不能泄露为供应商的未知请求字段。
		delete(fields, "disable_fallbacks")
		delete(fields, "session_id")
		if pathModel {
			delete(fields, field)
		} else if field != "" {
			fields[field], _ = json.Marshal(model)
		}
		return json.Marshal(fields)
	}
	if b.Media == "multipart/form-data" {
		if b.Fields[field] == model && !pathModel && b.Fields["disable_fallbacks"] == nil && b.Fields["session_id"] == nil {
			return b.Raw, nil
		}
		var out bytes.Buffer
		writer := multipart.NewWriter(&out)
		if err := writer.SetBoundary(b.Boundary); err != nil {
			return nil, err
		}
		for _, part := range b.Parts {
			if part.Name == "disable_fallbacks" || part.Name == "session_id" {
				continue
			}
			data := part.Data
			if field != "" && part.Name == field {
				if pathModel {
					continue
				}
				data = []byte(model)
			}
			target, err := writer.CreatePart(part.Header)
			if err != nil {
				return nil, err
			}
			if _, err = target.Write(data); err != nil {
				return nil, err
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	}
	return nil, fmt.Errorf("model field requires JSON or multipart body")
}
