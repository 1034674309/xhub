import { validateSections, type TemplateSections } from "./templateSections";
import { policyError, strategyLabels, type Allocation } from "./allocationPolicy";

/** 部署目录投影用于模型与组的权重表单及预览；仅含展示信息，不包含凭据。 */
export type SplitDeployment = {
  /** 稳定部署 ID；不存在时不可配置权重或建立会话偏好。 */
  deployment_id?: string;
  /** 客户端使用的公开模型名；多个不同供应商部署可以共享它。 */
  model_name: string;
  /** 连接地址，只用于目录展示，不作为会话或供应商身份。 */
  api_base: string;
  /** 发送给供应商的上游型号，可与公开模型名不同。 */
  model: string;
  /** 部署支持的标准/原生入口能力，后端每轮据此过滤候选。 */
  endpoint_types?: string[];
  /** 执行协议标识，决定凭据适配和请求转换方式。 */
  transport?: string;
  /** 后台提供的不可用原因，存在时帮助解释部署为何不可选。 */
  unavailable_reason?: string;
  /** 供应商适配器标识，不等同于具体连接 ID。 */
  provider?: string;
  /** 供应商连接显示名，仅供展示；粘性使用 deployment_id。 */
  supplier?: string;
};

/** 精确模型规则覆盖组策略；只有没有合法动态偏好时才决定首选部署。 */
export type ModelRoutingRule = {
  /** 精确公开模型名，不接受通配符或组别名。 */
  model: string;
  /** 路由策略协议值，支持随机、加权及价格/延迟/负载排序。 */
  strategy: string;
  /** traffic-split 专用；省略继承模型默认权重，显式空数组等权，0 排除部署。 */
  allocations?: Allocation[];
};

/** 路由模板的受控编辑草稿；数值暂存字符串，允许用户输入中间态，保存时严格校验。 */
export type TemplateFormState = TemplateSections & {
  /** 每条部署的总尝试次数，包含首次；可重放失败耗尽后才切换部署。 */
  max_attempts: string;
  /** 单次上游尝试超时秒数，必须大于 0，不是整条回退链的总时长。 */
  timeout_seconds: string;
  /** 部署进入被动冷却的失败阈值；0 禁用被动冷却，不截断当前已分配重试次数。 */
  failure_threshold: string;
  /** 达到失败阈值后的冷却秒数；后续请求排除仍处于冷却的部署。 */
  cooldown_seconds: string;
  /** 精确公开模型的覆盖规则；未覆盖的模型沿用组策略或模型默认配置。 */
  model_routes: ModelRoutingRule[];
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === "object" && !Array.isArray(value);

/** emptyForm 为创建页返回模型负载均衡、组、回退和可靠性骨架；无参数和外部副作用。 */
export const emptyForm = (): TemplateFormState => ({
  max_attempts: "1",
  timeout_seconds: "60",
  failure_threshold: "3",
  cooldown_seconds: "60",
  model_routes: [],
  routing_groups: [],
  fallbacks: [],
  context_window_fallbacks: [],
  content_policy_fallbacks: [],
});

/** formFromBody 将已校验正文转换为独立草稿；参数为完整模板，返回受控表单，不修改原文档。
 * 模型规则、路由组和回退保留省略与显式空值的区别；编辑和复制共用，无后台写入。 */
export const formFromBody = (body: Record<string, unknown> = {}): TemplateFormState => {
  const retry = isRecord(body.retry_policy) ? body.retry_policy : {};
  return {
    ...emptyForm(),
    ...Object.fromEntries(
      ["routing_groups", "fallbacks", "context_window_fallbacks", "content_policy_fallbacks"].map((key) => [
        key,
        Object.hasOwn(body, key) ? structuredClone(body[key]) : undefined,
      ]),
    ),
    ...Object.fromEntries(
      ["max_attempts", "timeout_seconds", "failure_threshold", "cooldown_seconds"]
        .filter((key) => retry[key] !== undefined)
        .map((key) => [key, String(retry[key])]),
    ),
    model_routes: Array.isArray(body.model_routes) ? structuredClone(body.model_routes) : [],
  };
};

/** bodyFromForm 写出唯一模板格式，拒绝无效数字、重复模型和未知策略。 */
export const bodyFromForm = (
  form: TemplateFormState,
): { ok: true; body: Record<string, unknown> } | { ok: false; field: string } => {
  const body: Record<string, unknown> = {};
  const retryPolicy: Record<string, number> = {};
  for (const key of ["max_attempts", "timeout_seconds", "failure_threshold", "cooldown_seconds"] as const) {
    const value = Number(form[key]);
    if (
      !form[key].trim() ||
      !Number.isFinite(value) ||
      value < 0 ||
      ((key === "max_attempts" || key === "timeout_seconds") && value === 0) ||
      ((key === "max_attempts" || key === "failure_threshold") && !Number.isInteger(value))
    )
      return { ok: false, field: key };
    retryPolicy[key] = value;
  }
  const seen = new Set<string>();
  for (const rule of form.model_routes) {
    const name = rule.model.trim();
    if (!name || seen.has(name) || !Object.hasOwn(strategyLabels, rule.strategy)) {
      return { ok: false, field: "model_routes" };
    }
    if (Object.hasOwn(rule, "allocations")) {
      if (rule.strategy !== "traffic-split" || !Array.isArray(rule.allocations))
        return { ok: false, field: "model_routes.allocations" };
      if (rule.allocations.length && policyError(rule)) return { ok: false, field: "model_routes.allocations" };
    }
    seen.add(name);
  }
  body.model_routes = form.model_routes.map((rule) => ({
    model: rule.model.trim(),
    strategy: rule.strategy,
    ...(Object.hasOwn(rule, "allocations") ? { allocations: structuredClone(rule.allocations) } : {}),
  }));
  body.retry_policy = retryPolicy;
  for (const key of ["routing_groups", "fallbacks", "context_window_fallbacks", "content_policy_fallbacks"] as const) {
    if (form[key] !== undefined) body[key] = structuredClone(form[key]);
  }
  const error = validateSections(body);
  if (error) return { ok: false, field: error };
  return { ok: true, body };
};

/** parseDocument 严格解析导入 JSON；入口级条件和未知字段失败；组内支持按部署ID配置权重。 */
export const parseDocument = (text: string): { ok: true; body: Record<string, unknown> } | { ok: false } => {
  try {
    const body: unknown = JSON.parse(text);
    const allowed = [
      "model_routes",
      "retry_policy",
      "routing_groups",
      "fallbacks",
      "context_window_fallbacks",
      "content_policy_fallbacks",
    ];
    if (!isRecord(body) || Object.keys(body).some((key) => !allowed.includes(key))) return { ok: false };
    const retryFields = ["max_attempts", "timeout_seconds", "failure_threshold", "cooldown_seconds"];
    if (
      !isRecord(body.retry_policy) ||
      Object.keys(body.retry_policy).some((key) => !retryFields.includes(key)) ||
      retryFields.some((key) => typeof (body.retry_policy as Record<string, unknown>)[key] !== "number")
    )
      return { ok: false };
    if (
      !Array.isArray(body.model_routes) ||
      body.model_routes.some(
        (rule) =>
          !isRecord(rule) ||
          typeof rule.model !== "string" ||
          typeof rule.strategy !== "string" ||
          Object.keys(rule).some((key) => !["model", "strategy", "allocations"].includes(key)),
      )
    )
      return { ok: false };
    if (validateSections(body)) return { ok: false };
    const normalized = bodyFromForm(formFromBody(body));
    return normalized.ok ? { ok: true, body: normalized.body } : { ok: false };
  } catch {
    return { ok: false };
  }
};

/** prettyDocument 格式化模板 JSON，供查看、下载和编辑使用。 */
export const prettyDocument = (body: Record<string, unknown>): string => JSON.stringify(body, null, 2);

/** nextCopyName 为复制模板选择未占用名称，不写后台。 */
export const nextCopyName = (name: string, existing: readonly string[], copyOf: (name: string) => string): string => {
  const base = copyOf(name).trim() || name;
  let next = base;
  for (let n = 2; existing.includes(next); n++) next = base + " " + n;
  return next;
};

/** formatUpdatedAt 格式化有效时间；空值和错误时间返回空字符串。 */
export const formatUpdatedAt = (value: string | undefined): string => {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleString();
};

/** deploymentsFromInfo 从模型接口投影公开模型目录，保持上游模型标识原样。 */
export const deploymentsFromInfo = (rows: unknown[]): SplitDeployment[] => {
  const out: SplitDeployment[] = [];
  for (const row of rows) {
    if (!isRecord(row)) continue;
    const params = isRecord(row.litellm_params) ? row.litellm_params : {};
    const modelName = typeof row.model_name === "string" ? row.model_name : "";
    const model = typeof params.model === "string" && params.model.trim() ? params.model : modelName;
    if (!model || model.startsWith("auto_router/")) continue;
    const info = isRecord(row.model_info) ? row.model_info : {};
    const deploymentId = [params.deployment_id, info.id].find((value) => typeof value === "string" && value.trim());
    if (!deploymentId) continue;
    out.push({
      deployment_id: deploymentId as string,
      model_name: modelName || model,
      api_base: typeof params.api_base === "string" ? params.api_base.trim() : "",
      model,
      endpoint_types: Array.isArray(info.endpoint_types)
        ? info.endpoint_types.filter((id): id is string => typeof id === "string")
        : [],
      transport: typeof info.transport === "string" ? info.transport : "",
      unavailable_reason: typeof info.unavailable_reason === "string" ? info.unavailable_reason : undefined,
      provider: typeof params.custom_llm_provider === "string" ? params.custom_llm_provider : undefined,
      supplier: typeof params.litellm_credential_name === "string" ? params.litellm_credential_name : undefined,
    });
  }
  return out;
};
