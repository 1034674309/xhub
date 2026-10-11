// Package qiniu 登记七牛兼容的 Ark、Fal 视频协议与模型目录。
package qiniu

import (
	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
	providermeasurement "github.com/sunqirui1987/xhub/internal/provider/measurement"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	"github.com/sunqirui1987/xhub/internal/util"
)

// 进程启动时登记这个目录的执行传输和模型价格。管理员通过 Custom 或 OpenAI 兼容凭据显式配置连接，
// 这里不贡献固定供应商、不创建凭据，也不读取环境变量，避免把七牛当成预装账户。
// 参数：无。
// 返回：无。七牛兼容内容生成传输已登记，模型价格也进了目录。
// 调用：Go 在载入这个包时自动执行。
// 测试：seedance_test.go、provider_forms_test.go。
func init() {
	util.Debug("provider qiniu loading its registered transport and models")
	providerregistry.RegisterTransport(providercontract.Transport{
		CatalogID:  "qiniu",
		EndpointID: "bypass:ark-video", Protocol: "ark", Family: "video", ModelGroup: "Seedance",
		Auth: providercontract.AuthConfig{Header: "Authorization", Prefix: "Bearer"},
		ID:   "qiniu_contents_generation", Kind: providercontract.KindBypass,
		Label: "Bypass - 七牛内容生成 /v3/contents/generations/tasks",
		// OpenAI 兼容连接绑定 qiniu 目录后按型号选择实现；未绑定时仍须显式配置，地址和名称不授予能力。
		Providers: []string{"custom", "custom_openai", "openai"}, ModelField: "model", TaskID: "id", StripPrefix: "qiniu",
		Billing: providermeasurement.SeedanceBilling(),
		Actions: []providercontract.Action{
			{Name: "create", Method: "POST", PublicPath: "/v3/contents/generations/tasks", UpstreamPath: "/v3/contents/generations/tasks"},
			{Name: "get", Method: "GET", PublicPath: "/v3/contents/generations/tasks/{id}", UpstreamPath: "/v3/contents/generations/tasks/{id}"},
		},
	})
	for _, id := range []string{
		"bytedance/doubao-seedance-2-0-260128",
		"bytedance/doubao-seedance-2-0-fast-260128",
		"bytedance/doubao-seedance-2-0-mini-260615",
		"bytedance/doubao-seedance-2-5-260628",
	} {
		providerregistry.RegisterModel(providerregistry.Model{
			ID: "qiniu/" + id, Provider: "qiniu", Official: id,
			TransportID: "qiniu_contents_generation",
			Source:      "https://docs.modelink.ai/api/video-doubao-seedance-20",
			PriceModel:  id, PriceSource: "https://api.modelink.ai/v1/market/models",
		})
	}
}
