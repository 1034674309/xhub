# registry

内置注册与路径匹配.

代码入口：[catalog_binding.go](catalog_binding.go), [match.go](match.go), [models.go](models.go), [registry.go](registry.go), [suppliers.go](suppliers.go)

单元测试与源代码放在一起：[boundaries_test.go](boundaries_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [isolation_test.go](isolation_test.go), [match_test.go](match_test.go), [registry_test.go](registry_test.go)

go test ./internal/provider/registry/...

注册和读取均复制可变切片、映射和计量回调容器。Match 仅接受后台固定动作，不读取部署上传路径。

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。

## 覆盖率检查

运行 scripts/core-codec-coverage.sh（使用 bash），注册表和对话编解码各自必须达到 95% 语句覆盖率。测试失败、缺少报告或低于下限都会返回失败。覆盖率来自各包自身的单元测试，不混入后台回归或浏览器测试。
