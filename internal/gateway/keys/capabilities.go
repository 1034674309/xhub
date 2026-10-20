package keys

import "fmt"

// validateKeyCapabilities 校验创建、编辑和轮换请求的能力声明；参数为 JSON 请求，返回可直接展示的字段错误。
// XHub 当前只发行模型调用凭据，管理操作使用登录会话；空值表示未设置，false 表示关闭自动轮换。
// 非空不支持能力必须拒绝，不能静默扩大权限或承诺不存在的调度；本函数无持久化副作用。
func validateKeyCapabilities(body map[string]any) error {
	if raw := body["duration"]; raw != nil {
		if _, ok := raw.(string); !ok {
			return fmt.Errorf("duration must be a positive duration string")
		}
	}
	if raw := body["key_type"]; raw != nil {
		value, ok := raw.(string)
		if !ok || (value != "" && value != "llm_api") {
			return fmt.Errorf("key_type supports only llm_api")
		}
	}
	if raw := body["auto_rotate"]; raw != nil {
		value, ok := raw.(bool)
		if !ok || value {
			return fmt.Errorf("auto_rotate is not supported; use manual rotation")
		}
	}
	for _, field := range []string{"rotation_interval", "grace_period"} {
		if raw := body[field]; raw != nil {
			value, ok := raw.(string)
			if !ok || value != "" {
				return fmt.Errorf("%s is not supported; rotation revokes the old key immediately", field)
			}
		}
	}
	if raw := body["allowed_routes"]; raw != nil {
		routes, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("allowed_routes must be an array containing only llm_api_routes")
		}
		for _, route := range routes {
			if value, ok := route.(string); !ok || value != "llm_api_routes" {
				return fmt.Errorf("allowed_routes supports only llm_api_routes")
			}
		}
	}
	return nil
}
