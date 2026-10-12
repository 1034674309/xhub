import { t } from "@/i18n";
/** 客户覆盖策略和全局相对权重使用同一部署身份；部署 ID 是唯一选路标识。 */
export type Allocation = {
  /** 稳定部署 ID，指向供应商连接和上游型号组成的实际执行目标，不是公开模型名或供应商显示名。 */
  deployment_id: string;
  /** 新分配使用的相对权重，有限且非负；0 排除部署，正权重变化不打断仍合法的动态会话偏好。 */
  weight: number;
};
/** 当前模板对候选部署的分配规则；仅在没有合法会话偏好或选择其余候选时决定顺序。 */
export type AllocationPolicy = {
  /** 策略协议值，必须属于 strategyLabels；已有合法会话目标始终优先于价格、负载或权重。 */
  strategy: string;
  /** traffic-split 的部署权重；其他策略不接受非空分配，未列出的合法部署按权重 1 处理。 */
  allocations?: Allocation[];
};
export const strategyLabels: Record<string, string> = {
  "simple-shuffle": "pages.routeTemplates.editor.shuffleRecommended",
  random: "pages.routeTemplates.editor.shuffle",
  "traffic-split": "pages.routeTemplates.editor.traffic",
  "least-busy": "pages.routeTemplates.editor.leastBusy",
  "cost-based-routing": "pages.routeTemplates.editor.cost",
  "latency-based-routing": "pages.routeTemplates.editor.latency",
  "usage-based-routing": "pages.routeTemplates.editor.usage",
};
/** 校验编辑或导入的相对权重；允许任意总和，拒绝重复部署及全部零，无副作用。 */
export function policyError(policy: AllocationPolicy): string {
  if (!policy || !Object.hasOwn(strategyLabels, policy.strategy))
    return t("pages.routeTemplates.editor.validAllocation");
  if (policy.allocations !== undefined && !Array.isArray(policy.allocations))
    return t("pages.routeTemplates.editor.weightList");
  if (policy.strategy !== "traffic-split")
    return policy.allocations?.length ? t("pages.routeTemplates.editor.onlyTraffic") : "";
  const rows = policy.allocations ?? [];
  if (!rows.length) return t("pages.routeTemplates.editor.setWeights");
  const ids = new Set<string>();
  for (const row of rows) {
    if (!row || typeof row.deployment_id !== "string" || !row.deployment_id.trim() || ids.has(row.deployment_id))
      return t("pages.routeTemplates.editor.uniqueID");
    ids.add(row.deployment_id);
    if (Object.keys(row).some((key) => !["deployment_id", "weight"].includes(key)))
      return t("pages.routeTemplates.editor.unknownWeightField");
    if (typeof row.weight !== "number" || !Number.isFinite(row.weight) || row.weight < 0)
      return t("pages.routeTemplates.editor.nonnegative");
  }
  if (!Number.isFinite(rows.reduce((total, row) => total + row.weight, 0)))
    return t("pages.routeTemplates.editor.totalOverflow");
  return rows.some((row) => row.weight > 0) ? "" : t("pages.routeTemplates.editor.positive");
}
