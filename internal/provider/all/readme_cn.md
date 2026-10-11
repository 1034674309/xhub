# all

启动时装配全部供应商.

代码入口：[all.go](all.go)

本包只导入供应商包，触发启动注册，不包含执行或转换逻辑。供应商行为的单元测试放在各自源码旁，注册表契约测试在 [registry](../registry/readme_cn.md)。

使用 `go test ./internal/provider/all/...` 检查装配编译。

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
