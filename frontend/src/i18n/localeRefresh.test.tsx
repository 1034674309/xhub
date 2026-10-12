import React, { memo, useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { useT } from "./I18nProvider";
import { I18nProvider } from "./I18nProvider";
import LanguageSwitcher from "@/components/LanguageSwitcher";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));

/** 模拟 memo 页面容器；参数 children 为页面内容，返回同一子树，无副作用。 */
const Frozen = memo(function Frozen({ children }: { children: React.ReactNode }) {
  return children;
});

/** 模拟调试台标题与草稿；无参数，返回订阅语言的标题和输入框，状态只由用户输入更新。 */
function Title() {
  const t = useT();
  const [draft, setDraft] = useState("");
  return (
    <>
      <h1>{t("login.title")}</h1>
      <input aria-label="draft" value={draft} onChange={(event) => setDraft(event.target.value)} />
    </>
  );
}

describe("locale switch", () => {
  /** 前置真实语言 Provider；验证 memo 内标题切换和空/非空中文草稿往返保留，DOM 由测试框架清理。 */
  it("re-renders memoized text and preserves drafts in both locales", async () => {
      const user = userEvent.setup();
      render(
        <I18nProvider initialLocale="zh-CN">
            <Frozen>
              <Title />
              <LanguageSwitcher />
            </Frozen>
        </I18nProvider>,
      );

      expect(screen.getByRole("heading", { name: "登录" })).toBeInTheDocument();
      expect(screen.getByRole("textbox", { name: "draft" })).toHaveValue("");
      await user.type(screen.getByRole("textbox", { name: "draft" }), "用户的猫");
      await user.click(screen.getByRole("button", { name: "English" }));
      expect(screen.getByRole("heading", { name: "Login" })).toBeInTheDocument();
      expect(screen.getByRole("textbox", { name: "draft" })).toHaveValue("用户的猫");
      await user.click(screen.getByRole("button", { name: "English" }));
      expect(screen.getByRole("textbox", { name: "draft" })).toHaveValue("用户的猫");
      await user.click(screen.getByRole("button", { name: "中文" }));
      expect(screen.getByRole("textbox", { name: "draft" })).toHaveValue("用户的猫");
      expect(screen.getByRole("heading", { name: "登录" })).toBeInTheDocument();
    },
  );
});

