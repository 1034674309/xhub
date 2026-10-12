// Package dataplane 数据面目录总览。gateway 提供身份、凭据、缓存和记账宿主，execution 负责请求执行；其余子包只提供明确边界能力。
// 统一推理调用 execution.Serve，供应商原生请求调用 execution.ServeBypass；不要把本目录当成第二套网关服务。
// contract 定义接口，routing 选目标，dialogue 适配协议，transport 发原生 HTTP，stream 做透传，usage 归一统计，diagnostics 保存事实，runtime 管理热状态与异步落库。
// 具体接入与可运行示例见 readme_cn.md；本根包不提供可调用的执行方法。
package dataplane
