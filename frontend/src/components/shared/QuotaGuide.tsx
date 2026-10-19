"use client";

import { ChevronDown } from "lucide-react";
import { useT } from "@/i18n";

/** 默认折叠额度帮助，点击标题后展开；参数 scope 指定层级，返回随语言切换的说明区域。调用：管理页面；无写入副作用。 */
export function QuotaGuide({ scope }: { scope: "organization" | "team" | "person" | "key" }) {
  const t = useT();
  return (
    <aside aria-label={t("quotaGuide.title")} className="rounded-lg border bg-muted/20 text-sm">
      <details className="group p-4">
        <summary className="flex cursor-pointer list-none items-center justify-between gap-2 font-medium [&::-webkit-details-marker]:hidden">
          {t("quotaGuide.title")}
          <ChevronDown aria-hidden="true" className="size-4 transition-transform group-open:rotate-180" />
        </summary>
        <div className="mt-3 space-y-2">
          {["chain", scope, "shared", "singleTeam", "business", "rates"].map((key) => (
            <p key={key} className="mt-1 text-muted-foreground">
              {t("quotaGuide." + key)}
            </p>
          ))}
        </div>
      </details>
    </aside>
  );
}
