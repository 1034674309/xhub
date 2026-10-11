# capability

公开端点与协议匹配.

代码入口：[capability.go](capability.go), [endpoints.go](endpoints.go), [execution.go](execution.go)

单元测试与源代码放在一起：[capability_test.go](capability_test.go)

go test ./internal/provider/capability/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
