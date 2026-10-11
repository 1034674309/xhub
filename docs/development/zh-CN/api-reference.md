# 接口文档与处理器源码

对外调用看请求、响应与协议边界；修改后台看路由与处理器。不要把源码登记表当 OpenAPI。

| 调用场景 | 文档 |
| --- | --- |
| 查询公开模型 | [Models](../../api/zh-CN/models.md) |
| 普通对话 | [Chat](../../api/zh-CN/chat.md)、[Responses](../../api/zh-CN/responses.md) |
| 原生协议 | [Bypass 路径](../../api/zh-CN/bypass.md) |
| 视频任务 | [Ark](../../api/zh-CN/ark.md)、[FAL](../../api/zh-CN/fal.md) |
| 错误定位 | [状态码与日志](../../api/zh-CN/errors.md) |

## 请求示例

~~~bash
curl "$XHUB_BASE_URL/v1/models" \
  -H "Authorization: Bearer $XHUB_API_KEY"
~~~

应用使用 XHub 虚拟密钥，不使用供应商密钥。XHUB_BASE_URL 是网关根地址。响应中的模型 id 是公开名，不是供应商账户凭据。

## 改接口时找这里

| 范围 | 注册与处理器 |
| --- | --- |
| 网关 | [routes.go](../../../internal/gateway/routes.go) |
| 模型 / 价格 | [models/mount.go](../../../internal/gateway/models/mount.go) |
| 组织 / 团队 / 身份 | [identity/mount.go](../../../internal/gateway/identity/mount.go) |
| 密钥 | [keys/mount.go](../../../internal/gateway/keys/mount.go) |
| 模板 / 设置 | [prefs/mount.go](../../../internal/gateway/prefs/mount.go) |
| 用量 / 日志 | [usage/mount.go](../../../internal/gateway/usage/mount.go) |
| 协议路径 | [provider](../../../internal/provider/readme_cn.md) |

~~~bash
rg 'Register|Handle|GET|POST' internal/gateway/models/mount.go
go test ./internal/gateway/models -count=1
~~~

[完整字面量处理器索引](handler-index.md) · [测试指南](testing.md)
