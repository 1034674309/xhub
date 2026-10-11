# credentials

可信凭据水合.

代码入口：[credential.go](credential.go)

单元测试与源代码放在一起：[hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
