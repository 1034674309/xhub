"""流终态浏览器夹具：只响应专属本地型号，不使用外部凭据或修改业务状态。"""
import json
import select


MODELS = {"e2e-stream-terminal-missing", "e2e-stream-terminal-response", "e2e-stream-terminal-held", "e2e-stream-terminal-length"}


def terminal_payload(model):
    """生成专属型号的JSON结果和SSE事件；参数为已校验型号，返回二元组，无网络或文件副作用。"""
    usage = {"prompt_tokens": 8, "completion_tokens": 2}
    message = {"role": "assistant", "content": "TERMINAL_BROWSER_OK"}
    stop = "stop"
    if model.endswith("length"):
        message["tool_calls"] = [{"index": 0, "id": "call_terminal_browser", "type": "function", "function": {"name": "lookup", "arguments": "{}"}}]
        stop = "length"
    if model.endswith("missing"):
        stop = None
    payload = {"id": "chat_terminal_browser", "choices": [{"index": 0, "message": message, "finish_reason": stop}], "usage": usage}
    events = [{"id": payload["id"], "choices": [{"index": 0, "delta": message}]}, {"id": payload["id"], "choices": [{"index": 0, "delta": {}, "finish_reason": stop}], "usage": usage}, "[DONE]"]
    if model.endswith("response"):
        events = [{"type": "response.created", "response": {"id": "resp_terminal_browser", "status": "in_progress", "output": []}}, {"type": "response.output_text.delta", "delta": "TERMINAL_BROWSER_OK"}, {"type": "response.completed"}]
    return payload, events


def serve_stream_terminal(handler, request):
    """处理专属模型的真实HTTP请求；返回是否已处理，非法凭据/路径/流模式返回明确4xx。

    浏览器到Go网关的E2E调用；held在合法终态后等待网关关闭响应，最多10秒兜底，不以Content-Length制造隐式EOF。
    不匹配的型号不写响应；请求与服务由隔离运行器清理。
    """
    model = request.get("model")
    if model not in MODELS:
        return False
    if handler.headers.get("Authorization") != "Bearer sk-fake":
        handler.send_error(401)
        return True
    endpoint = "/responses" if model.endswith("response") else "/chat/completions"
    if not handler.path.endswith(endpoint) or (model.endswith(("missing", "response", "held")) and request.get("stream") is not True):
        handler.send_error(400, "terminal fixture requires the declared protocol and streaming mode")
        return True
    payload, events = terminal_payload(model)
    if not request.get("stream"):
        handler._json(payload)
    elif model.endswith("held"):
        handler.send_response(200)
        handler.send_header("Content-Type", "text/event-stream")
        handler.send_header("Connection", "close")
        handler.end_headers()
        raw = "".join("data: " + (event if isinstance(event, str) else json.dumps(event)) + "\n\n" for event in events)
        handler.wfile.write(raw.encode())
        handler.wfile.flush()
        # 网关在有效终态后应主动关闭正文；旧桥接会一直等待，浏览器场景据此检查及时完成。
        select.select([handler.connection], [], [], 10)
        handler.close_connection = True
    else:
        handler._sse(events)
    return True
