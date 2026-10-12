package config

import "net/url"

// MonitorIdentity 提取部署的供应商连接及实际模型，供账单快照和监控目录共同调用。
// 接收者为配置部署，返回公开身份；缺少实际模型时回退到部署名，无写入副作用。
// 内联地址只保留协议、主机和路径，禁止将账号、密码、查询参数或片段暴露到报表。
func (e ModelEntry) MonitorIdentity() (supplier, model string) {
	model = e.ParamString("model", e.ModelName)
	supplier = e.ParamString("litellm_credential_name", "")
	if supplier == "" {
		if address, err := url.Parse(e.ParamString("api_base", "")); err == nil && address.Host != "" {
			supplier = address.Scheme + "://" + address.Host + address.EscapedPath()
		}
	}
	return supplier, model
}
