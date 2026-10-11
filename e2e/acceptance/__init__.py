"""XHub 真实验收公共 API；执行入口统一为 scripts/e2e.py。"""
from .dataset import Dataset
from .observer import Observer
from .support import acceptance_run_id, temporary_retry_delay, chat_probe_result, fallback_acceptance_deployments, fallback_acceptance_route, weighted_acceptance_route, header_value, response_cache_hit, cache_bill_check, load_manifest, resolve_document, hierarchy, write_private, http, curl_json, forward_supplier_chat, chat_candidates, bill_check

