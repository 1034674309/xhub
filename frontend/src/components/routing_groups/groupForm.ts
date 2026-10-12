import { t } from "@/i18n";
import type { Allocation } from "@/components/route_templates/allocationPolicy";
/** 完整路由模板内的一条路由组定义；组别名展开成员，成员直接请求仍只使用自身部署。 */
export type GroupConfig = {
  /** 可作为请求 model 的唯一组别名；不能与公开模型重名，也不是供应商名称。 */
  group_name: string;
  /** 精确公开模型成员名，每个模型在同一模板中最多属于一组，不包含部署 ID。 */
  models: string[];
  /** 无合法会话偏好时的候选排序策略；精确模型规则优先于此组策略。 */
  routing_strategy: string;
  /** 仅 traffic-split 使用的组内部署权重，不叠加成员模型默认；未列出的部署权重为 1。 */
  routing_strategy_args?: {
    /** 使用稳定部署 ID 标识，权重 0 排除该部署。空数组表示等权。 */
    allocations: Allocation[];
  };
};
/** 编辑组时使用的公开模型目录投影，不含认证密钥或可修改的后台对象。 */
export type CatalogModel = {
  /** 模型管理中的公开模型名，用于校验组成员，不是上游型号。 */
  model_name: string;
  /** 该公开模型的全部部署，供权重引用校验和上游型号展示。 */
  deployments: {
    /** 稳定部署 ID，与 Allocation.deployment_id 一致。 */
    id: string;
    /** 仅包含展示用上游型号的参数投影，不携带凭据。 */
    litellm_params?: {
      /** 发送给供应商的上游型号，缺失时由页面显示公开模型名。 */
      model?: string;
    };
  }[];
  /** 模型默认的各类别回退链，供表单说明；模板对应源和类别可覆盖它。 */
  fallback_policy?: Record<string, string[]>;
};
export const groupStrategies: Record<string, string> = {
  "simple-shuffle": "pages.routeTemplates.editor.shuffle",
  random: "pages.routeTemplates.editor.shuffle",
  "least-busy": "pages.routeTemplates.editor.leastBusy",
  "cost-based-routing": "pages.routeTemplates.editor.cost",
  "latency-based-routing": "pages.routeTemplates.editor.latency",
  "usage-based-routing": "pages.routeTemplates.editor.usage",
  "traffic-split": "pages.routeTemplates.editor.weighted",
};
export const groupHints: Record<string, string> = {
  random: "pages.routeTemplates.editor.shuffleHint",
  "simple-shuffle": "pages.routeTemplates.editor.shuffleHint",
  "least-busy": "pages.routeTemplates.editor.busyHint",
  "cost-based-routing": "pages.routeTemplates.editor.costHint",
  "latency-based-routing": "pages.routeTemplates.editor.latencyHint",
  "usage-based-routing": "pages.routeTemplates.editor.usageHint",
  "traffic-split": "pages.routeTemplates.editor.trafficHint",
};
/** 校验弹窗草稿的名称、成员占用与真实部署权重；返回当前语言错误或空串，不修改输入。
 * 保存前调用，后台再次验证；编辑时允许保留当前名称和原有成员。 */
export function groupFormError(
  draft: GroupConfig,
  groups: GroupConfig[],
  catalog: CatalogModel[],
  editing = false,
): string {
  if (
    !draft.group_name ||
    draft.group_name.trim() !== draft.group_name ||
    /[\s*]/.test(draft.group_name) ||
    [...draft.group_name].length > 64 ||
    draft.group_name === "default"
  )
    return t("pages.routeTemplates.editor.validGroupName");
  if (
    catalog.some((row) => row.model_name === draft.group_name) ||
    (!editing && groups.some((g) => g.group_name === draft.group_name))
  )
    return t("pages.routeTemplates.editor.groupNameConflict");
  if (!draft.models.length || new Set(draft.models).size !== draft.models.length)
    return t("pages.routeTemplates.editor.uniqueMembers");
  for (const name of draft.models) {
    if (!catalog.some((row) => row.model_name === name) || name.includes("*"))
      return t("pages.routeTemplates.editor.exactMembers");
    if (groups.some((g) => g.group_name !== (editing ? draft.group_name : "") && g.models.includes(name)))
      return t("pages.routeTemplates.editor.memberUsed");
  }
  if (!Object.hasOwn(groupStrategies, draft.routing_strategy)) return t("pages.routeTemplates.editor.validStrategy");
  const args = draft.routing_strategy_args;
  if (args && draft.routing_strategy !== "traffic-split") return t("pages.routeTemplates.editor.onlyWeighted");
  if (args) {
    const validIDs = new Set(
      catalog.filter((r) => draft.models.includes(r.model_name)).flatMap((r) => r.deployments.map((d) => d.id)),
    );
    const rows = args.allocations;
    if (
      new Set(rows.map((r) => r.deployment_id)).size !== rows.length ||
      rows.some((r) => !validIDs.has(r.deployment_id) || !Number.isFinite(r.weight) || r.weight < 0)
    )
      return t("pages.routeTemplates.editor.validGroupWeights");
    if (rows.length && (!rows.some((r) => r.weight > 0) || !Number.isFinite(rows.reduce((n, r) => n + r.weight, 0))))
      return t("pages.routeTemplates.editor.positiveFinite");
  }
  return "";
}
/** 成员变更时丢弃已移除部署的权重；参数为草稿和完整目录，返回可保存副本，无服务器写入。 */
export function reconcileGroup(draft: GroupConfig, catalog: CatalogModel[]): GroupConfig {
  if (draft.routing_strategy !== "traffic-split") {
    const { routing_strategy_args: _, ...base } = draft;
    return base;
  }
  const previous = new Map(draft.routing_strategy_args?.allocations.map((r) => [r.deployment_id, r.weight]));
  return {
    ...draft,
    routing_strategy_args: {
      allocations: catalog
        .filter((r) => draft.models.includes(r.model_name))
        .flatMap((r) => r.deployments.map((d) => ({ deployment_id: d.id, weight: previous.get(d.id) ?? 1 }))),
    },
  };
}
