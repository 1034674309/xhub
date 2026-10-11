# openai

OpenAI 原生协议注册.

代码入口：[native.go](native.go)

单元测试与源代码放在一起：[native_test.go](native_test.go), [registration_test.go](registration_test.go)

go test ./internal/provider/openai/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
