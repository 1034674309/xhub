# Extend XGo guardrails

Open Guardrails → XGo custom script, paste this code, test ordinary and matching text, then save. Only pre_call text checks on normal Chat / Responses are supported.

~~~go
import . "xhub/guardrail"

// ApplyGuardrail 检查请求文本，返回拦截或修改结果，供 pre_call 调用。
// texts 数量与顺序不变；requestData 是 model/metadata 副本，inputType 为 request。
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

Select a saved rule per request:

~~~json
{"model":"company-chat","messages":[{"role":"user","content":"Hello"}],"guardrails":["script-policy"]}
~~~

| Return / setting | Behavior |
| --- | --- |
| Allow() | Original text; does not write back texts |
| Modify(texts) | Apply changes with the same count and order |
| Block(reason) | Stop before upstream execution |
| Flag(reason, metadata) | Log and continue |
| default_on | Union with requested rules; client cannot disable defaults |
| priority | Ascending; equal values sort by ID |
| Error / timeout / invalid result | Block the request |

Only xhub/guardrail imports are allowed. RE2 has no lookarounds. RegexReplace performs literal substitution, not $1 expansion. Client metadata is not authenticated identity. Post-call checks, image moderation and team approvals are not implemented.

[Executor](../../../internal/gateway/guard/custom.go) · [Primitives](../../../internal/gateway/guard/primitives.go)

~~~bash
go test ./internal/gateway/guard -count=1
~~~
