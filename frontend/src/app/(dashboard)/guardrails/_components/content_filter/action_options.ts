import { t } from "@/i18n";
export const ACTION_ITEMS = [
  { value: "BLOCK", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Block"); } },
  { value: "MASK", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Mask"); } },
] as const;

export const SEVERITY_ITEMS = [
  { value: "high", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("High"); } },
  { value: "medium", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Medium"); } },
  { value: "low", /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Low"); } },
] as const;
