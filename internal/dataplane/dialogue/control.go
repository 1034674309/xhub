// 本文件集中移除网关已经消费的请求控制字段，避免泄露到供应商或被协议转换器误拒绝。
// 调用关系与边界：统一对话请求构造与流协议转换；具体参数、返回及副作用见各方法说明。
package dialogue

// StripGatewayFields 从即将发往上游的正文中删除网关专用连接、限额和控制字段。
// 参数 body 应是本次执行的独立映射；返回值无，原地删除，nil 安全。
// BuildRegisteredOperation 调用；只删明确维护的网关字段，metadata、id、user、cache 等合法协议及未知扩展保留。
// 避免控制开关泄露到供应商或被其误解；调用方如需保留原始日志必须先复制顶层映射。
func StripGatewayFields(body map[string]any) {
	for _, key := range []string{
		"api_key", "api_base", "api_version", "custom_llm_provider", "litellm_credential_name",
		"rpm", "tpm", "guardrails", "litellm_trace_id", "session_id", "disable_fallbacks",
	} {
		delete(body, key)
	}
}
