import { describe, expect, it } from "vitest";
import { translate } from "./translate";

describe("LLM 技术术语", () => {
  /** 前置中英翻译目录；验证所有 LLM 标签保留技术缩写且无错误学位译名；纯函数无需清理。 */
  it.each([
    "Continue to LLM", "Incoming LLM Request", "LLM Classifier",
    "LLM Judge Results", "LLM response:", "LLM spend",
    "Original LLM Cost:", "Underlying LLM",
  ])("preserves LLM in %s", (key) => {
    for (const locale of ["zh-CN", "en"] as const) {
      expect(translate(locale, key)).toContain("LLM");
      expect(translate(locale, key)).not.toContain("法学硕士");
    }
  });
  /** 前置费用汇总目录；验证中文正文完整本地化、英文不变，不改变金额；无需清理。 */
  it("localizes cost labels without translating LLM", () => {
    expect(translate("zh-CN", "Original LLM Cost:")).toBe("LLM 原始费用：");
    expect(translate("zh-CN", "Final Calculated Cost:")).toBe("最终计算费用：");
    expect(translate("en", "Original LLM Cost:")).toBe("Original LLM Cost:");
  });
});
