import test from "node:test";
import assert from "node:assert/strict";
import { documentPage, homepage } from "../src/templates.mjs";
import { readFile } from "node:fs/promises";

/** 目的：共用翻译文件的帮助文章仍指向独立地址；前置双语目录与重复源码映射，验证正常、缺失文章边界，无写入无需清理。 */
test("帮助目录按文章定位而不是共享翻译文件定位", () => {
  for (const lang of ["zh", "en"]) {
    const documents = ["index", "help/troubleshooting", "tools-agents-clients/overview", "billing/usage-based-billing", "api/models"].map((path) => ({ source: `center/${lang}/${path}.md`, title: path, lang, repositorySource: "shared.ts" }));
    const source = lang === "en" ? "README.md" : "README.zh-CN.md";
    documents.push({ source, title: "Docs", lang, repositorySource: `docs/${source}` });
    const html = documentPage({ source, title: "Docs", html: "", headings: [], documents, lang, alternate: source });
    const sidebar = html.match(/<aside class="docs-sidebar"[\s\S]*?<\/aside>/)[0];
    for (const path of ["tools-agents-clients/overview", "billing/usage-based-billing", "api/models"]) {
      assert.equal(sidebar.split(`center/${lang}/${path}.html`).length - 1, 1, `${lang}/${path} 应独立出现一次`);
    }
    assert.doesNotMatch(sidebar, /api\/errors.html/, "未登记文章不能生成死链");
    assert.doesNotMatch(html, /English overview/);
  }
});

/** 目的：首页提供四个双语入口且落地页按任务分层；前置真实模板与文档，验证开发入口和过时状态移除，纯读取无需清理。 */
test("双语文档入口区分安装、使用、API 和开发", async () => {
  for (const [lang, locale, file] of [["zh", "zh-CN", "README.zh-CN.md"], ["en", "en", "README.md"]]) {
    const html = homepage(lang);
    assert.equal((html.match(/class="doc-card"/g) || []).length, 4);
    assert.ok(html.includes(`docs/development/${locale}/README.html`));
    assert.ok(html.includes(`docs/api/${locale}/models.html`));
    const source = await readFile(new URL(`../../docs/${file}`, import.meta.url), "utf8");
    assert.equal((source.match(/^## /gm) || []).length, 3);
    assert.doesNotMatch(source, /not implemented|尚未实现|Suno protocol development/);
    if (lang === "en") assert.doesNotMatch(source.replace("简体中文", ""), /[\u4e00-\u9fff]/);
  }
});
