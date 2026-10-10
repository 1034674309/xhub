import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { auditDocumentLinks, markdownReferences } from "../src/document-links.mjs";

/** 目的：验证词法审计正确识别含括号路径并忽略代码块伪链接；前置纯 Markdown 夹具；只读取内存，无需清理。 */
test("Marked AST handles parentheses and ignores fenced examples", () => {
  assert.deepEqual(markdownReferences("[guide](guide_(v2).md)\n\n```md\n[false](missing.md)\n```"), [
    { kind: "link", href: "guide_(v2).md" },
  ]);
});

/** 目的：验证公开源文档的相对文件和标题锚点均可解析；前置真实受版本管理文档；只读仓库，无需清理。 */
test("public Markdown links resolve through the Marked AST audit", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const report = auditDocumentLinks(root);
  assert.ok(report.stats.files > 100);
  assert.ok(report.stats.links > 100);
  assert.deepEqual(report.errors, []);
});
