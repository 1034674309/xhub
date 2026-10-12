"use client";
import { useT } from "@/i18n";
import { Input } from "@/components/ui/input";
import { policyError, strategyLabels, type AllocationPolicy } from "./allocationPolicy";
import type { SplitDeployment } from "./templateForm";
/** 编辑客户覆盖策略；参数为草稿、完整部署和回调，返回受控表单，不保存或调用上游。 */
export default function AllocationPolicyEditor({
  policy,
  deployments,
  onChange,
  disabled = false,
}: {
  policy: AllocationPolicy;
  deployments: SplitDeployment[];
  onChange: (policy: AllocationPolicy) => void;
  disabled?: boolean;
}) {
  const t = useT();
  const rows = deployments.filter((row) => row.deployment_id);
  const error = policyError(policy);
  return (
    <fieldset disabled={disabled} className="space-y-3">
      {
        <label className="flex items-center gap-3">
          {t("pages.routeTemplates.editor.strategy")}
          <select
            aria-label={t("pages.routeTemplates.editor.strategy")}
            className="rounded-md border bg-background p-2"
            value={policy.strategy}
            onChange={(event) => {
              const strategy = event.target.value;
              onChange(
                strategy === "traffic-split"
                  ? { strategy, allocations: rows.map((row) => ({ deployment_id: row.deployment_id!, weight: 1 })) }
                  : { strategy },
              );
            }}
          >
            {Object.entries(strategyLabels).map(([id, label]) => (
              <option key={id} value={id}>
                {t(label)}
              </option>
            ))}
          </select>
        </label>
      }
      {policy.strategy === "random" && (
        <p className="text-sm text-muted-foreground">{t("pages.routeTemplates.editor.shuffleHint")}</p>
      )}
      {policy.strategy === "traffic-split" && (
        <>
          <p className="text-sm text-muted-foreground">{t("pages.routeTemplates.editor.trafficHint")}</p>
          {rows.map((row) => (
            <div
              key={row.deployment_id}
              className="flex flex-wrap items-center justify-between gap-3 rounded-md border p-3"
            >
              <div>
                <p>
                  {row.supplier || row.provider || t("pages.routeTemplates.editor.unknownProvider")} · {row.model}
                </p>
                <p className="text-xs text-muted-foreground">
                  {row.transport} · {row.deployment_id}
                </p>
                {row.unavailable_reason && <p className="text-xs text-destructive">{row.unavailable_reason}</p>}
              </div>
              <label className="flex items-center gap-2">
                <Input
                  className="w-24"
                  type="number"
                  min={0}
                  step="any"
                  aria-label={t("pages.routeTemplates.editor.templateWeight", { id: row.deployment_id })}
                  value={policy.allocations?.find((a) => a.deployment_id === row.deployment_id)?.weight ?? 1}
                  onChange={(event) => {
                    const allocations = (policy.allocations ?? []).filter((a) => a.deployment_id !== row.deployment_id);
                    onChange({
                      strategy: "traffic-split",
                      allocations: [
                        ...allocations,
                        { deployment_id: row.deployment_id!, weight: Number(event.target.value) },
                      ],
                    });
                  }}
                />
              </label>
            </div>
          ))}
        </>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}
