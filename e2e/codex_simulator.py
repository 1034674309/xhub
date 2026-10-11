#!/usr/bin/env python3
"""隔离浏览器调用的 Codex 模拟入口，复用真实数据集的三轮请求与逐轮对账。"""
import json
import sys
from acceptance import Dataset, load_manifest


def resolve_billing_identity(dataset, key):
    """用途：为个人密钥解析调用时计费归属；参数为数据集客户端和密钥响应，返回保留显式字段并增加 billing_* 快照的副本。

    Playwright 的临时个人密钥调用此函数；它从 /user/info 的唯一成员关系读取团队和组织。
    无团队时两项均为空，多团队或不完整响应违反当前单团队契约并抛 AssertionError；不修改输入。
    """
    resolved = dict(key)
    resolved["billing_team_id"] = key.get("team_id") or ""
    resolved["billing_organization_id"] = key.get("organization_id") or ""
    if key.get("owner_type") != "personal":
        return resolved
    # 创建响应的团队字段可能是旧快照，且不携带组织；个人凭证必须按调用时成员关系核对。
    resolved["billing_team_id"] = ""
    resolved["billing_organization_id"] = ""
    user_id = key.get("user_id")
    if not user_id:
        raise AssertionError("个人密钥缺少 user_id，无法解析计费归属")
    response, _ = dataset.api("/user/info?user_id=" + user_id)
    user = response.get("user_info") if isinstance(response, dict) else None
    teams = user.get("teams") if isinstance(user, dict) else None
    if not isinstance(teams, list):
        raise AssertionError("用户信息缺少 teams，无法解析计费归属")
    if len(teams) > 1:
        raise AssertionError("个人密钥用户存在多个团队，无法确定计费归属")
    if teams:
        team = teams[0]
        if not isinstance(team, dict) or not team.get("team_id") or not team.get("organization_id"):
            raise AssertionError("用户团队缺少 team_id 或 organization_id")
        resolved["billing_team_id"] = team["team_id"]
        resolved["billing_organization_id"] = team["organization_id"]
    return resolved


def main():
    """用途：从标准输入读取网关、身份与临时目录，输出脱敏会话证据；无参数/返回值。

    Playwright 调用；凭据不进入 argv，真实请求或对账失败时退出非零，临时目录及数据库由浏览器 runner 清理。
    """
    config = json.load(sys.stdin)
    dataset = Dataset(config["gateway"], config["directory"], load_manifest(), logger=lambda _: None)
    dataset.admin = config["admin"]
    key = resolve_billing_identity(dataset, config["key"])
    print(json.dumps(dataset.codex_conversation(key)))


if __name__ == "__main__":
    main()
