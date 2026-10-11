import { useT } from "@/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { PriceRate } from "./modelEditorPricing";
const measures = {
  token: "providerGuideUI.millionTokens",
  picture: "providerGuideUI.pictureUnit",
  second: "providerGuideUI.secondUnit",
  query: "providerGuideUI.queryUnit",
};
const sides = {
  input: "providerGuideUI.input",
  output: "providerGuideUI.output",
  cache_read: "providerGuideUI.cacheRead",
  cache_write: "providerGuideUI.cacheWrite",
  batch_input: "providerGuideUI.batchInput",
  batch_output: "providerGuideUI.batchOutput",
};
const windows = { all: "providerGuideUI.allDay", offpeak: "providerGuideUI.offpeak", peak: "providerGuideUI.peak" };
const selectClass = "h-9 w-full rounded-md border bg-background px-2 text-sm";
/** 编辑混合计费规则；参数为费率和更新回调，返回本地化表单，供模型编辑器使用；不写入后台。 */
export default function RateEditor({
  rates,
  onChange,
}: {
  rates: PriceRate[];
  onChange: (rates: PriceRate[]) => void;
}) {
  const t = useT();
  /** 合并指定行的变更；参数为行号及字段，返回无，通过回调更新草稿，不修改原数组。 */
  const update = (index: number, patch: Partial<PriceRate>) =>
    onChange(rates.map((rate, position) => (position === index ? { ...rate, ...patch } : rate)));
  return (
    <div className="space-y-4">
      <p className="text-xs leading-5 text-muted-foreground">{t("providerGuideUI.rateIntro")}</p>
      {rates.map((rate, index) => (
        <fieldset key={index} className="grid gap-3 rounded-lg border p-3 sm:grid-cols-2">
          <label className="space-y-1 text-xs">
            {t("providerGuideUI.measure")}
            <select
              aria-label={t("providerGuideUI.rateUnitLabel", { index: index + 1 })}
              className={selectClass}
              value={rate.measure}
              onChange={(event) => update(index, { measure: event.target.value })}
            >
              {Object.entries(measures).map(([id, label]) => (
                <option key={id} value={id}>
                  {t(label)}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            {t("providerGuideUI.item")}
            <select
              aria-label={t("providerGuideUI.rateItemLabel", { index: index + 1 })}
              className={selectClass}
              value={rate.side}
              onChange={(event) => update(index, { side: event.target.value })}
            >
              {Object.entries(sides).map(([id, label]) => (
                <option key={id} value={id}>
                  {t(label)}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            {t("providerGuideUI.rateWindow")}
            <select
              aria-label={t("providerGuideUI.rateWindowLabel", { index: index + 1 })}
              className={selectClass}
              value={rate.window}
              onChange={(event) => update(index, { window: event.target.value })}
            >
              {Object.entries(windows).map(([id, label]) => (
                <option key={id} value={id}>
                  {t(label)}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-xs">
            {t("providerGuideUI.ratePrice", { unit: t(measures[rate.measure as keyof typeof measures]) })}
            <Input
              aria-label={t("providerGuideUI.ratePriceLabel", { index: index + 1 })}
              type="number"
              min="0"
              step="any"
              value={
                Number.isFinite(rate.usd)
                  ? Number((rate.usd * (rate.measure === "token" ? 1e6 : 1)).toPrecision(12))
                  : ""
              }
              onChange={(event) =>
                update(index, {
                  usd:
                    event.target.value === "" ? NaN : Number(event.target.value) / (rate.measure === "token" ? 1e6 : 1),
                })
              }
            />
          </label>
          <label className="space-y-1 text-xs">
            {t("providerGuideUI.variant")}
            <Input
              aria-label={t("providerGuideUI.rateVariantLabel", { index: index + 1 })}
              value={rate.variant ?? ""}
              onChange={(event) => update(index, { variant: event.target.value })}
              placeholder={t("providerGuideUI.generalRate")}
            />
          </label>
          <Button
            type="button"
            variant="ghost"
            className="self-end text-destructive"
            onClick={() => onChange(rates.filter((_, position) => index !== position))}
          >
            {t("providerGuideUI.removeRate")}
          </Button>
        </fieldset>
      ))}
      <Button
        type="button"
        variant="outline"
        onClick={() => onChange([...rates, { measure: "token", side: "input", window: "all", usd: NaN }])}
      >
        {t("providerGuideUI.addRate")}
      </Button>
    </div>
  );
}
