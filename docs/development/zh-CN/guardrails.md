# XGo 护栏二开

在“护栏 → XGo 自定义脚本”粘贴代码，测试普通文本与命中文本，再保存。当前只执行普通 Chat / Responses 的 pre_call 文本检查。

~~~go
import . "xhub/guardrail"

// ApplyGuardrail 检查请求文本；返回拦截或修改结果，供 pre_call 执行器调用。
// texts 保留原有数量和顺序；requestData 是 model/metadata 副本，inputType 为 request。
func ApplyGuardrail(texts []string, requestData map[string]any, inputType string) map[string]any {
    for i, text := range texts {
        if Contains(Lower(text), "secret") {
            return Block("Sensitive content detected")
        }
        texts[i] = RegexReplace(text, "1[3-9][0-9]{9}", "[REDACTED]")
    }
    return Modify(texts)
}
~~~

按需选择已保存规则：

~~~json
{"model":"company-chat","messages":[{"role":"user","content":"Hello"}],"guardrails":["script-policy"]}
~~~

| 返回 / 配置 | 行为 |
| --- | --- |
| Allow() | 放行原文；不会回写 texts |
| Modify(texts) | 回写修改；数量、顺序必须不变 |
| Block(reason) | 阻断，不调用上游 |
| Flag(reason, metadata) | 记录告警，继续 |
| default_on | 默认规则与请求选择取并集，客户端不能关闭 |
| priority | 小值先执行；同值按 ID 排序 |
| 错误 / 超时 / 无效返回 | 阻断请求 |

仅允许导入 xhub/guardrail。RE2 不支持前后向断言；RegexReplace 是字面量替换，不展开 $1。客户端 metadata 不能充当可信身份。不支持 post_call、图片审核或团队审批。

[执行器](../../../internal/gateway/guard/custom.go) · [Primitives](../../../internal/gateway/guard/primitives.go)

~~~bash
go test ./internal/gateway/guard -count=1
~~~
