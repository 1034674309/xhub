// Package google 登记 Gemini 与 Vertex 原生协议，地址和凭据由部署显式提供。
package google

import (
	"strings"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providermeasurement "github.com/sunqirui1987/xhub/internal/provider/measurement"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
)

// init 在启动时登记 Google 固定动作；无参数或返回值，仅修改内置注册表。
// Gemini 使用 API key，Vertex 使用 OAuth Bearer；项目、区域和发布者路径由部署配置，不猜测默认值。
func init() {
	for _, protocol := range []string{"gemini", "vertex"} {
		public, upstream := "/v1beta/models/{model}", "/v1beta/models/{model}"
		auth := providercontract.AuthConfig{Header: "x-goog-api-key"}
		if protocol == "vertex" {
			public, upstream = "/vertex/v1/models/{model}", "/models/{model}"
			auth = providercontract.AuthConfig{Header: "Authorization", Prefix: "Bearer"}
		}
		tr := providercontract.Transport{ID: protocol + "_generate_content", Label: protocol, EndpointID: protocol, Protocol: protocol, Kind: providercontract.KindBypass, Family: "chat", Auth: auth, ResponseUsage: providermeasurement.NativeUsage(protocol)}
		for _, operation := range []string{"generateContent", "streamGenerateContent", "countTokens"} {
			tr.Actions = append(tr.Actions, providercontract.Action{Name: "create", Method: "POST", PublicPath: public + ":" + operation, UpstreamPath: upstream + ":" + operation})
		}
		// 同一上游配置额外登记独立 Bypass 路径，保留原厂扩展字段。
		for _, action := range append([]providercontract.Action(nil), tr.Actions...) {
			if protocol == "gemini" {
				action.PublicPath = "/bypass/gemini" + action.PublicPath
			} else {
				action.PublicPath = "/bypass/vertex" + strings.TrimPrefix(action.PublicPath, "/vertex")
			}
			tr.Actions = append(tr.Actions, action)
		}
		providerregistry.RegisterTransport(tr)
	}
}
