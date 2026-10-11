import { useT } from "@/i18n";
import { pricingRates, type CatalogRow } from "./modelEditorPricing";

const units: Record<string, string> = {
  token: "providerGuideUI.millionTokens",
  picture: "providerGuideUI.pictureUnit",
  second: "providerGuideUI.secondUnit",
  query: "providerGuideUI.queryUnit",
};
const sides: Record<string, string> = {
  input: "providerGuideUI.input",
  output: "providerGuideUI.output",
  cache_read: "providerGuideUI.cacheRead",
  cache_write: "providerGuideUI.cacheWrite",
  batch_input: "providerGuideUI.batchInput",
  batch_output: "providerGuideUI.batchOutput",
};
const windows: Record<string, string> = {
  all: "providerGuideUI.allDay",
  peak: "providerGuideUI.peak",
  offpeak: "providerGuideUI.offpeak",
};

/** 展示目录费率；参数为可选目录行，返回本地化价格表或空提示，供模型编辑器调用，无副作用。 */
export default function PricingTable({ row }: { row?: CatalogRow }) {
  const t = useT();
  const rates = pricingRates(row);
  if (!rates.length)
    return (
      <p className="rounded-lg border border-dashed p-5 text-sm text-muted-foreground">
        {t("providerGuideUI.noPrice")}
      </p>
    );
  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-left text-sm">
        <thead className="bg-muted/50">
          <tr>
            <th className="p-3 font-medium">{t("providerGuideUI.item")}</th>
            <th className="p-3 font-medium">{t("providerGuideUI.window")}</th>
            <th className="p-3 text-right font-medium">{t("providerGuideUI.usdPrice")}</th>
          </tr>
        </thead>
        <tbody>
          {rates.map((rate, index) => (
            <tr key={index} className="border-t">
              <td className="p-3">
                {sides[rate.side] ? t(sides[rate.side]) : rate.side}
                {rate.variant && rate.variant !== "uncached" ? " · " + rate.variant : ""}
              </td>
              <td className="p-3 text-muted-foreground">
                {windows[rate.window] ? t(windows[rate.window]) : rate.window}
              </td>
              <td className="whitespace-nowrap p-3 text-right tabular-nums">
                {Number((rate.usd * (rate.measure === "token" ? 1e6 : 1)).toPrecision(12))} /{" "}
                {units[rate.measure] ? t(units[rate.measure]) : rate.measure}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
