# credentials

可信凭据水合.

命名凭据只填入执行层读取的 `api_key`、`api_base` 和 `custom_llm_provider`。非空凭据密钥替换部署密钥，其余字段只补空值；环境变量引用在水合时展开。已有部署参数保持原值。

本层没有 Azure、AWS、Vertex 自动认证执行器，因此不从命名凭据复制这些认证字段。护栏的 AWS 配置由护栏模块独立处理。

代码入口：[credential.go](credential.go)

单元测试与源代码放在一起：[hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
