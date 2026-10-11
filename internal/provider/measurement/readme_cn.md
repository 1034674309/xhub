# measurement

响应/SSE 用量和任务计量事实.

代码入口：[fal_billing.go](fal_billing.go), [native_usage.go](native_usage.go), [seedance_billing.go](seedance_billing.go)

单元测试与源代码放在一起：[bindings_test.go](bindings_test.go), [google_usage_test.go](google_usage_test.go), [native_usage_test.go](native_usage_test.go)

go test ./internal/provider/measurement/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
