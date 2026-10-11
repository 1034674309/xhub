"use client";
import { useT } from "@/i18n";
import { useState } from "react";
import ModelFallbackEditor, { type FallbackPolicy } from "./ModelFallbackEditor";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import useAuthorized from "@/app/(dashboard)/hooks/useAuthorized";
import { apiClient } from "@/components/networking";
import { useModelCostMap } from "@/app/(dashboard)/hooks/models/useModelCostMap";
import { useModelDetailRouting } from "../detailNavigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import ModelStatusToggle from "@/components/add_model/ModelStatusToggle";
import {
  effectivePricing,
  hasPrice,
  pricingRates,
  type EditorModel,
  type CatalogRow,
} from "@/components/add_model/modelEditorPricing";
import { policyError, type AllocationPolicy } from "@/components/route_templates/allocationPolicy";
import { toast } from "@/lib/toast";
/** 一个公开模型组拥有完整部署；目录未提供全局默认权重或返回 null 时使用权重 1。 */
type ModelGroup = {
  model_name: string;
  fallback_policy?: FallbackPolicy;
  deployments: (EditorModel & { id: string })[];
  default_weights?: { allocations?: AllocationPolicy["allocations"] | null } | null;
};
/** 计算实际部署的计费摘要，供聚合模型表展示，未定价时明确提示。 */
function priceSummary(model: EditorModel, rows: Record<string, CatalogRow>, t: ReturnType<typeof useT>) {
  const price = effectivePricing(model, rows);
  if (!hasPrice(price.row)) return t("providerGuideUI.unpriced");
  const rates = pricingRates(price.row);
  const measures = new Set(rates.map((rate) => rate.measure));
  const units: Record<string, string> = {
    token: t("providerGuideUI.perToken"),
    picture: t("providerGuideUI.perPicture"),
    second: t("providerGuideUI.seconds"),
    query: t("providerGuideUI.perQuery"),
  };
  return Array.from(measures)
    .map((measure) => units[measure] ?? measure)
    .join(" · ");
}

/** 展示声明的上游执行配置，不推断协议。 */
function callType(info: Record<string, unknown>): string {
  if (typeof info.transport === "string") return info.transport;
  return Array.isArray(info.endpoint_types) ? String(info.endpoint_types[0] ?? "") : "";
}

/** 展示完整部署表，多部署在原行编辑权重；参数为模型组、价格与保存回调。
 * 保存刷新目录，失败保留草稿；单部署仍有已保存分配时展示并允许修复，不调用供应商。
 */
function GroupDeployments({
  group,
  accessToken,
  rows,
  onSaved,
  openModel,
}: {
  group: ModelGroup;
  accessToken: string;
  rows: Record<string, CatalogRow>;
  onSaved: () => void;
  openModel: (id: string) => void;
}) {
  const t = useT();
  const [draft, setDraft] = useState<AllocationPolicy | null>(null);
  const [busy, setBusy] = useState(false);
  // 单部署可能残留零权重或已删除部署的分配，隐藏会让后台筛选与页面状态不一致。
  const showWeights = group.deployments.length > 1 || !!group.default_weights?.allocations?.length;
  /** 按规范部署 ID 读取草稿或目录权重，返回相对权重供展示与编辑初始化；
   * 缺失默认配置或分配项时返回 1，保留明确配置的 0，无写入副作用。
   */
  const weight = (id: string) =>
    (draft?.allocations ?? group.default_weights?.allocations ?? []).find((row) => row.deployment_id === id)?.weight ??
    1;
  /** 开始编辑完整模型组；使用目录返回的规范部署 ID，未配置权重为 1。 */
  // 默认权重仅作用于客户没有显式覆盖的请求；编辑完整组以避免遗漏部署。
  const edit = () =>
    setDraft({
      strategy: "traffic-split",
      allocations: group.deployments.map((dep) => ({ deployment_id: dep.id, weight: weight(dep.id) })),
    });
  /** 保存相对权重并刷新目录；失败保留输入，防止丢失管理员草稿。 */
  const save = async () => {
    if (!draft || policyError(draft)) return;
    setBusy(true);
    try {
      await apiClient.put("/model/default", {
        accessToken,
        body: { model_name: group.model_name, weights: { allocations: draft.allocations } },
      });
      setDraft(null);
      onSaved();
      toast.success(t("providerGuideUI.weightsSaved"));
    } catch (error) {
      toast.fromError(error);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      {showWeights && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">{t("providerGuideUI.weightHelp")}</p>
          {draft ? (
            <div className="flex gap-2">
              <Button disabled={busy || !!policyError(draft)} onClick={save}>
                {t("providerGuideUI.saveWeights")}
              </Button>
              <Button variant="ghost" disabled={busy} onClick={() => setDraft(null)}>
                {t("providerGuideUI.cancel")}
              </Button>
            </div>
          ) : (
            <Button size="sm" variant="outline" onClick={edit}>
              {t("providerGuideUI.editWeights")}
            </Button>
          )}
        </div>
      )}
      {draft && policyError(draft) && (
        <p role="alert" className="text-sm text-destructive">
          {policyError(draft)}
        </p>
      )}
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr>
              <th className="p-2">{t("providerGuideUI.connectionHeading")}</th>
              <th className="p-2">{t("providerGuideUI.protocolHeading")}</th>
              <th className="p-2">{t("providerGuideUI.pricingHeading")}</th>
              <th className="p-2">{t("providerGuideUI.status")}</th>
              {showWeights && <th className="p-2">{t("providerGuideUI.weight")}</th>}
              <th className="p-2">{t("providerGuideUI.actions")}</th>
            </tr>
          </thead>
          <tbody>
            {group.deployments.map((dep) => {
              const id = dep.id;
              return (
                <tr key={id} className="border-t">
                  <td className="p-2">
                    <p>{String(dep.litellm_params.litellm_credential_name || t("providerGuideUI.standalone"))}</p>
                    <p>{String(dep.litellm_params.model)}</p>
                    <p className="text-xs text-muted-foreground">{id}</p>
                  </td>
                  <td className="p-2">
                    <p>{callType(dep.model_info)}</p>
                    <p className="text-xs text-muted-foreground">
                      {Array.isArray(dep.model_info.endpoint_types)
                        ? dep.model_info.endpoint_types.join(" · ")
                        : t("providerGuideUI.noEndpoints")}
                    </p>
                  </td>
                  <td className="p-2">
                    <p>{priceSummary(dep, rows, t)}</p>
                    <p className="text-xs text-muted-foreground">{effectivePricing(dep, rows).source}</p>
                  </td>
                  <td className="p-2">
                    <ModelStatusToggle model={dep} />
                  </td>
                  {showWeights && (
                    <td className="p-2">
                      {draft ? (
                        <Input
                          className="w-24"
                          type="number"
                          min={0}
                          step="any"
                          disabled={busy}
                          aria-label={t("providerGuideUI.weightLabel", { id })}
                          value={weight(id)}
                          onChange={(event) =>
                            setDraft({
                              strategy: "traffic-split",
                              allocations: draft.allocations!.map((row) =>
                                row.deployment_id === id ? { ...row, weight: Number(event.target.value) } : row,
                              ),
                            })
                          }
                        />
                      ) : (
                        weight(id)
                      )}
                    </td>
                  )}
                  <td className="p-2">
                    <Button variant="ghost" size="sm" onClick={() => openModel(String(dep.model_info.id))}>
                      {t("providerGuideUI.details")}
                    </Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </>
  );
}
/** 模型管理按公开名称聚合、整组搜索与分页；显示真实部署并编辑默认权重及每个公开模型的回退策略。
 * 无参数，返回模型管理界面；平台管理员保存后刷新接口数据，错误保留编辑状态，不调用上游。
 */
export default function AllModelsPanel() {
  const t = useT();
  const { accessToken } = useAuthorized();
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const queryClient = useQueryClient();
  const models = useQuery({
    queryKey: ["models", "list", "groups", page, search, accessToken],
    enabled: !!accessToken,
    queryFn: () =>
      apiClient.get<{ data: ModelGroup[]; total_count: number; total_pages: number }>("/model/groups", {
        accessToken: accessToken!,
        query: { page, size: 20, search },
      }),
  });
  const { data: costMap } = useModelCostMap();
  const { openModel } = useModelDetailRouting();
  const rows = (costMap ?? {}) as Record<string, CatalogRow>;
  return (
    <section className="space-y-5" aria-label={t("providerGuideUI.management")}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-semibold">
            {t("providerGuideUI.publicCount", { count: models.data?.total_count ?? 0 })}
          </h2>
          <p className="text-sm text-muted-foreground">{t("providerGuideUI.groupHelp")}</p>
        </div>
        <Input
          className="max-w-sm"
          aria-label={t("providerGuideUI.search")}
          placeholder={t("providerGuideUI.searchPlaceholder")}
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(1);
          }}
        />
      </div>
      {models.isLoading && <p role="status">{t("providerGuideUI.loadingModels")}</p>}
      {models.isError && (
        <div role="alert">
          {t("providerGuideUI.modelsFailed")}
          <Button onClick={() => models.refetch()}>{t("providerGuideUI.reload")}</Button>
        </div>
      )}
      {models.data?.data.map((group) => (
        <section
          key={group.model_name}
          aria-label={t("providerGuideUI.publicGroup", { name: group.model_name })}
          className="space-y-4 rounded-xl border bg-card p-5"
        >
          {/* 回退入口紧邻公开模型名，单部署也显示；点击后在弹窗中编辑，不展开模型卡片。 */}
          <div className="flex flex-wrap items-start gap-4">
            <div className="min-w-0">
              <h3 className="text-lg font-semibold">{group.model_name}</h3>
              <p className="text-sm text-muted-foreground">
                {t("providerGuideUI.deploymentCount", { count: group.deployments.length })}
              </p>
            </div>
            {accessToken && (
              <ModelFallbackEditor
                model={group.model_name}
                policy={group.fallback_policy}
                accessToken={accessToken}
                onSaved={() => {
                  void queryClient.invalidateQueries({ queryKey: ["models", "list"] });
                }}
              />
            )}
          </div>
          {new Set(group.deployments.map((dep) => String(dep.litellm_params.model))).size > 1 && (
            <p className="text-sm text-amber-700">{t("providerGuideUI.mixedUpstream")}</p>
          )}
          {accessToken && (
            <GroupDeployments
              group={group}
              accessToken={accessToken}
              rows={rows}
              openModel={openModel}
              onSaved={() => {
                void queryClient.invalidateQueries({ queryKey: ["models", "list"] });
              }}
            />
          )}
        </section>
      ))}
      {!models.isLoading && !models.isError && !models.data?.data.length && <p>{t("providerGuideUI.emptyModels")}</p>}
      <div className="flex items-center justify-between">
        <p>{t("providerGuideUI.page", { page, total: models.data?.total_pages ?? 1 })}</p>
        <div className="flex gap-2">
          <Button variant="outline" disabled={page === 1} onClick={() => setPage(page - 1)}>
            {t("providerGuideUI.previous")}
          </Button>
          <Button
            variant="outline"
            disabled={page >= (models.data?.total_pages ?? 1)}
            onClick={() => setPage(page + 1)}
          >
            {t("providerGuideUI.next")}
          </Button>
        </div>
      </div>
    </section>
  );
}
