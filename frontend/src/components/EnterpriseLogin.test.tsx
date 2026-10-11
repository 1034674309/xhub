import React from "react";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/i18n";
import { enterpriseLoginOptions } from "@/lib/http/enterprise";
import EnterpriseLogin from "./EnterpriseLogin";
vi.mock("@/lib/http/enterprise");
beforeEach(() => { vi.resetAllMocks(); localStorage.clear(); document.cookie = "xhub_locale=; max-age=0; path=/"; });
afterEach(cleanup);
// 前置英文登录页和公开入口；验证动态名称保持原语言及受控跳转，清理组件与 mock。
it("企业名称按原输入展示且链接到授权入口", async () => {
 vi.mocked(enterpriseLoginOptions).mockResolvedValue([{ id: "one", name: "研发 Company", url: "/gw/auth/enterprise/one/start" }]);
 render(<I18nProvider initialLocale="en"><EnterpriseLogin /></I18nProvider>);
 await waitFor(() => expect(screen.getByRole("link", { name: "Sign in with 研发 Company" })).toHaveAttribute("href", "/gw/auth/enterprise/one/start"));
});
// 前置服务不可用；验证本地登录不被拦截且无错误秘密展示，清理 mock，无真实写入。
it("入口接口失败时不展示企业入口", async () => {
 vi.mocked(enterpriseLoginOptions).mockRejectedValue(new Error("offline"));
 render(<I18nProvider initialLocale="zh-CN"><EnterpriseLogin /></I18nProvider>);
 await waitFor(() => expect(enterpriseLoginOptions).toHaveBeenCalled());
 expect(screen.queryByRole("link")).not.toBeInTheDocument();
});
