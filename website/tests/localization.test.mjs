import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync } from "node:fs";
import { documentLanguage, localizedSource } from "../src/localization.mjs";
import { homepage } from "../src/templates.mjs";

/** 目的：验证语言目录及历史后缀判断；前置为内存路径，返回正确语言，无文件写入和清理。 */
test("language folders and existing module naming conventions resolve", () => {
  for (const source of ["docs/development/en/routing.md", "docs/api/en/models.md", "docs/getting-started.md", "internal/router/readme.md"]) assert.equal(documentLanguage(source), "en", source);
  for (const source of ["docs/development/zh-CN/routing.md", "docs/api/zh-CN/models.md", "internal/router/readme_cn.md"]) assert.equal(documentLanguage(source), "zh", source);
});

/** 目的：验证双向同主题切换、缺失翻译与非法语言；前置为已知路径集合，不允许退回首页，无需清理。 */
test("counterpart lookup preserves topic and rejects unavailable languages", () => {
  const en = "docs/development/en/routing.md";
  const zh = "docs/development/zh-CN/routing.md";
  const known = new Set([en, zh]);
  assert.equal(localizedSource(en, "zh", known), zh);
  assert.equal(localizedSource(zh, "en", known), en);
  assert.equal(localizedSource(en, "en", known), en);
  assert.equal(localizedSource(en, "zh", new Set([en])), null);
  assert.throws(() => localizedSource(en, "fr", known), TypeError);
});

/** 目的：验证真实目录没有遗漏或多余主题，首页链接使用对应语言目录；只读源文件和内存 HTML，无需清理。 */
test("development editions have matching filenames and localized homepage links", () => {
  const files = (lang) => readdirSync(new URL("../../docs/development/" + lang + "/", import.meta.url)).filter((name) => name.endsWith(".md")).sort();
  assert.deepEqual(files("en"), files("zh-CN"));
  assert.ok(files("en").length >= 26);
  for (const lang of ["en", "zh"]) {
    const html = homepage(lang);
    assert.ok(html.includes("development/" + (lang === "zh" ? "zh-CN" : "en") + "/runtime.html"));
    assert.ok(!html.includes(".en.html"));
    assert.ok(!html.includes("development/" + (lang === "zh" ? "en" : "zh-CN") + "/"));
  }
});
