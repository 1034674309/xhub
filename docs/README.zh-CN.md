# XHub 文档

[English](README.md) · [安装 XHub](getting-started.zh-CN.md)

## 使用 XHub

| 要做什么 | 从这里开始 |
| --- | --- |
| 安装并登录 | [安装指南](getting-started.zh-CN.md) |
| 接入供应商、发布模型、创建密钥 | [图文使用手册](user-guide.zh-CN.md) |
| 排查调用失败 | [故障排查](user-guide.zh-CN.md#10-常见问题排查) |
| 接入应用 | [应用调用](user-guide.zh-CN.md#6-从应用发起调用) |
| 核对用量与费用 | [用量与费用](user-guide.zh-CN.md#8-查看用量费用和日志) |

## 调用 API

使用已经发布的模型名称和 XHub 虚拟密钥。

| 要做什么 | 接口文档 |
| --- | --- |
| 查询可用模型 | [模型列表](api/zh-CN/models.md) |
| 发送对话 | [Chat Completions](api/zh-CN/chat.md)、[Responses](api/zh-CN/responses.md) |
| 生成图片或视频 | [图片](api/zh-CN/images.md)、[视频](api/zh-CN/videos.md) |
| 排查接口错误 | [错误响应](api/zh-CN/errors.md) |

## 开发 XHub

| 要改什么 | 开发文档 |
| --- | --- |
| 找实现位置 | [开发入口](development/zh-CN/README.md)、[请求执行链](development/zh-CN/architecture.md) |
| 添加上游协议 | [Provider 注册代码示例](development/zh-CN/provider-catalog.md) |
| 修改路由或计费 | [路由](development/zh-CN/routing.md)、[计价](development/zh-CN/pricing.md) |
| 编写护栏 | [XGo 示例](development/zh-CN/guardrails.md) |
| 验证修改 | [测试命令](development/zh-CN/testing.md)、[浏览器 E2E](development/zh-CN/e2e-regression.md) |
| 查看企业身份与日志归档 | [实现状态与外部验证边界](development/zh-CN/enterprise-identity-log-archive-design.md) |
