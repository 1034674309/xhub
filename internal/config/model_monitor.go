package config

import "net/url"

// MonitorIdentity 提取部署的供应商连接及实际模型，供账单快照和监控目录共同调用。
// 接收者为配置部署，返回公开身份；缺少实际模型时回退到部署名，无写入副作用。
// 内联地址只保留协议、主机和路径，禁止将账号、密码、查询参数或片段暴露到报表。
func (e ModelEntry) MonitorIdentity() (supplier, model string) {
	model = e.ParamString("model", e.ModelName)
	supplier = e.ParamString("litellm_credential_name", "")
	if supplier == "" {
		supplier = e.LogAPIBase()
	}
	return supplier, model
}

// LogAPIBase 返回部署基础地址的日志快照，供调用结算保存上游来源。
// 接收者为已解析凭证的部署；返回协议、主机与转义路径，无写入副作用。
// 缺失或非法地址返回空串；剥离账号、密码、查询参数及片段，避免泄露认证信息。
func (e ModelEntry) LogAPIBase() string {
	address, err := url.Parse(e.ParamString("api_base", ""))
	if err != nil || address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
		return ""
	}
	return address.Scheme + "://" + address.Host + address.EscapedPath()
}
