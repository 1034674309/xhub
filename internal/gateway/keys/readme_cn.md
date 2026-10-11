# keys：平台与个人 API 密钥生命周期

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

平台与个人 API 密钥生命周期。不发推理；创建、撤销、过期、权限和秘密返回范围需一起审核。

## Review 顺序

host.go → mount.go → generate.go → admin.go / personal.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入身份及存储，auth/IAM 管理哈希与范围。

## 验证

同目录测试：key_input_test.go, personal_test.go

仓库根目录执行：go test ./internal/gateway/keys -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
