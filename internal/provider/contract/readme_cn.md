# contract

共享数据契约.

代码入口：[type.go](type.go)

单元测试与源代码放在一起：[match_test.go](match_test.go)

go test ./internal/provider/contract/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
