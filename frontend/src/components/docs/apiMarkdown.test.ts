import { describe, expect, it } from "vitest";
import { docPages } from "./catalog";
import { readApiMarkdown } from "./apiMarkdown";

describe("API Markdown sources", () => {
  /** 目的：逐篇检查真实双语源文件不重复标题；前置目录中的独立 Markdown，正文保留有效章节，禁止空概述和标题占位段；只读文件，无需清理数据。 */
  it.each(docPages.filter((page) => page.group === "api").map((page) => page.id))(
    "keeps one article title in %s",
    (id) => {
      for (const [locale, markdown] of Object.entries(readApiMarkdown(id))) {
        const lines = markdown.split("\n");
        const title = lines[0].replace(/^# /, "");
        expect(
          lines.filter((line) => line === title || line === "# " + title || line === "### " + title),
          `${id}/${locale}: 标题不应作为正文占位重复`,
        ).toEqual(["# " + title]);
        expect(markdown, `${id}/${locale}: 必须保留实质章节`).toMatch(/^## .+/m);
        expect(markdown, `${id}/${locale}: 不应出现空概述章节`).not.toMatch(/## (?:Overview|接口概述)\s+##/);
      }
    },
  );
  /** 目的：验证非法文件标识不能读取目录外文件；前置未信任输入，空值、路径穿越和大写均拒绝；无文件写入或数据清理。 */
  it.each(["", "../ark", "Ark"])("rejects invalid ID %s", (id) => {
    expect(() => readApiMarkdown(id)).toThrow("Invalid API document ID");
  });
});
