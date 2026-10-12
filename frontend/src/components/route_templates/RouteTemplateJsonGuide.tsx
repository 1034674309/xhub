"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useT } from "@/i18n";
import routeTemplateExample from "../../../../docs/testdata/route-template-example.json";

/** 可导入示例覆盖当前严格结构；公开模型名是占位值，不包含凭据，也不会发起请求。 */
export const ROUTE_TEMPLATE_EXAMPLE = routeTemplateExample;

const fieldRows = [
  ["model_routes", "fieldModelRoutes"],
  ["retry_policy", "fieldRetryPolicyRequired"],
  ["routing_groups", "fieldRoutingGroups"],
  ["fallbacks", "fieldFallbackShape"],
  ["context_window_fallbacks", "fieldContextFallbacks"],
  ["content_policy_fallbacks", "fieldContentFallbacks"],
] as const;

const strategies = [
  "random",
  "simple-shuffle",
  "traffic-split",
  "least-busy",
  "cost-based-routing",
  "latency-based-routing",
  "usage-based-routing",
] as const;

/** JsonExample 显示并复制严格结构示例；失败时给出本地化反馈，不更新模板草稿或后台数据。 */
function JsonExample() {
  const t = useT();
  const [status, setStatus] = useState("");
  const json = JSON.stringify(ROUTE_TEMPLATE_EXAMPLE, null, 2);

  /** copy 写入系统剪贴板并返回可见结果；权限失败只更新状态，无外部数据副作用。 */
  async function copy() {
    try {
      await navigator.clipboard.writeText(json);
      setStatus(t("pages.routeTemplates.jsonGuide.copied"));
    } catch {
      setStatus(t("pages.routeTemplates.jsonGuide.copyFailed"));
    }
  }

  return (
    <section aria-label={t("pages.routeTemplates.jsonGuide.exampleTitle")} className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h4 className="font-medium">{t("pages.routeTemplates.jsonGuide.exampleTitle")}</h4>
        <Button variant="outline" size="sm" onClick={() => void copy()}>
          {t("pages.routeTemplates.jsonGuide.copy")}
        </Button>
      </div>
      <pre className="overflow-x-auto rounded-lg border bg-muted/30 p-4 text-xs leading-5">{json}</pre>
      <p role="status" className="text-xs text-muted-foreground">
        {status}
      </p>
    </section>
  );
}

/** RouteTemplateJsonGuide 展示随语言切换的严格模板契约、示例和执行边界；仅复制示例，不执行配置。 */
export default function RouteTemplateJsonGuide() {
  const t = useT();
  return (
    <div className="space-y-4 pb-4 text-sm">
      <h3 className="text-base font-semibold">{t("pages.routeTemplates.jsonGuide.title")}</h3>
      <p>{t("pages.routeTemplates.jsonGuide.intro")}</p>
      <Tabs defaultValue="fields">
        <TabsList className="h-auto flex-wrap justify-start" aria-label={t("pages.routeTemplates.jsonGuide.tabsLabel")}>
          <TabsTrigger value="fields">{t("pages.routeTemplates.jsonGuide.tabFields")}</TabsTrigger>
          <TabsTrigger value="strategies">{t("pages.routeTemplates.jsonGuide.tabStrategies")}</TabsTrigger>
          <TabsTrigger value="examples">{t("pages.routeTemplates.jsonGuide.tabExamples")}</TabsTrigger>
          <TabsTrigger value="execution">{t("pages.routeTemplates.jsonGuide.tabExecution")}</TabsTrigger>
          <TabsTrigger value="validation">{t("pages.routeTemplates.jsonGuide.tabValidation")}</TabsTrigger>
        </TabsList>

        <TabsContent value="fields" className="space-y-4 pt-3">
          <p>{t("pages.routeTemplates.jsonGuide.currentStatusBody")}</p>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">{t("pages.routeTemplates.jsonGuide.referenceTitle")}</caption>
              <thead className="border-b">
                <tr>
                  <th className="p-2">{t("pages.routeTemplates.jsonGuide.field")}</th>
                  <th className="p-2">{t("pages.routeTemplates.jsonGuide.behavior")}</th>
                </tr>
              </thead>
              <tbody>
                {fieldRows.map(([field, key]) => (
                  <tr key={field} className="border-b align-top">
                    <td className="break-words p-2 font-mono">{field}</td>
                    <td className="p-2">{t("pages.routeTemplates.jsonGuide." + key)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-muted-foreground">{t("pages.routeTemplates.jsonGuide.referenceHint")}</p>
        </TabsContent>

        <TabsContent value="strategies" className="space-y-4 pt-3">
          <p>{t("pages.routeTemplates.jsonGuide.strategiesHint")}</p>
          <ul className="grid gap-2 sm:grid-cols-2">
            {strategies.map((strategy) => (
              <li key={strategy}>
                <code>{strategy}</code>
              </li>
            ))}
          </ul>
          <p>{t("pages.routeTemplates.jsonGuide.fieldRoutingStrategyArgs")}</p>
        </TabsContent>

        <TabsContent value="examples" className="space-y-6 pt-3">
          <p>{t("pages.routeTemplates.jsonGuide.examplesHint")}</p>
          <JsonExample />
        </TabsContent>

        <TabsContent value="execution" className="space-y-4 pt-3">
          <h4 className="font-medium">{t("pages.routeTemplates.jsonGuide.flowTitle")}</h4>
          <p>{t("pages.routeTemplates.jsonGuide.executionBody")}</p>
          <p>{t("pages.routeTemplates.jsonGuide.executionBoundary")}</p>
        </TabsContent>

        <TabsContent value="validation" className="space-y-4 pt-3">
          <h4 className="font-medium">{t("pages.routeTemplates.jsonGuide.tabValidation")}</h4>
          <p>{t("pages.routeTemplates.jsonGuide.validationBody")}</p>
          <p>{t("pages.routeTemplates.jsonGuide.healthFieldsHint")}</p>
        </TabsContent>
      </Tabs>
    </div>
  );
}
