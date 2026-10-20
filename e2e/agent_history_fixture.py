"""原生 Agent 历史专用可控上游；只处理 e2e-history-*，不连接供应商。"""
import json


def validate_chat_history(messages):
    """校验并行调用与逆序结果的关联；参数为 Chat 消息，返回是否含工具结果，错误抛 ValueError，无副作用。"""
    found = False
    for index, message in enumerate(messages):
        calls = message.get("tool_calls", [])
        if calls:
            if len(calls) != 2 or [c["id"] for c in calls] != ["call_a", "call_b"]:
                raise ValueError("parallel calls must share one assistant message")
            results = messages[index + 1:index + 3]
            if [(r.get("role"), r.get("tool_call_id"), r.get("content")) for r in results] != [
                ("tool", "call_b", "beta-value"), ("tool", "call_a", "alpha-value")
            ]:
                raise ValueError("parallel results changed or detached")
            found = True
    return found


def messages_events(content, model):
    """将完整原生块拆成合法 SSE 事件；参数为块和模型，返回事件数组；签名及工具参数分片，无存储副作用。"""
    events = [{"type": "message_start", "message": {"id": "msg_history", "type": "message", "role": "assistant", "model": model, "content": [], "usage": {"input_tokens": 8}}}]
    stop = "end_turn"
    for index, block in enumerate(content):
        start = dict(block)
        deltas = []
        if block["type"] == "thinking":
            start.update(thinking="", signature="")
            deltas = [{"type": "thinking_delta", "thinking": block["thinking"]},
                      {"type": "signature_delta", "signature": "native-"}, {"type": "signature_delta", "signature": "signature"}]
        if block["type"] == "tool_use":
            stop = "tool_use"
            start["input"] = {}
            deltas = [{"type": "input_json_delta", "partial_json": '{"path":'}, {"type": "input_json_delta", "partial_json": '"fixture"}'}]
        events.append({"type": "content_block_start", "index": index, "content_block": start})
        events += [{"type": "content_block_delta", "index": index, "delta": delta} for delta in deltas]
        events.append({"type": "content_block_stop", "index": index})
    events += [{"type": "message_delta", "delta": {"stop_reason": stop}, "usage": {"output_tokens": 2}}, {"type": "message_stop"}]
    return events


def handle_agent_history(handler, request):
    """处理浏览器历史验收请求；参数为真实 HTTP Handler 和正文，返回是否已处理；错误400，其余按同步/流式输出。

    原生签名、caller、index 与并行工具关联必须完整抵达；不认识模型返回 False，测试运行器清理进程。
    """
    model = request.get("model")
    if model not in ("e2e-history-messages", "e2e-history-chat"):
        return False
    try:
        messages = request.get("messages", [])
        if model.endswith("messages"):
            if not handler.path.endswith("/messages"):
                raise ValueError("wrong native Messages route")
            original = [{"type": "thinking", "thinking": "Inspect file", "signature": "native-signature"},
                        {"type": "redacted_thinking", "data": "native-cipher"},
                        {"type": "tool_use", "id": "call_read", "name": "read", "input": {"path": "fixture"}, "caller": {"type": "direct"}}]
            assistants = [m for m in messages if m.get("role") == "assistant"]
            if assistants:
                if assistants[-1].get("content") != original or messages[-1].get("content") != [{"type": "tool_result", "tool_use_id": "call_read", "content": "file-value"}]:
                    raise ValueError("native signed history or tool result changed")
            content = original if request.get("tools") and not assistants else [{"type": "text", "text": "history-ok"}]
            if request.get("stream"):
                handler._sse(messages_events(content, model))
            else:
                handler._json({"id": "msg_history", "type": "message", "role": "assistant", "model": model, "content": content,
                               "stop_reason": "tool_use" if content == original else "end_turn", "usage": {"input_tokens": 8, "output_tokens": 2}})
        else:
            if not handler.path.endswith("/chat/completions"):
                raise ValueError("wrong native Chat route")
            continued = validate_chat_history(messages)
            calls = [{"index": i, "id": "call_" + key, "type": "function", "function": {"name": "lookup", "arguments": json.dumps({"key": key})}} for i, key in enumerate(("a", "b"))]
            message = {"role": "assistant", "content": None, "tool_calls": calls} if request.get("tools") and not continued else {"role": "assistant", "content": "history-ok"}
            stop = "tool_calls" if "tool_calls" in message else "stop"
            usage = {"prompt_tokens": 8, "completion_tokens": 2}
            if request.get("stream"):
                handler._sse([{"id": "chatcmpl_history", "model": model, "choices": [{"index": 0, "delta": message, "finish_reason": stop}], "usage": usage}, "[DONE]"])
            else:
                handler._json({"id": "chatcmpl_history", "model": model, "choices": [{"index": 0, "message": message, "finish_reason": stop}], "usage": usage})
    except (ValueError, KeyError, TypeError) as error:
        handler._response(400, json.dumps({"error": {"message": str(error)}}).encode(), "application/json")
    return True
