package util

import (
	"encoding/json"
	"errors"
	"io"
)

// ReadRequiredJSONObject 严格读取一个必需JSON对象；reader为管理更新请求正文，返回完整字段或固定解析错误。
// 用于用户/Key单项更新：任何读取或解码失败（包括数字溢出）都不得返回部分map，null/空正文/尾随第二值也拒绝。
// 管理JSON限1MiB；对象内明确的null和数字零原样保留，字段缺省语义由调用方判断；只消耗正文，不鉴权或写库。
func ReadRequiredJSONObject(reader io.Reader) (map[string]any, error) {
	const maxBody = 1 << 20
	invalid := errors.New("request body must be one valid JSON object no larger than 1 MiB")
	if reader == nil {
		return nil, invalid
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxBody+1))
	if err != nil || len(raw) > maxBody {
		return nil, invalid
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return nil, invalid
	}
	return body, nil
}
