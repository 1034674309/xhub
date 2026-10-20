"""签名推理浏览器夹具；只接受专属本地模型，严格核对两轮工具与完整原生历史。"""
import json
import uuid


def signed_content(round_number):
    """生成指定轮次的thinking/redacted/text/tool有序块；返回合成内容，供响应与历史对照，无副作用。"""
    tool_input = {"step": round_number}
    if round_number == 2:
        tool_input["previous"] = "VALUE_1"
    return [
        {"type": "thinking", "thinking": f"Public thought {round_number}", "signature": f"PRIVATE_SIGNATURE_{round_number}"},
        {"type": "redacted_thinking", "data": f"PRIVATE_REDACTED_{round_number}"},
        {"type": "text", "text": "Working."},
        {"type": "tool_use", "id": f"call_{round_number}", "name": "lookup", "input": tool_input},
    ]


def _text(content):
    """读取字符串或文本块中的测试值，参数为内容；返回合成文本，不修改请求或外部状态。"""
    if isinstance(content, str):
        return content
    if not isinstance(content, list):
        raise ValueError("expected string or text block content")
    return "".join(part.get("text", "") for part in content if isinstance(part, dict) and part.get("type") == "text")


def signed_payload(request):
    """校验真实出站请求并生成完整JSON/SSE；参数为Messages正文，返回二元组，异常为ValueError，无网络副作用。"""
    if request.get("thinking") != {"type": "adaptive"} or request.get("output_config") != {"effort": "low"}:
        raise ValueError("adaptive thinking or low effort was lost")
    if request.get("max_tokens") != 64:
        raise ValueError("output cap was lost")
    messages = request.get("messages")
    if not isinstance(messages, list) or not messages or messages[0].get("role") != "user" or _text(messages[0].get("content")) != "Read both synthetic values using lookup.":
        raise ValueError("initial user prompt changed")
    if len(messages) not in (1, 3, 5):
        raise ValueError("expected initial user plus complete assistant/tool pairs")
    round_number = (len(messages) + 1) // 2
    for number in range(1, round_number):
        assistant, result = messages[2 * number - 1:2 * number + 1]
        if assistant != {"role": "assistant", "content": signed_content(number)}:
            raise ValueError("signed block order, signature or tool arguments changed")
        blocks = result.get("content")
        if result.get("role") != "user" or not isinstance(blocks, list) or len(blocks) != 1:
            raise ValueError("tool result role or count changed")
        block = blocks[0]
        if block.get("type") != "tool_result" or block.get("tool_use_id") != f"call_{number}" or _text(block.get("content")) != f"VALUE_{number}":
            raise ValueError("dependent tool result changed")
    content = signed_content(round_number) if round_number < 3 else [{"type": "text", "text": "VALUE_1|VALUE_2"}]
    payload = {"id": "msg_signed_" + uuid.uuid4().hex, "type": "message", "role": "assistant", "content": content,
               "stop_reason": "tool_use" if round_number < 3 else "end_turn", "usage": {"input_tokens": 8, "output_tokens": 4}}
    events = [{"type": "message_start", "message": {**payload, "content": [], "stop_reason": None, "usage": {"input_tokens": 8, "output_tokens": 0}}}]
    for index, block in enumerate(content):
        initial = dict(block)
        if block["type"] == "thinking":
            initial.update(thinking="", signature="")
        elif block["type"] == "text":
            initial["text"] = ""
        elif block["type"] == "tool_use":
            initial["input"] = {}
        events.append({"type": "content_block_start", "index": index, "content_block": initial})
        deltas = []
        if block["type"] == "thinking":
            deltas = [{"type": "thinking_delta", "thinking": block["thinking"]}] + [
                {"type": "signature_delta", "signature": part} for part in (block["signature"][:5], block["signature"][5:])]
        elif block["type"] == "text":
            deltas = [{"type": "text_delta", "text": block["text"]}]
        elif block["type"] == "tool_use":
            arguments = json.dumps(block["input"])
            deltas = [{"type": "input_json_delta", "partial_json": part} for part in (arguments[:5], arguments[5:])]
        events.extend({"type": "content_block_delta", "index": index, "delta": delta} for delta in deltas)
        events.append({"type": "content_block_stop", "index": index})
    events.extend([{"type": "message_delta", "delta": {"stop_reason": payload["stop_reason"]}, "usage": {"output_tokens": 4}}, {"type": "message_stop"}])
    return payload, events


def serve_signed_history(handler, request):
    """处理专属模型的本地Messages HTTP调用；返回是否接管，错误明确400，非专属请求无副作用。"""
    if request.get("model") != "e2e-claude-signed":
        return False
    if handler.headers.get("x-api-key") != "sk-fake" and handler.headers.get("Authorization") != "Bearer sk-fake":
        handler.send_error(401)
        return True
    try:
        if not handler.path.endswith("/messages"):
            raise ValueError("expected native Messages path")
        payload, events = signed_payload(request)
    except (ValueError, TypeError, AttributeError) as exc:
        handler.send_error(400, str(exc))
        return True
    if request.get("stream"):
        handler._sse(events)
    else:
        handler._json(payload)
    return True
