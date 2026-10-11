package credentials

import (
	"os"
	"strings"
	"sync"

	"github.com/sunqirui1987/xhub/internal/util"
)

// credentialFields 限制命名凭据可复制的字段，防止未知元数据进入部署。
// 只水合当前执行层实际读取的连接字段；Azure/AWS/Vertex 自动认证参数没有对应执行器，不宣称支持。
var credentialFields = []string{"api_key", "api_base", "custom_llm_provider"}

var logTraceOnceCredential sync.Once

// Hydrate 为部署填入可信凭据字段，供 gateway 挂接命名凭据时调用。
// 参数 params 为已有部署参数、credentialValues 为存储凭据；返回新的顶层映射，不修改输入。
// 非空凭据 api_key 替换旧密钥，其余字段只填空值；os.environ/NAME 在调用时展开，协议标识转小写。
// 环境变量不存在时为空，不复制白名单以外字段；调用方负责连接与协议支持校验。
func Hydrate(params, credentialValues map[string]any) map[string]any {
	logTraceOnceCredential.Do(func() { util.Trace("enter credentials.Hydrate") })

	out := map[string]any{}
	for key, value := range params {
		out[key] = value
	}
	if len(credentialValues) > 0 {
		if text, ok := credentialValues["api_key"].(string); ok && strings.TrimSpace(text) != "" {
			delete(out, "api_key")
		}
		for _, key := range credentialFields {
			if !blank(out[key]) {
				continue
			}
			value, ok := credentialValues[key]
			if !ok || blank(value) {
				continue
			}
			out[key] = value
		}
	}
	for key, value := range out {
		text, ok := value.(string)
		if !ok {
			continue
		}
		expanded := expandEnv(text)
		if key == "custom_llm_provider" {
			expanded = strings.ToLower(expanded)
		}
		out[key] = expanded
	}
	return out
}

// expandEnv recognizes os.environ/NAME in a config string. A string without that prefix is returned unchanged. A missing variable becomes an empty string, matching os.Getenv.
// 参数 s（string）：配置里的字符串，可能带 os.environ/ 前缀。
// 返回 string（string）：os.environ/NAME 换成环境变量的值。没有这个前缀时原样返回，变量不存在时为空串。
// 调用：仅在 credential.go 内使用
// 测试：无直接单测
func expandEnv(s string) string {
	s = strings.TrimSpace(s)
	const prefix = "os.environ/"
	if strings.HasPrefix(s, prefix) {
		return os.Getenv(strings.TrimPrefix(s, prefix))
	}
	return s
}

// blank reports whether a credential value is empty. A blank field does not overwrite a value already set on the deployment.
// 参数 v（any）：JSON 里读出的动态值。数字、字符串和对象都要接住，类型不符时按零值而不是 panic。
// 返回 bool（bool）：凭据值是空白时返回真。空白不能覆盖部署上已经写好的值。
// 调用：仅在 credential.go 内使用
// 测试：无直接单测
func blank(v any) bool {
	if v == nil {
		return true
	}
	text, ok := v.(string)
	return ok && strings.TrimSpace(text) == ""
}
