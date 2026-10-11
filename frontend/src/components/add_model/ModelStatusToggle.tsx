"use client";
import { useT } from "@/i18n";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import useAuthorized from "@/app/(dashboard)/hooks/useAuthorized";
import { modelPatchUpdateCall } from "@/components/networking";
import { Switch } from "@/components/ui/switch";
import { isProxyAdminRole } from "@/utils/roles";
import { toast } from "@/lib/toast";
import type { EditorModel } from "./modelEditorPricing";

/** 展示和保存部署启用状态；参数为部署，返回本地化开关；仅管理员可写，失败保留状态并提示。 */
export default function ModelStatusToggle({ model }: { model: EditorModel }) {
  const t = useT();
  const { accessToken, userRole, isViewOnly } = useAuthorized();
  const client = useQueryClient();
  const [saving, setSaving] = useState(false);
  const enabled = model.model_info.disabled !== true;
  const editable = !isViewOnly && isProxyAdminRole(userRole ?? "") && model.model_info.db_model === true;
  const change = async (checked: boolean) => {
    if (!editable || !accessToken) return;
    setSaving(true);
    try {
      await modelPatchUpdateCall(accessToken, { model_info: { disabled: !checked } }, String(model.model_info.id));
      await client.invalidateQueries({ queryKey: ["models"] });
      await client.invalidateQueries({ queryKey: ["allProxyModels"] });
      await client.invalidateQueries({ queryKey: ["userModels"] });
      await client.invalidateQueries({ queryKey: ["modelHub"] });
      await client.invalidateQueries({ queryKey: ["selectedTeamModels"] });
      toast.success(checked ? t("providerGuideUI.enabledToast") : t("providerGuideUI.disabledToast"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("providerGuideUI.statusFailed"));
    } finally {
      setSaving(false);
    }
  };
  return (
    <div className="flex items-center gap-2 whitespace-nowrap">
      <Switch
        checked={enabled}
        disabled={!editable || saving}
        onCheckedChange={change}
        aria-label={t("providerGuideUI.toggleLabel", { name: model.model_name })}
      />
      <span className="text-xs text-muted-foreground">{enabled ? t("providerGuideUI.enabled") : t("providerGuideUI.disabled")}</span>
    </div>
  );
}
