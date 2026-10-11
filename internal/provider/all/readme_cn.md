# all

启动时装配全部供应商.

代码入口：[all.go](all.go)

单元测试与源代码放在一起：

go test ./internal/provider/all/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
