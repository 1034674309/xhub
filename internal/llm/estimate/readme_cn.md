# 本地 token 与成本估算辅助

[English](readme.md)

## 说明

本包提供确定性的 token 数、token 成本、折扣、加价和服务层级预估。费率单位是美元/token。结果供准入、界面预览和 LiteLLM 兼容 token-count 响应使用，不是供应商结算的权威证据。

### 依赖与测试

本包使用配置的 tokenizer 库与日志。测试覆盖折扣/加价优先级、固定加价、token 算术、层级映射和非法边界，不访问网络。

## 对外接口

本包不注册 HTTP。gateway 的 token-count 与价格逻辑调用这些函数。ApplyDiscount 返回最终成本、百分比和折扣额；供应商配置优先于 global，缺失或非法值不改变成本。ApplyMargin 返回最终值、百分比、固定额和总加价，优先级相同。TokenCost 返回输入/输出成本，负 token 数按零处理。

MapTrafficType 把 Gemini 流量类型映射到计费层级，并返回是否已知、是否标准。NormalizeServiceTier 只接受可结算层级，缺失、非字符串和 auto 不能作为最终层级。CountTokens 返回数量、实际 tokenizer 模型和错误。OpenAISupportedParams 返回新切片，并按模型/目录场景调整 response_format 与 user。ModelUsedForCount 去掉部署模型的一层供应商前缀。

## 内部核心实现

折扣以受限百分比作用于基础成本。加价先选择供应商项、再选择 global，并组合百分比与固定值。token 成本用规范化数量乘每 token 费率，不提前舍入。流量规范化保留未知与标准的区别，避免把 auto 写成已结算层级。

CountTokens 模拟 LiteLLM/OpenAI chat 记数：编码 prompt 或消息字段，加入每消息框架开销，以稳定文本形式纳入 name 和结构化内容，最后加入 assistant 起始开销。它选择兼容 tokenizer，无法建立编码时返回错误。隐藏推理、供应商二次分词、二进制媒体和未来 tokenizer 变化无法由本地估算观察。

## 源码职责与入口

- [cost.go](cost.go)：Margin、折扣/加价选择与算术、TokenCost、Gemini 流量映射和服务层级规范化。
- [tokencount.go](tokencount.go)：CountTokens 对话记数、OpenAISupportedParams 和 ModelUsedForCount 前缀处理。

验证命令：go test ./internal/llm/estimate -count=1。真实供应商分词、多模态单位、隐藏 token 和最终账单属于外部边界。
