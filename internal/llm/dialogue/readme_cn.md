# dialogue

统一对话编解码与严格校验.

代码入口：[dialogue_google.go](dialogue_google.go), [model.go](model.go), [request.go](request.go), [response.go](response.go), [usage.go](usage.go), [validation.go](validation.go)

单元测试与源代码放在一起：[dialogue_google_test.go](dialogue_google_test.go), [dialogue_sdk_test.go](dialogue_sdk_test.go), [dialogue_test.go](dialogue_test.go), [google_boundaries_test.go](google_boundaries_test.go), [request_boundaries_test.go](request_boundaries_test.go), [response_boundaries_test.go](response_boundaries_test.go)

go test ./internal/llm/dialogue/...

model 定义统一表示，request/response 编解码，validation 校验字段，usage 统一用量，dialogue_google 处理 Google 协议。无网络调用。

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。

## 覆盖率检查

运行 scripts/core-codec-coverage.sh（使用 bash），注册表和对话编解码各自必须达到 95% 语句覆盖率。测试失败、缺少报告或低于下限都会返回失败。覆盖率来自各包自身的单元测试，不混入后台回归或浏览器测试。
