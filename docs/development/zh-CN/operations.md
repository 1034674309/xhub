# 请求故障定位

保留时间、公开模型、HTTP 状态和 x-litellm-call-id。先找到对应请求日志，再修改配置。

## 诊断请求

此请求会调用模型，可能产生供应商费用。

~~~bash
curl -i "$XHUB_BASE_URL/v1/chat/completions"   -H "Authorization: Bearer $XHUB_API_KEY"   -H "Content-Type: application/json"   -d '{"model":"company-chat","messages":[{"role":"user","content":"Reply with ready."}]}'
~~~

| 现象 | 检查 |
| --- | --- |
| 401 | XHub 密钥是否有效，不能发送供应商 Key |
| 403 / 隐藏存在性的 404 | 当前成员关系、所有权、模型范围 |
| 429 | 预算、RPM/TPM、供应商响应 |
| 无可用部署 | 启用状态、transport、端点能力 |
| 流中断 | 事件和最终日志；HTTP 状态可能已发送 |
| 缺少费用 | 实测用量、价格快照、PostgreSQL、Redis 积压 |

分享证据前移除凭据、个人信息及签名媒体 URL。不要删除生产数据来修复测试。

## 源码与验证

[计费入口](../../../internal/gateway/spend.go) · [运行边界](runtime.md)

~~~bash
go test ./internal/gateway/usage -count=1
~~~
