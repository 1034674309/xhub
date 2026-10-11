# 七牛 Fal 与内容生成模型目录

[English](readme.md)

## 说明

本包登记七牛兼容 Fal 队列 transport 和 /v3 内容生成 transport。连接由管理员通过 Custom/OpenAI 兼容部署提供；注册过程不创建账号，也不读取凭据。模型条目分别保存上游 ID、定价 ID、来源、transport ID 与计量维度。

### 依赖与测试

本包使用 provider 注册表和共享 Fal/Seedance 计量。测试覆盖精确路径、鉴权、前缀、实测单位、成功边界、变体，以及上下文不足时阻止自动定价。

## 对外接口

本包无导出函数和本地路由。Fal 提供精确 POST /queue/{模型路径}、GET /queue/{组}/requests/{request_id}、GET .../status。内容生成提供 POST /v3/contents/generations/tasks 和 GET /v3/contents/generations/tasks/{id}，没有 list。provider.Match 命中后交给原生 dataplane。

## 内部核心实现

registerFal 为每个队列族创建 transport、两个共享查询 action 和每个具体模型的 create action。它使用 Authorization: Key、request_id，去掉 qiniu/，改写队列 URL，并按分辨率、音频、变体绑定 FalBilling。init 明确枚举 Seedance/Dreamina、Kling、Vidu、Veo、MiniMax，通配符不能扩大白名单。

内容 transport 使用 Bearer、id、前缀移除和 SeedanceBilling；四个 bytedance Seedance 模型分别保存市场定价 ID 与来源。后续结算要求明确成功终态和正数实测 usage。

## 源码职责与入口

- [fal.go](fal.go)：falEndpoint、registerFal、具体队列 action/模型、目录、Key 鉴权、URL 改写与 Fal 计量。
- [seedance.go](seedance.go)：qiniu_contents_generation 创建/查询、四个模型、Bearer、前缀和 Seedance 计量。

验证命令：go test ./internal/provider/qiniu ./internal/provider -count=1。可用性、额度、时长与价格时效属于外部边界。
