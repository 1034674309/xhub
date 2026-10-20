import useAuthorized from "@/app/(dashboard)/hooks/useAuthorized";
import { Alert, AlertTitle } from "@/components/shared/Alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Check, Copy, RefreshCw, TriangleAlert } from "lucide-react";
import React, { useEffect, useMemo, useState } from "react";
import { useWatch } from "react-hook-form";
import { CopyToClipboard } from "react-copy-to-clipboard";
import { z } from "zod/v4";
import { KeyResponse } from "../key_team_helpers/key_list";
import { toast } from "@/lib/toast";
import { regenerateKeyCall } from "../networking";
import { FieldGroup } from "@/components/ui/field";
import { FormField } from "@/components/shared/form/FormField";
import { Input } from "@/components/ui/input";
import { TooltipProvider } from "@/components/ui/tooltip";
import { useZodForm } from "@/lib/forms/useZodForm";
import { calculateExpiryPreviewFromDuration, formatExpiresUtc, isKeyExpired } from "@/utils/keyExpiryUtils";
import { buildRegenerateKeyPayload, type RegenerateKeyFormValues } from "./regenerateKeyPayload";
import { useT, t } from "@/i18n";

const DURATION_PATTERN = /^([1-9]\d*(s|m|h|d))?$/;

const EMPTY_VALUES: RegenerateKeyFormValues = {
  key_alias: undefined,
  max_budget: undefined,
  tpm_limit: undefined,
  rpm_limit: undefined,
  duration: "",
  grace_period: "",
};

/** 为轮换表单创建校验规则；参数为过期状态，返回期限规则；无副作用。 */
const buildSchema = (keyIsExpired: boolean): z.ZodType<RegenerateKeyFormValues, RegenerateKeyFormValues> => {
  const shape = {
    key_alias: z.string().nullish(),
    max_budget: z.number().nullish(),
    tpm_limit: z.number().nullish(),
    rpm_limit: z.number().nullish(),
    duration: keyIsExpired
      ? z
          .string()
          .min(1, t("keyEditor.expiredDurationRequired"))
          .regex(DURATION_PATTERN, t("keyEditor.invalidDuration"))
      : z.string().regex(DURATION_PATTERN, t("keyEditor.invalidDuration")),
    grace_period: z.string().regex(DURATION_PATTERN, t("keyEditor.invalidDuration")),
  };

  return z.object(shape);
};

interface RegenerateKeyModalProps {
  selectedToken: KeyResponse | null;
  visible: boolean;
  onClose: () => void;
  onKeyUpdate?: (updatedKeyData: Partial<KeyResponse>) => void;
}

/** 展示手动轮换表单；输入密钥快照及回调，保存后仅采用服务端持久化值；旧凭据立即撤销。 */
export function RegenerateKeyModal({ selectedToken, visible, onClose, onKeyUpdate }: RegenerateKeyModalProps) {
  const t = useT();
  const { accessToken } = useAuthorized();
  const [regeneratedKey, setRegeneratedKey] = useState<string | null>(null);
  const [isRegenerating, setIsRegenerating] = useState(false);
  const [copied, setCopied] = useState(false);

  const keyIsExpired = isKeyExpired(selectedToken?.expires);
  const schema = useMemo(() => buildSchema(keyIsExpired), [keyIsExpired, t]);
  const form = useZodForm(schema, { defaultValues: EMPTY_VALUES });
  const durationValue = useWatch({ control: form.control, name: "duration" });

  useEffect(() => {
    if (visible && selectedToken && accessToken) {
      const seededValues: RegenerateKeyFormValues = {
        key_alias: selectedToken.key_alias,
        max_budget: selectedToken.max_budget,
        tpm_limit: selectedToken.tpm_limit,
        rpm_limit: selectedToken.rpm_limit,
        duration: selectedToken.duration || "",
        grace_period: "",
      };
      form.reset(seededValues);
    }
  }, [visible, selectedToken, form, accessToken]);

  const newExpiryTime =
    durationValue && DURATION_PATTERN.test(durationValue) ? calculateExpiryPreviewFromDuration(durationValue) : null;

  /** 提交手动轮换；参数为已校验表单，保存后以服务端快照更新父页面，异常保留输入并提示。 */
  const submitRegenerateKey = async (values: RegenerateKeyFormValues) => {
    if (!selectedToken || !accessToken) return;

    const formValues = buildRegenerateKeyPayload(values);
    try {
      const response = await regenerateKeyCall(accessToken, selectedToken.token_id || selectedToken.token, formValues);
      setRegeneratedKey(response.key);
      toast.success(t("Virtual Key regenerated successfully"));

      // 服务端响应是持久化结果，不能以表单值覆盖并掩盖后台忽略字段的问题。
      const updatedKeyData: Partial<KeyResponse> = {
        ...response,
        token: response.token_id || response.token || selectedToken.token,
        key_name: response.key,
        expires: response.expires ?? selectedToken.expires,
      };

      // Update the parent component with new key data
      if (onKeyUpdate) {
        onKeyUpdate(updatedKeyData);
      }

      setIsRegenerating(false);
    } catch (error) {
      setIsRegenerating(false); // Reset regenerating state on error
      console.error("Error regenerating key:", error);
      toast.fromError(error);
    }
  };

  const handleRegenerateKey = () => {
    if (!selectedToken || !accessToken) return;

    setIsRegenerating(true);
    void form.handleSubmit(submitRegenerateKey, () => setIsRegenerating(false))();
  };

  const handleClose = () => {
    setRegeneratedKey(null);
    setIsRegenerating(false);
    setCopied(false);
    form.reset(EMPTY_VALUES);
    onClose();
  };

  const handleCopyKey = () => {
    setCopied(true);
  };

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && handleClose()} disablePointerDismissal>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{t("Regenerate Virtual Key")}</DialogTitle>
        </DialogHeader>
        {regeneratedKey ? (
          <div className="flex flex-col gap-4">
            <Alert variant="warning">
              <TriangleAlert />
              <AlertTitle>{t("Save it now, you will not see it again")}</AlertTitle>
            </Alert>

            <div className="flex flex-col gap-0.5">
              <span className="text-xs text-muted-foreground">{t("Key Alias")}</span>
              <span className="text-sm text-foreground">{selectedToken?.key_alias || t("No alias set")}</span>
            </div>

            <div className="flex flex-col gap-1.5">
              <span className="text-xs text-muted-foreground">{t("Virtual Key")}</span>
              <div className="rounded-md border border-border bg-muted px-4 py-3.5 font-mono text-base break-all text-foreground">
                {regeneratedKey}
              </div>
            </div>
          </div>
        ) : (
          <TooltipProvider>
            <form onSubmit={(event) => event.preventDefault()} noValidate className="mt-1">
              <FieldGroup>
                <FormField control={form.control} name="key_alias" label={t("Key Alias")}>
                  {({ ref, value, ...field }) => <Input {...field} ref={ref} value={value ?? ""} disabled />}
                </FormField>

                <div className="grid grid-cols-3 gap-3">
                  <FormField control={form.control} name="max_budget" label={t("Max Budget (USD)")}>
                    {({ ref, value, onChange, ...field }) => (
                      <Input
                        {...field}
                        ref={ref}
                        type="number"
                        step={0.01}
                        value={value ?? ""}
                        onChange={(event) => onChange(event.target.value === "" ? null : event.target.valueAsNumber)}
                      />
                    )}
                  </FormField>

                  <FormField control={form.control} name="tpm_limit" label={t("TPM Limit")}>
                    {({ ref, value, onChange, ...field }) => (
                      <Input
                        {...field}
                        ref={ref}
                        type="number"
                        value={value ?? ""}
                        onChange={(event) => onChange(event.target.value === "" ? null : event.target.valueAsNumber)}
                      />
                    )}
                  </FormField>

                  <FormField control={form.control} name="rpm_limit" label={t("RPM Limit")}>
                    {({ ref, value, onChange, ...field }) => (
                      <Input
                        {...field}
                        ref={ref}
                        type="number"
                        value={value ?? ""}
                        onChange={(event) => onChange(event.target.value === "" ? null : event.target.valueAsNumber)}
                      />
                    )}
                  </FormField>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <FormField
                    control={form.control}
                    name="duration"
                    label={t("Expire Key")}
                    description={
                      <span className="flex flex-col gap-0.5 text-xs">
                        <span className={keyIsExpired ? "text-destructive" : "text-muted-foreground"}>
                          {t("Current expiry: {value0}{value1}", {
                            value0: selectedToken?.expires
                              ? formatExpiresUtc(selectedToken.expires)
                              : t("keyEditor.neverExpire"),
                            value1: keyIsExpired ? t("keyEditor.expiredSuffix") : "",
                          })}
                        </span>
                        {newExpiryTime && (
                          <span className="text-success">{t("New expiry: {newExpiryTime}", { newExpiryTime })}</span>
                        )}
                      </span>
                    }
                  >
                    {({ ref, ...field }) => <Input {...field} ref={ref} placeholder={t("keyEditor.editPlaceholder")} />}
                  </FormField>

                  <p className="text-sm text-muted-foreground">{t("keyEditor.immediateRotation")}</p>
                </div>
              </FieldGroup>
            </form>
          </TooltipProvider>
        )}
        <DialogFooter>
          {regeneratedKey ? (
            <>
              <Button variant="outline" onClick={handleClose}>
                {t("Close")}
              </Button>
              <CopyToClipboard text={regeneratedKey} onCopy={handleCopyKey}>
                <Button>
                  {copied ? <Check /> : <Copy />}
                  {copied ? t("Copied") : t("Copy Key")}
                </Button>
              </CopyToClipboard>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={handleClose}>
                {t("Cancel")}
              </Button>
              <Button onClick={handleRegenerateKey} disabled={isRegenerating} aria-busy={isRegenerating}>
                <RefreshCw />
                {t("Regenerate")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
