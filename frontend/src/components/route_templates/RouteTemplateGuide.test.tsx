import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";
import { I18nProvider } from "@/i18n";
import RouteTemplateGuide from "./RouteTemplateGuide";

/** 目的：验证帮助入口和弹窗标题、描述完整接入双语；前置本地组件；点击只改变弹窗状态，DOM 自动清理。 */
it.each([
  ["zh-CN", "完整 JSON 配置指南", "严格字段参考、可导入示例、执行顺序与 XHub 差异。"],
  ["en", "Complete JSON configuration guide", "Strict field reference, importable examples, execution order, and XHub differences."],
] as const)("opens localized route-template help in %s", async (locale, button, description) => {
  const user = userEvent.setup();
  render(<I18nProvider initialLocale={locale}><RouteTemplateGuide /></I18nProvider>);
  await user.click(screen.getByRole("button", { name: button }));
  const dialog = screen.getByRole("dialog", { name: button });
  expect(dialog).toHaveTextContent(description);
});
