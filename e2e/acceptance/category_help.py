"""E2E 分类的中文使用说明；仅生成帮助文本，不读取凭据或访问服务。"""
from .runner_runtime import E2E_CATEGORIES


CATEGORY_DESCRIPTIONS = {
    "entities": "实体基线：核对 3 个组织、9 个团队、27 个成员、9 个项目、81 把成员密钥及团队成员关系。",
    "gpt-1": "GPT 组织继承：用继承组织路由的代表密钥执行 Codex 三轮会话，核对上下文、会话粘滞和五级账单。",
    "gpt-2": "GPT Fenno 专用：用只绑定 Fenno 供应商的代表密钥执行同样的三轮会话与账单验收。",
    "gpt-3": "GPT 七牛专用：用只绑定七牛供应商的代表密钥执行同样的三轮会话与账单验收。",
    "gpt-4": "GPT 团队继承：用继承团队路由的代表密钥执行同样的三轮会话与账单验收；1–4 是路由分类编号，不是模型版本。",
    "glm": "GLM 会话：验证三轮对话上下文、真实回答、用量与账单。",
    "image": "图片生成：验证真实生成结果、图片文件及对应请求日志。",
    "ark-video": "Ark 视频：验证创建任务、轮询终态、结果与日志，并验证重复结算不会重复扣费。",
    "fal-video": "FAL Seedance 视频：验证创建任务、轮询终态、结果与日志，以及结算去重。",
    "kling-video": "FAL Kling 视频：验证创建任务、轮询终态、结果与日志，以及结算去重。",
    "inheritance": "路由继承：核对组织、团队及密钥显式绑定的实际生效路由层级。",
    "chat-bills": "聊天计费：按四种路由各选一把密钥，验证 Chat 请求、用量、五级归属和价格快照；已有证据先复验。",
    "permissions": "权限隔离：验证四类角色的操作权限，以及跨组织访问被拒绝。",
    "xgo": "XGo 脚本护栏：验证脚本持久化、调试执行、请求拦截、脱敏和放行。",
    "limits": "额度与限流：验证五级预算、每分钟请求数 RPM、每分钟 token 数 TPM、模型白名单及密钥生命周期。",
    "model-lifecycle": "模型生命周期：验证创建、下架后不可调用、恢复后可调用及删除。",
    "guard-lifecycle": "护栏生命周期：验证编辑、默认启用、执行效果及删除后的行为。",
    "global-guards": "全局护栏：验证违规请求在调用上游前被拦截，个人信息在转发前完成脱敏。",
    "profile-isolation": "供应商隔离：验证单供应商配置的密钥只访问绑定供应商，不串到其他供应商。",
    "cache-weight": "缓存与权重：验证缓存命中零费用、跨密钥隔离、封禁生效及修改路由权重后缓存失效。",
    "fallback": "故障回退：验证上游 429 后切换目标、同会话不同正文复用成功目标、逐轮真实调用、目标权限及禁用回退。",
    "media-logs": "媒体日志复验：读取已有图片与视频请求证据，核对精确日志和任务终态；应先有媒体分类的成功证据。",
    "database": "数据库计量：核对请求事件、日报、token、五级金额和结算去重；完整核对需先有聊天及媒体证据。",
    "browser": "真实基线浏览器：打开共享验收数据的控制台、日志、护栏和角色页面；媒体日志检查依赖已有证据。",
    "business-browser": "浏览器业务回归：在独立测试库中操作密钥、模型、成员、护栏、Playground、错误与任务日志，并检查页面和角色可见性；使用本地模拟供应商。",
    "regression": "后台回归：运行真实网关与数据库的接口契约、权限、预算、协议和持久化测试；默认使用模拟供应商。",
}


def category_help(categories=E2E_CATEGORIES):
    """用途：生成中文分类说明；参数为有序分类序列，返回可打印文本，供 CLI 列表及测试调用。

    空序列仍显示用法，未知分类抛 ValueError；无服务、凭据或数据读写副作用。
    """
    unknown = set(categories) - CATEGORY_DESCRIPTIONS.keys()
    if unknown:
        raise ValueError("缺少 E2E 分类中文说明: " + ", ".join(sorted(unknown)))
    lines = [
        "E2E 分类说明（端到端验收：从接口或浏览器操作到实际数据结果）",
        "用法：make e2e <分类> [<分类> ...]；分类名称保持英文，说明如下。",
        "示例：make e2e entities；make e2e limits permissions；make e2e browser；隔离后台回归使用 make regression",
        "全量：make e2e 执行真实网关支持的全部分类，包括可能付费的真实模型请求；故障注入和自启服务分类仅由隔离测试入口执行。",
        "前提：安装后端/前端及浏览器依赖，启动真实网关和前端，配置供应商凭据，并已有 make testdata 创建的共享基线。",
        "建数：make testdata 同样直连 E2E_GATEWAY（默认 http://127.0.0.1:4000），需要 E2E_DATASET_PASSWORD；不清空数据库、不启动服务、不调用模型，已有组织时拒绝覆盖。",
        "并行：make e2e 默认 E2E_JOBS=4，并行图片和三种视频；E2E_JOBS=1 恢复串行。共享配置变更、账单核对与浏览器在媒体完成后执行。",
        "费用：GPT、GLM、图片、视频及包含放行/推理的业务分类可能调用真实供应商并计费；已有成功证据的复验不等于重新生成。",
        "网关：make e2e 复用 E2E_GATEWAY（默认 http://127.0.0.1:4000）和 E2E_UI（默认 http://127.0.0.1:3000），不启动服务、不创建观察器、不改写供应商地址；管理员使用 E2E_DATASET_TOKEN 或 E2E_DATASET_ADMIN/E2E_DATASET_PASSWORD，也可复用基线会话；默认本地 4000 自动读取 configs/config.yaml 管理员配置，E2E_CONFIG 可指定文件。",
        "隔离：profile-isolation、cache-weight、fallback、business-browser、regression 需要模拟供应商或独立服务；使用 make e2e-offline 或 make regression，不计入真实网关验收通过数。",
        "报告：.e2e/real-acceptance-current（report.json、cases.json、cases.md、runner.log 及浏览器证据）。",
        "本列表仅显示帮助，不读取凭据、不启动服务、不修改数据、不调用供应商。",
        "",
    ]
    lines.extend(f"  {category:<18} {CATEGORY_DESCRIPTIONS[category]}" for category in categories)
    return "\n".join(lines)
