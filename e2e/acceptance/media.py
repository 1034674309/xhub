"""真实验收的 media 模块；共享上下文由 Dataset 静态组合，资源与失败证据归本轮所有。"""
import json
import copy
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
import time

from .support import header_value


class MediaCases:
    """用途：封装 media 场景；通过 self 访问本轮状态、管理 API 和证据，供 Dataset 调用，不自行启动全套测试。"""

    def verify_media_models_parallel(self, key):
        """用途：并行执行选中的独立媒体用例；参数为授权密钥，无返回值，由模型阶段调用。

        每个工作线程使用独立数据集、证据目录和报告，避免读改写丢失；仅共享线程安全的清单。
        全部任务结束后顺序合并媒体检查点，即使失败也保留成功任务，随后抛出首个失败。
        不与修改额度、模型或护栏的场景重叠，不启动网关；每个视频任务内部仍按顺序创建、轮询和结算。
        """
        categories = [name for name in ("image", "ark-video", "fal-video", "kling-video") if self.case_enabled(name)]
        if not categories:
            return
        workers = []
        for category in categories:
            worker = type(self)(self.gateway, self.directory / "parallel" / self.run / category,
                                self.data, logger=self.logger)
            worker.state, worker.report = copy.deepcopy(self.state), copy.deepcopy(self.report)
            worker.admin, worker.run = self.admin, self.run
            worker.selected_categories = frozenset([category])
            worker.checklist = self.checklist
            workers.append(worker)
        baseline_report = copy.deepcopy(self.report)
        failures = []
        self.progress(f"  [媒体并行] 用例={len(workers)}，并发上限={self.media_jobs}")
        with ThreadPoolExecutor(max_workers=self.media_jobs) as pool:
            futures = [pool.submit(worker.verify_media_models, key) for worker in workers]
            for future in as_completed(futures):
                try:
                    future.result()
                except Exception as error:
                    failures.append(error)
        # 每个 worker 只替换其模型的检查点；其余历史项保持原样，新增动作按固定分类顺序合并。
        for worker in workers:
            for field in ("checks", "probe_attempts", "media_calls", "media_tasks"):
                baseline = self.report.get(field, [])
                for row in worker.report.get(field, []):
                    if row in baseline_report.get(field, []):
                        continue
                    if field in ("media_calls", "media_tasks"):
                        baseline[:] = [old for old in baseline if (old.get("model"), old.get("kind")) != (row.get("model"), row.get("kind"))]
                    if row not in baseline:
                        baseline.append(row)
                self.report[field] = baseline
        self.save()
        if failures:
            raise failures[0]

    def image_generation(self, definition, key, attempts=3, delay=10):
        """用途：执行真实图片生成并对明确的临时上游错误有限重试；参数为模型定义、个人密钥、最大次数和间隔，返回成功响应与头；每次尝试均独立记录，最终仍失败时抛错且不算通过。"""
        model, protocol = definition["public_name"], definition["transport"]
        body = {"model": model, "prompt": "A red apple on a white background.",
                "n": 1, "size": "1024x1024"}
        retryable = (429, 502, 503, 504)
        for attempt in range(1, attempts + 1):
            self.action_start(model, protocol, f"第 {attempt}/{attempts} 次真实图片生成",
                              "POST /bypass/openai/v1/images/generations，1 张 1024x1024")
            answer, headers, status = self.api(definition["create_path"], body, token=key["key"],
                                                expected=(200, *retryable), include_status=True)
            if status == 200:
                self.action_ok(model, protocol, f"第 {attempt}/{attempts} 次真实图片生成响应",
                               "HTTP=200，继续核对图片文件、请求日志和计费", record=False)
                return answer, headers
            error = answer.get("error", answer) if isinstance(answer, dict) else {}
            if isinstance(error, dict):
                summary = str(error.get("message") or error.get("code") or "无错误正文")[:300]
            else:
                summary = str(error)[:300]
            self.action_fail(model, protocol, f"第 {attempt}/{attempts} 次真实图片生成",
                             RuntimeError(f"HTTP={status}，上游临时错误={summary}"))
            if attempt < attempts:
                self.action_wait(model, protocol, "真实图片生成重试", f"{delay} 秒后进行第 {attempt + 1}/{attempts} 次")
                time.sleep(delay)
        raise AssertionError(f"gpt-image-2 连续 {attempts} 次真实生成均遇到临时上游错误")

    def verify_image_model(self, key):
        """用途：通过 Fenno gpt-image-2 真实生成一张图片；参数为个人密钥，返回无；核对响应、请求日志和目录计价证据。"""
        definition = next(row for row in self.data["models"] if row["id"] == "fenno-image")
        model, protocol = definition["public_name"], definition["transport"]
        answer, headers = self.image_generation(definition, key)
        images = answer.get("data", [])
        if not images or not (images[0].get("url") or images[0].get("b64_json")):
            raise AssertionError("gpt-image-2 真实响应缺少图片结果")
        result = self.image_result(model, answer)
        call_id = next((v for k, v in headers.items() if k.lower() == "x-litellm-call-id"), None)
        if not call_id:
            raise AssertionError("gpt-image-2 响应缺少请求日志 ID")
        bill = self.wait_bill(call_id)
        if not bill.get("metadata", {}).get("cost_breakdown") or float(bill.get("spend", 0)) <= 0:
            raise AssertionError("gpt-image-2 缺少目录价格快照或费用")
        self.report["media_calls"] = self.report.get("media_calls", []) + [{"model": model, "kind": "image",
            "call_id": call_id, "cost": bill["spend"], "bypass_result": result}]
        self.verify_image_log(self.report["media_calls"][-1])
        self.action_ok(model, protocol, "真实图片生成、日志与计费",
                       f"bypass 结果={json.dumps(result, ensure_ascii=False)}；请求日志={call_id}，金额={float(bill['spend']):.9f}")

    def verify_image_log(self, call):
        """用途：复核 gpt-image-2 的真实日志正文；参数为图片检查点，返回完整日志；验证模型、精确 prompt、图片正文和价格快照，已有付费结果可安全续跑。"""
        model, prompt = call["model"], "A red apple on a white background."
        bill = self.wait_bill(call["call_id"])
        proxy = bill.get("proxy_server_request", {})
        request = proxy.get("body", {}) if isinstance(proxy, dict) else {}
        response = bill.get("response", {})
        images = response.get("data", []) if isinstance(response, dict) else []
        if bill.get("model") != model or request.get("model") != model or request.get("prompt") != prompt:
            raise AssertionError("gpt-image-2 请求日志模型或 prompt 不完整")
        if not images or not isinstance(images[0], dict) or not (images[0].get("url") or images[0].get("b64_json")):
            raise AssertionError("gpt-image-2 请求日志缺少真实图片响应")
        if not bill.get("metadata", {}).get("cost_breakdown"):
            raise AssertionError("gpt-image-2 请求日志缺少价格快照")
        self.action_ok(model, "bypass_openai_image_generation", "请求日志正文",
                       f"request_id={call['call_id']}，模型、prompt、图片响应和价格快照完整", record=False)
        return bill

    def verify_video_logs(self, task, definition):
        """用途：复核视频的原始请求与终态证据；参数为任务检查点与模型定义，返回创建和结算详情（原日志更新时为同一条）；验证 prompt、Task ID、完成状态、最终 URL、usage 或时长，证据不全抛断言，只读日志接口。"""
        model, protocol = task["model"], task["transport"]
        prompt = "A red apple on a white table, static camera, gentle natural light."
        create_id, settlement_id = task.get("create_call_id"), task.get("settlement_log_id")
        if not create_id or not settlement_id:
            raise AssertionError(model + " 缺少创建或终态日志 ID")
        created = self.wait_log(create_id)
        if create_id == settlement_id and created.get("status") != "completed":
            raise AssertionError(model + " 原任务日志尚未完成")
        proxy = created.get("proxy_server_request", {})
        request = proxy.get("body", {}) if isinstance(proxy, dict) else {}
        response = created.get("response", {})
        request_prompt = request.get("prompt")
        if protocol == "qiniu_contents_generation":
            request_prompt = "\n".join(str(row.get("text", "")) for row in request.get("content", []) if isinstance(row, dict))
        response_task = str(response.get("id") or response.get("request_id") or "") if isinstance(response, dict) else ""
        if created.get("model") != model or request_prompt != prompt or response_task != task["task_id"]:
            raise AssertionError(model + " 创建日志的模型、prompt 或 Task ID 不完整")
        self.action_ok(model, protocol, "视频创建日志正文",
                       f"request_id={create_id}，Task ID={task['task_id']}，模型、prompt、状态和查询地址完整", record=False)

        settled = self.wait_bill(settlement_id)
        final = settled.get("response", {})
        result = final.get("result", {}) if isinstance(final, dict) and isinstance(final.get("result"), dict) else {}
        content = final.get("content", {}) if isinstance(final, dict) and isinstance(final.get("content"), dict) else {}
        video = result.get("video", {}) if isinstance(result.get("video"), dict) else {}
        url = content.get("video_url") or video.get("url")
        final_task = str(final.get("id") or final.get("request_id") or "") if isinstance(final, dict) else ""
        status = str(final.get("status") or "") if isinstance(final, dict) else ""
        expected_url = task.get("bypass_result", {}).get("video_url")
        usage = (final.get("usage") or result.get("usage")) if isinstance(final, dict) else None
        if settled.get("model") != model or final_task != task["task_id"] or not status or not url or url != expected_url:
            raise AssertionError(model + " 终态日志缺少 Task ID、状态或真实视频 URL")
        if protocol in ("qiniu_contents_generation", "qiniu_fal_doubao_20") and not isinstance(usage, dict):
            raise AssertionError(model + " 终态日志缺少真实 usage")
        if protocol == "qiniu_fal_kling" and float(video.get("duration", 0)) <= 0:
            raise AssertionError(model + " 终态日志缺少按时长结算依据")
        usage_text = f"usage={json.dumps(usage, ensure_ascii=False)}" if usage else f"duration={video.get('duration')}（供应商未返回 token usage）"
        self.action_ok(model, protocol, "视频终态日志正文",
                       f"request_id={settlement_id}，Task ID={task['task_id']}，状态={status}，最终 URL 已核对，{usage_text}", record=False)
        return created, settled

    def poll_video(self, key, definition, task_id, status_path, result_path, terminal, failure):
        """用途：轮询真实视频任务并取得终态；参数含密钥、定义、任务和路径/状态集合，返回最终 JSON；十秒输出一次等待日志。"""
        model, protocol = definition["public_name"], definition["transport"]
        deadline = time.monotonic() + 600
        while time.monotonic() < deadline:
            time.sleep(10)
            expected = (200, 202) if definition["transport"].startswith("qiniu_fal_") else 200
            status_doc, _, http_status = self.api(status_path, token=key["key"], expected=expected, include_status=True)
            status = str(status_doc.get("status", ""))
            self.action_wait(model, protocol, "轮询视频任务",
                             f"任务={task_id}，HTTP={http_status}，状态={status or 'unknown'}")
            if status.lower() in failure:
                raise AssertionError("真实视频任务失败，状态=" + status)
            if status.upper() != terminal:
                continue
            if result_path == status_path:
                return status_doc
            result = self.video_result_query(key, definition, result_path, "读取真实视频终态")
            return result.get("result", result)
        raise TimeoutError("真实视频任务十分钟内未完成，任务=" + task_id)

    def video_result_query(self, key, definition, result_path, action, attempts=3, delay=5):
        """用途：幂等读取真实视频终态并处理供应商临时错误；参数为密钥、模型、结果路径、动作名、次数和间隔，返回一次HTTP 200结果；所有失败尝试有日志，耗尽后抛错。"""
        model, protocol = definition["public_name"], definition["transport"]
        retryable = (429, 502, 503, 504)
        for attempt in range(1, attempts + 1):
            answer, _, status = self.api(result_path, token=key["key"], expected=(200, *retryable), include_status=True)
            if status == 200:
                return answer
            error = answer.get("error", answer) if isinstance(answer, dict) else {}
            if isinstance(error, dict):
                summary = str(error.get("message") or error.get("code") or "无错误正文")[:300]
            else:
                summary = str(error)[:300]
            self.action_fail(model, protocol, f"{action}第 {attempt}/{attempts} 次",
                             RuntimeError(f"HTTP={status}，上游临时错误={summary}"))
            if attempt < attempts:
                self.action_wait(model, protocol, action, f"{delay} 秒后重试第 {attempt + 1}/{attempts} 次")
                time.sleep(delay)
        raise AssertionError(f"{model} {action}连续 {attempts} 次遇到临时上游错误")

    def verify_video_model(self, key, definition):
        """用途：创建并完成最小真实视频任务；参数为个人密钥和模型定义，返回无；验证本地 URL、终态、视频及重复查询，并将创建与结算 ID 同设为原日志 ID 保存检查点。调用付费供应商，缺失 ID 或产物、终态失败及超时均抛异常。"""
        model, protocol = definition["public_name"], definition["transport"]
        if protocol == "qiniu_contents_generation":
            body = {"model": model, "content": [{"type": "text", "text": "A red apple on a white table, static camera, gentle natural light."}],
                    "duration": 4, "resolution": "480p", "ratio": "16:9", "generate_audio": False}
        elif protocol == "qiniu_fal_doubao_20":
            body = {"prompt": "A red apple on a white table, static camera, gentle natural light.",
                    "duration": "4", "resolution": "480p", "aspect_ratio": "16:9", "generate_audio": False}
        else:
            body = {"prompt": "A red apple on a white table, static camera, gentle natural light.", "duration": "5"}
        self.action_start(model, protocol, "创建真实视频任务", "使用最小时长和最低验收规格")
        created, create_headers = self.api(definition["create_path"], body, token=key["key"])
        create_call_id = header_value(create_headers, "x-litellm-call-id")
        if not create_call_id:
            raise AssertionError(model + " 创建响应缺少请求日志 ID")
        task_id = str(created.get("id") or created.get("request_id") or "")
        if not task_id:
            raise AssertionError("视频创建响应缺少任务 ID")
        if protocol == "qiniu_contents_generation":
            result_path = status_path = definition["query_prefix"] + task_id
            terminal, failures = "SUCCEEDED", {"failed", "expired", "cancelled"}
        else:
            result_path = definition["query_prefix"] + task_id
            status_path = result_path + "/status"
            if created.get("response_url") != result_path or created.get("status_url") != status_path:
                raise AssertionError("FAL 返回的任务 URL 未改写为 XHub 本地路径")
            terminal, failures = "COMPLETED", {"failed", "error", "cancelled"}
        self.action_ok(model, protocol, "创建真实视频任务", f"任务={task_id}，查询路径已验证")
        result = self.poll_video(key, definition, task_id, status_path, result_path, terminal, failures)
        if protocol == "qiniu_contents_generation":
            content = result.get("content", {})
            usage = result.get("usage", {})
            if not content.get("video_url") or int(usage.get("completion_tokens", 0)) <= 0:
                raise AssertionError("Ark Seedance 终态缺少 video_url 或非零 usage")
            video_url, duration = content["video_url"], content.get("duration", result.get("duration"))
        else:
            video = result.get("video", {})
            if not video.get("url") or (protocol == "qiniu_fal_kling" and float(video.get("duration", 0)) <= 0):
                raise AssertionError("FAL 视频终态缺少视频 URL 或时长")
            video_url, duration = video["url"], video.get("duration")
        artifact = self.result_json(definition["id"] + "-bypass-result.json", result)
        bypass_result = {"task_id": task_id, "terminal_status": terminal, "video_url": video_url,
                         "duration": duration, "response_json": artifact}
        for repeat in range(1, 4):
            self.video_result_query(key, definition, result_path, f"第 {repeat}/3 次终态查询")
            self.action_ok(model, protocol, "重复查询去重前置", f"第 {repeat}/3 次终态查询成功", record=False)
        self.report["media_tasks"] = self.report.get("media_tasks", []) + [{"model": model, "kind": "video",
            "transport": protocol, "task_id": task_id, "result_path": result_path,
            "create_call_id": create_call_id, "settlement_log_id": create_call_id, "bypass_result": bypass_result}]
        self.action_ok(model, protocol, "真实视频终态",
                       f"bypass 结果={json.dumps(bypass_result, ensure_ascii=False)}；已重复查询 3 次")

    def verify_media_models(self, key):
        """用途：依次验收图片和三种视频 bypass；参数为允许全部模型的密钥，返回无；有效检查点直接复核文件，每种付费协议仅创建一个最小任务。"""
        if self.case_enabled("image"):
            with self.case("image") as evidence:
                image_model = next(row for row in self.data["models"] if row["id"] == "fenno-image")["public_name"]
                image_rows = [row for row in self.report.get("media_calls", [])
                              if row.get("model") == image_model and row.get("kind") == "image"]
                image = image_rows[-1] if image_rows else None
                image_result = image.get("bypass_result", {}) if image else {}
                if image and all(Path(image_result.get(field, "")).is_file() for field in ("artifact", "response_json")):
                    self.report["media_calls"] = [row for row in self.report.get("media_calls", [])
                                                    if not (row.get("model") == image_model and row.get("kind") == "image")] + [image]
                    self.action_ok(image_model, "bypass_openai_image_generation", "复核已有真实图片结果",
                                   f"bytes={image_result.get('bytes')}，sha256={image_result.get('sha256')}，文件与响应 JSON 均存在", record=False)
                    self.verify_image_log(image)
                    evidence["status"] = "revalidated"
                else:
                    self.verify_image_model(key)
        for ident in self.data["verification_order"][3:]:
            case_id = {"qiniu-ark-seedance": "ark-video", "qiniu-fal-seedance": "fal-video", "qiniu-fal-kling": "kling-video"}[ident]
            if not self.case_enabled(case_id):
                continue
            with self.case(case_id) as evidence:
                definition = next(row for row in self.data["models"] if row["id"] == ident)
                model = definition["public_name"]
                rows = [row for row in self.report.get("media_tasks", [])
                        if row.get("model") == model and row.get("kind") == "video"]
                task = rows[-1] if rows else None
                result = task.get("bypass_result", {}) if task else {}
                response_json = result.get("response_json", "")
                if task and result.get("video_url") and result.get("duration") and Path(response_json).is_file():
                    self.report["media_tasks"] = [row for row in self.report.get("media_tasks", [])
                                                    if not (row.get("model") == model and row.get("kind") == "video")] + [task]
                    self.action_ok(model, definition["transport"], "复核已有真实视频结果",
                                   f"任务={result.get('task_id')}，终态={result.get('terminal_status')}，时长={result.get('duration')}，真实 URL 与响应 JSON 均存在", record=False)
                    self.verify_video_logs(task, definition)
                    evidence["status"] = "revalidated"
                else:
                    self.verify_video_model(key, definition)
        self.save()

    def verify_saved_media_logs(self):
        """用途：统一复核报告中所有真实媒体日志；无参数和返回值；由 runner 在恢复精确日志 ID 后调用，确保图片和三种视频每个子项都经过详情接口验证。"""
        # 动态探测的聊天定义没有固定 public_name；这里仅建立视频模型映射，避免续跑媒体复核误读聊天定义。
        definitions = {
            row["public_name"]: row
            for row in self.data["models"]
            if row.get("kind") == "video" and row.get("public_name")
        }
        for call in self.report.get("media_calls", []):
            if call.get("kind") == "image":
                self.verify_image_log(call)
        for task in self.report.get("media_tasks", []):
            definition = definitions.get(task.get("model"))
            if task.get("kind") == "video" and definition:
                self.verify_video_logs(task, definition)
