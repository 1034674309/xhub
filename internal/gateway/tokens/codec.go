package tokens

import (
	"encoding/json"
	"io"
	"net/http"
)

// str reads v as a string. A non-string returns an empty string and does not panic.
// 参数 v（any）：JSON 里读出的动态值。数字、字符串和对象都要接住，类型不符时按零值而不是 panic。
// 返回 string（string）：按字符串读出的值。不是字符串或没有该键时为空串，不 panic。
// 调用：gateway/access.go、gateway/engine.go、gateway/theme_settings.go、gateway/tokens/tokens.go。
// 测试：无直接单测
func str(v any) string {

	s, _ := v.(string)
	return s
}

// readMap reads a JSON object. An empty body or a parse failure returns an empty map, and the caller decides how to treat a missing field.
// 参数 r（*http.Request）：入站 HTTP 请求，用来读路径、头和正文。
// 返回 map[string]any（map[string]any）：读取表的字段表。缺键表示上游或库里没有这个字段。
// 调用：gateway/access.go、gateway/theme_settings.go、gateway/tokens/tokens.go。
// 测试：无直接单测
func readMap(r *http.Request) map[string]any {
	var body map[string]any
	b, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(b, &body)
	if body == nil {
		body = map[string]any{}
	}
	return body
}
