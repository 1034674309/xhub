import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { auditDocumentLinks, markdownReferences } from "../src/document-links.mjs";

/** 目的：验证词法审计正确识别含括号路径并忽略代码块伪链接；前置纯 Markdown 夹具；只读取内存，无需清理。 */
test("Marked AST handles parentheses and ignores fenced examples", () => {
  assert.deepEqual(markdownReferences("[guide](guide_(v2).md)\n\n```md\n[false](missing.md)\n```"), [
    { kind: "link", href: "guide_(v2).md" },
  ]);
});

/** 目的：本地忽略报告存在时也不能掩盖 CI 断链；前置双语性能文档，验证报告仅为文字且指南受版本管理；只读，无需清理。 */
test("双语性能文档链接到已跟踪指南而非本地报告", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const tracked = execFileSync("git", ["ls-files", "--error-unmatch", "benchmarks/README.md"], { cwd: root, encoding: "utf8" }).trim();
  assert.equal(tracked, "benchmarks/README.md", "公开压测指南必须包含在全新检出中");
  for (const locale of ["en", "zh-CN"]) {
    const document = readFileSync(new URL(`../../docs/development/${locale}/api-performance-design.md`, import.meta.url), "utf8");
    const references = markdownReferences(document);
    assert.ok(references.some(({ href }) => href === "../../../benchmarks/README.md"), `${locale} 必须提供压测指南入口`);
    assert.ok(references.every(({ href }) => !href.includes("benchmarks/reports/")), `${locale} 不能链接到被忽略的本地报告`);
    assert.ok(document.includes("`benchmarks/reports/staircase/report.md`"), `${locale} 必须保留本地报告路径说明`);
    assert.ok(document.includes("`benchmarks/reports/validation/summary.md`"), `${locale} 必须保留本地验证记录路径说明`);
  }
});

/** 目的：验证公开源文档的相对文件和标题锚点均可解析；前置真实受版本管理文档；只读仓库，无需清理。 */
test("public Markdown links resolve through the Marked AST audit", () => {
  const root = fileURLToPath(new URL("../../", import.meta.url));
  const report = auditDocumentLinks(root);
  assert.ok(report.stats.files > 100);
  assert.ok(report.stats.links > 100);
  assert.deepEqual(report.errors, []);
});
