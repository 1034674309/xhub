# 火山引擎方舟 Seedance 注册

[English](readme.md)

## 说明

本包声明 VolcEngine supplier 并登记方舟原生异步内容生成。管理员仍需提供 base URL、Bearer 凭据和部署价格；两个 Seedance 模型是目录快捷项。

### 依赖与测试

本包使用 provider 注册表与共享 Seedance 计量。测试验证 supplier/模型、前缀、路径和绑定，不发真实请求。

## 对外接口

本包无导出 API 或 handler。init 使 provider.Match 识别 POST /api/v3/contents/generations/tasks、GET 同路径列表、GET /api/v3/contents/generations/tasks/{id}；原生 dataplane 负责鉴权、固定与结算。

## 内部核心实现

ark_contents_generation 使用 bypass:ark-video、ark 协议、video family、Bearer、model 字段、id 任务字段，并去掉 volcengine/。创建、列表、查询是独立白名单 action。SeedanceBilling 只接受明确成功终态 usage。存储模型 ID 映射到方舟官方名称，价格由部署配置。

## 源码职责与入口

- [seedance.go](seedance.go)：supplier、transport、三个 action、两个模型绑定、鉴权/前缀/任务元数据和计量。

验证命令：go test ./internal/provider/volcengine ./internal/provider -count=1。真实账号、额度、字段和价格属于外部边界。
