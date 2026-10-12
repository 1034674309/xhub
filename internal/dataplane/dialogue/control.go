package dialogue

// StripGatewayFields 移除 XHub 的连接、限流和请求控制字段，保留供应商合法参数及未知扩展。
// 参数 body 为本次执行的独立正文副本；无返回值，原地删除字段，nil 安全。
// 调用：非对话统一请求构造；metadata、id、user、cache 等字段不属于本表。
func StripGatewayFields(body map[string]any) {
	for _, key := range []string{
		"api_key", "api_base", "api_version", "custom_llm_provider", "litellm_credential_name",
		"rpm", "tpm", "guardrails", "litellm_trace_id", "session_id", "disable_fallbacks",
	} {
		delete(body, key)
	}
}
