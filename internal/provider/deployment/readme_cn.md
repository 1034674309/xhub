# deployment

显式绑定、校验与候选筛选.

代码入口：[bindings.go](bindings.go), [candidates.go](candidates.go), [catalog_binding.go](catalog_binding.go), [execution.go](execution.go), [registry.go](registry.go), [validate.go](validate.go)

单元测试与源代码放在一起：[bindings_test.go](bindings_test.go), [candidates_test.go](candidates_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [registry_test.go](registry_test.go), [validate_test.go](validate_test.go)

go test ./internal/provider/deployment/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
