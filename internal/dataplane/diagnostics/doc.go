// Package diagnostics 出站事实观察和安全进程日志。原始响应统计与计费用量分别保存，观察失败不改变传输结果。
// 原始响应头并未自动脱敏；消费方在持久化或对外输出时仍须过滤敏感字段。
package diagnostics
