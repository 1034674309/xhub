"""验收编排模块 runner_database；仅运行入口选择的工作，失败交给上层清理。"""
import json
import re

from . import support as module
from .runner_runtime import sql


def audit_response_usage(schema="public"):
    """用途：安全核对普通成功响应中的 usage 与计量事件；参数为受 runner 控制的 schema，返回分类计数；空正文、非法 JSON、非法 token 或数值不一致都会抛出断言，查询只读。"""
    row = sql(f"""SET search_path TO {schema};
WITH candidate_logs AS MATERIALIZED (
  SELECT u.request_id, u.prompt_tokens, u.completion_tokens,
    pg_input_is_valid(l.response_body, 'jsonb') AS valid_json,
    CASE WHEN pg_input_is_valid(l.response_body, 'jsonb') THEN l.response_body::jsonb END AS response
  FROM usage_events u JOIN request_logs l ON l.request_id=u.request_id
  WHERE u.status='success' AND u.call_type IN ('chat', 'responses')
    AND u.request_id NOT LIKE 'official-settlement:%'
), parsed_logs AS MATERIALIZED (
  SELECT *, response ? 'usage' AS has_usage,
    CASE WHEN pg_input_is_valid(coalesce(response->'usage'->>'prompt_tokens', response->'usage'->>'input_tokens'), 'bigint')
      THEN coalesce(response->'usage'->>'prompt_tokens', response->'usage'->>'input_tokens')::bigint END AS response_prompt_tokens,
    CASE WHEN pg_input_is_valid(coalesce(response->'usage'->>'completion_tokens', response->'usage'->>'output_tokens'), 'bigint')
      THEN coalesce(response->'usage'->>'completion_tokens', response->'usage'->>'output_tokens')::bigint END AS response_completion_tokens
  FROM candidate_logs
)
SELECT json_build_object(
  'successful_responses', count(*),
  'invalid_json', count(*) FILTER (WHERE NOT valid_json),
  'with_usage', count(*) FILTER (WHERE has_usage),
  'invalid_usage', count(*) FILTER (WHERE has_usage AND
    (response_prompt_tokens IS NULL OR response_completion_tokens IS NULL)),
  'mismatches', count(*) FILTER (WHERE has_usage AND
    response_prompt_tokens IS NOT NULL AND response_completion_tokens IS NOT NULL AND
    (response_prompt_tokens IS DISTINCT FROM prompt_tokens OR
     response_completion_tokens IS DISTINCT FROM completion_tokens))
) FROM parsed_logs;""").strip()
    result = json.loads(row)
    if result["invalid_json"] != 0:
        raise AssertionError("普通成功响应存在空正文或非法 JSON: " + str(result["invalid_json"]))
    if result["invalid_usage"] != 0:
        raise AssertionError("普通成功响应存在非法 usage token: " + str(result["invalid_usage"]))
    if result["with_usage"] == 0:
        raise AssertionError("普通成功响应中没有可核对的 usage")
    if result["mismatches"] != 0:
        raise AssertionError("历史真实响应usage与持久化计量不一致: " + str(result["mismatches"]))
    return result

def video_log_conditions(task):
    """用途：生成视频日志的精确任务、创建与结算条件；参数为报告任务，返回三个 SQL 条件；供恢复及计量审计共用，兼容原日志更新和历史独立结算。缺少模型、任务或不支持的协议抛 ValueError，无数据库副作用。"""
    if not task.get("model") or not task.get("task_id") or task.get("transport") not in (
            "qiniu_contents_generation", "qiniu_fal_doubao_20", "qiniu_fal_kling"):
        raise ValueError("视频日志恢复需要模型、Task ID 和支持的协议")
    model = str(task["model"]).replace("'", "''")
    task_id = str(task["task_id"]).replace("'", "''")
    transport = task["transport"]
    suffix = "get" if task["transport"] == "qiniu_contents_generation" else "status"
    # 正文精确匹配任务字段，避免 Task ID 前缀、prompt 或历史同模型任务污染证据。
    scope = f"""u.model='{model}' AND CASE WHEN pg_input_is_valid(l.response_body, 'jsonb')
      THEN coalesce(nullif(l.response_body::jsonb->>'id', ''), l.response_body::jsonb->>'request_id')='{task_id}'
      ELSE false END"""
    create = f"u.call_type='{transport}:create'"
    # 新协议在原创建事件上写 completed/task_settled；旧协议保留 success 的独立结算事件。
    settled = f"""(({create} AND u.status='completed' AND u.task_settled) OR
      (u.call_type='{transport}:{suffix}' AND u.status='success'
       AND u.request_id LIKE 'official-settlement:%'))"""
    return scope, create, settled

def audit_video_settlement(task, schema="public"):
    """用途：核对精确视频任务只有一次有价格及正费用的结算；参数为任务和受控 schema，返回计数及日志 ID；供数据库验收调用，兼容原日志完成与旧独立结算，缺失或重复均抛断言，只读查询。"""
    scope, _, settled = video_log_conditions(task)
    row = sql(f"""SET search_path TO {schema};
SELECT json_build_object('rows',count(*),'priced',count(*) FILTER (WHERE u.price_snapshot<>''),
 'positive',count(*) FILTER (WHERE u.cost>0),'request_id',coalesce(min(u.request_id),''))
FROM usage_events u JOIN request_logs l USING(request_id)
WHERE {scope} AND {settled};""").strip()
    proof = json.loads(row)
    if any(proof.get(field) != 1 for field in ("rows", "priced", "positive")) or not proof.get("request_id"):
        raise AssertionError("视频任务结算未去重或缺少价格快照: " + task["model"] + " " + json.dumps(proof))
    return proof

def recover_media_log_ids(dataset, schema="public"):
    """用途：按模型、协议与精确 Task ID 恢复视频日志；参数为数据集和受控 schema，返回任务数；原日志结算时两个 ID 相同，兼容旧独立结算。缺失证据抛断言，只读 PostgreSQL 并保存报告，无供应商调用。"""
    recovered = 0
    for task in dataset.report.get("media_tasks", []):
        scope, create, settled = video_log_conditions(task)
        row = sql(f"""SET search_path TO {schema};
SELECT coalesce(min(u.request_id) FILTER (WHERE {create}), '') || '|' ||
       coalesce(min(u.request_id) FILTER (WHERE {settled}), '')
FROM usage_events u JOIN request_logs l USING(request_id)
WHERE {scope};""").strip()
        create_id, separator, settlement_id = row.partition("|")
        if not separator or not create_id or not settlement_id:
            raise AssertionError("无法恢复视频创建或终态日志 ID: " + task["model"] + " task=" + task["task_id"])
        task["create_call_id"] = create_id
        task["settlement_log_id"] = settlement_id
        recovered += 1
        dataset.progress(f"    ✅ [{task['model']} / {task['transport']} / 恢复精确日志 ID] 创建={create_id}，终态={settlement_id}")
    dataset.save()
    return recovered

def verify_database(dataset, schema="public"):
    """用途：核对聊天和媒体事件、日报、价格快照、去重与五级累计；参数为数据集和 schema，返回汇总；查询只读且失败立即中止验收。"""
    dataset.progress("    [数据库 1/6] 汇总成功事件、唯一请求和总金额，并与 usage_daily 对账")
    lines = sql(f"""SET search_path TO {schema};
SELECT json_build_object('events',count(*),'unique_calls',count(distinct request_id),'cost',coalesce(sum(cost),0)) FROM usage_events WHERE status IN ('success', 'completed');
SELECT coalesce(sum(cost),0) FROM usage_daily;
""").strip().splitlines()
    result = json.loads(lines[0])
    if abs(float(lines[1]) - result["cost"]) > 1e-9:
        raise AssertionError("每日聚合金额与真实请求事件不一致")
    dataset.progress(f"    ✅ [PostgreSQL / usage_daily / 总额对账] events={result['events']} unique_calls={result['unique_calls']} cost={result['cost']}")
    dataset.progress("    [数据库 2/6] 核对事件与每日汇总的请求数、输入 token、输出 token")
    counters = sql(f"SET search_path TO {schema}; SELECT json_build_object('requests',count(*),'prompt_tokens',coalesce(sum(prompt_tokens),0),'completion_tokens',coalesce(sum(completion_tokens),0)) FROM usage_events; SELECT json_build_object('requests',coalesce(sum(requests),0),'prompt_tokens',coalesce(sum(prompt_tokens),0),'completion_tokens',coalesce(sum(completion_tokens),0)) FROM usage_daily;").strip().splitlines()
    if json.loads(counters[0]) != json.loads(counters[1]):
        raise AssertionError("事件与每日汇总的请求数或token不一致")
    result["metering"] = json.loads(counters[0])
    dataset.progress(f"    ✅ [PostgreSQL / usage_events / 计量聚合] requests={result['metering']['requests']} prompt_tokens={result['metering']['prompt_tokens']} completion_tokens={result['metering']['completion_tokens']}")
    dataset.progress("    [数据库 3/6] 逐条核对含 usage 的普通响应与 usage_events")
    response_audit = audit_response_usage(schema)
    result["response_usage"] = response_audit
    dataset.progress(f"    ✅ [PostgreSQL / request_logs / 响应用量核对] 成功响应={response_audit['successful_responses']} 含usage={response_audit['with_usage']} 非法JSON=0 非法usage=0 用量不一致=0")
    dataset.progress("    [数据库 4/6] 精确核对 gpt-image-2 请求事件、价格快照和正费用")
    for call in dataset.report.get("media_calls", []):
        call_id = str(call["call_id"]).replace("'", "''")
        row = sql(f"""SET search_path TO {schema};
SELECT json_build_object('rows',count(*),'priced',count(*) FILTER (WHERE price_snapshot<>''),'positive',count(*) FILTER (WHERE cost>0))
FROM usage_events WHERE status='success' AND request_id='{call_id}';""").strip()
        proof = json.loads(row)
        if proof != {"rows": 1, "priced": 1, "positive": 1}:
            raise AssertionError("图片请求缺少唯一结算、价格快照或正费用: " + json.dumps(proof))
        result["image_settlement"] = proof
        dataset.progress(f"    ✅ [{call['model']} / bypass_openai_image_generation / 图片结算] 1 个真实请求、1 条 usage event、价格快照非空、费用为正")
    dataset.progress("    [数据库 5/6] 核对三种视频每个真实任务只有一条成功结算及价格快照")
    result["media_settlements"] = {}
    for task in dataset.report.get("media_tasks", []):
        proof = audit_video_settlement(task, schema)
        task["settlement_log_id"] = proof["request_id"]
        result["media_settlements"][task["model"]] = proof
        dataset.progress(f"    ✅ [{task['model']} / {task['transport']} / 结算去重] 1 个任务、1 条结算日志、3 次重复终态查询未重复扣费")
    dataset.progress("    [数据库 6/6] 核对组织、团队、用户、项目、密钥五级累计金额")
    for table, column, dimension in (("organizations", "id", "organization_id"), ("teams", "id", "team_id"),
                                     ("users", "id", "user_id"), ("projects", "id", "project_id"), ("api_keys", "id", "key_id")):
        mismatch = sql(f"SET search_path TO {schema}; SELECT count(*) FROM {table} x WHERE abs(x.spend - coalesce((SELECT sum(cost) FROM usage_events u WHERE u.{dimension}=x.{column}),0)) > 0.000000001;").strip()
        if mismatch != "0":
            raise AssertionError("累计计费不一致: " + table)
        dataset.progress(f"      ✅ [PostgreSQL / {table} / 累计金额] 不一致记录=0")
    dataset.report["checks"].append({"name": "postgres-events-daily-five-owner-spend", "passed": True})
    return result

def recover_shared_chat_checkpoints(dataset):
    """用途：从共享 PostgreSQL 恢复中断前已成功的 81 密钥聊天证据；参数为已加载状态的数据集，返回恢复数量；只接受真实 dataset 标记、choices、响应 usage、价格快照、模型和五级归属均匹配的每密钥最新记录。"""
    statement = """SELECT json_build_object(
 'request_id',u.request_id,'api_key',u.key_id,'user',u.user_id,'team_id',u.team_id,
 'project_id',u.project_id,'organization_id',u.organization_id,'model',u.model,
 'prompt_tokens',u.prompt_tokens,'completion_tokens',u.completion_tokens,'spend',u.cost,
 'metadata',json_build_object('cost_breakdown',u.price_snapshot),
 'messages',l.request_body::jsonb,'response',l.response_body::jsonb)
FROM usage_events u JOIN request_logs l USING(request_id)
WHERE u.status='success' AND u.call_type='chat' AND l.request_body LIKE '%dataset-%'
  AND u.prompt_tokens > 0 AND u.completion_tokens > 0 AND u.price_snapshot <> ''
ORDER BY u.ts DESC;"""
    rows = [json.loads(line) for line in sql(statement).splitlines() if line.strip()]
    keys = {key["token_id"]: key for key in dataset.state["keys"]}
    recovered = {}
    for bill in rows:
        key = keys.get(bill.get("api_key"))
        if not key or key["token_id"] in recovered or bill.get("model") != key.get("call_model"):
            continue
        messages, response = bill.get("messages", []), bill.get("response", {})
        match = re.search(r"dataset-[0-9a-f]+-\d+", json.dumps(messages, ensure_ascii=False))
        usage = response.get("usage", {}) if isinstance(response, dict) else {}
        try:
            cost = module.bill_check(bill, key, dataset.data["billing"])
            if not match or not response.get("choices") or any(
                    int(bill.get(field, 0)) != int(usage.get(field, -1))
                    for field in ("prompt_tokens", "completion_tokens")):
                continue
        except (AssertionError, TypeError, ValueError):
            continue
        recovered[key["token_id"]] = {"call_id": bill["request_id"], "marker": match.group(0),
            "key_id": key["token_id"], "profile": key["profile"], "model": key["call_model"],
            "prompt_tokens": bill["prompt_tokens"], "completion_tokens": bill["completion_tokens"], "cost": cost}
    existing = {row.get("key_id"): row for row in dataset.report.get("calls", [])
                if row.get("key_id") in keys and row.get("key_id") not in recovered}
    dataset.report["calls"] = [recovered.get(key["token_id"]) or existing.get(key["token_id"])
                               for key in dataset.state["keys"]
                               if recovered.get(key["token_id"]) or existing.get(key["token_id"])]
    dataset.save()
    dataset.progress(f"    ✅ [PostgreSQL / request_logs / 中断恢复] 已恢复并预校验 {len(recovered)}/81 把密钥的真实调用证据；运行期将逐条通过日志接口复验")
    return len(recovered)

