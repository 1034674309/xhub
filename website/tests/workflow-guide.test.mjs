import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { renderMarkdown } from "../src/markdown.mjs";

/** 目的：所有核心开发主题提供实际代码或操作入口，API 请求先于字段表；前置真实双语源，纯读取不清理数据。 */
test("开发文档提供操作入口，接口文档先给请求", async () => {
  const topics = ["architecture", "configuration", "guardrails", "implementation", "model-endpoints", "model-marketplace", "operations", "permissions", "pricing", "provider-catalog", "provider-credential-forms", "quota-chain", "routing", "routing-groups", "runtime", "testing"];
  for (const locale of ["en", "zh-CN"]) {
    for (const topic of topics) {
      const source = await readFile(new URL(`../../docs/development/${locale}/${topic}.md`, import.meta.url), "utf8");
      assert.ok(source.includes("~~~") || source.includes("!["), `${locale}/${topic}: 必须有代码、命令或操作截图`);
      assert.doesNotMatch(source, /English overview is available|See the Chinese document for details|go test \.\/internal\/dataplane -count/);
    }
    for (const endpoint of ["chat", "responses", "images", "ark", "models"]) {
      const source = await readFile(new URL(`../../docs/api/${locale}/${endpoint}.md`, import.meta.url), "utf8");
      const example = source.indexOf(locale === "en" ? "## Request example" : "## 请求示例");
      const fields = source.indexOf(locale === "en" ? "## Request parameters" : "## 请求参数");
      assert.ok(example >= 0 && example < fields, `${locale}/${endpoint}: 请求示例必须在参数表前`);
    }
  }
});

/** 目的：供应商双语帮助给出代码扩展步骤并移除启动配置段；前置为仓库文档，验证注册、导入、执行链及渲染，无写入无需清理。 */
test("供应商帮助聚焦页面接入与新协议开发", async () => {
  for (const [locale, heading] of [["zh-CN", "自己添加一个 provider"], ["en", "Add your own provider"]]) {
    const path = `docs/development/${locale}/provider-catalog.md`;
    const source = await readFile(new URL(`../../${path}`, import.meta.url), "utf8");
    assert.ok(!source.includes("config.yaml"), `${locale}: 不应保留启动配置段`);
    assert.ok(!source.includes("| litellm"), `${locale}: 不应展示内部配置表`);
    for (const required of [heading, "registry.RegisterTransport", "internal/provider/acme/", "internal/provider/all/all.go", "internal/dataplane/transport"]) {
      assert.ok(source.includes(required), `${locale}: 缺少接入步骤 ${required}`);
    }
    const { html } = renderMarkdown(source, path, "provider-catalog.md", new Map());
    assert.ok(html.includes(heading), `${locale}: 开发步骤必须渲染到网站`);
    assert.ok(html.includes("provider-form-"), `${locale}: 页面操作配图应保留`);
    assert.ok(html.includes("hljs-keyword"), `${locale}: Go 代码必须语法高亮`);
    assert.equal((source.match(/~~~go/g) || []).length, 3, `${locale}: 保留三个精简代码模块`);
  }
});

/** 目的：高亮不改变代码原文且不引入脚本；前置正常、未知语言和恶意正文，验证着色、边界与转义，无副作用无需清理。 */
test("代码高亮保留原文并安全处理未知语言", () => {
  const go = renderMarkdown("~~~go\npackage acme\nvar name = \"<script>alert(1)</script>\"\n~~~", "docs/example.md", "example.md", new Map()).html;
  assert.ok(go.includes("hljs-keyword"));
  assert.ok(!go.includes("<script>"));
  const unknown = renderMarkdown("~~~unknown-language\n<tag>& value\n~~~", "docs/example.md", "example.md", new Map()).html;
  assert.ok(unknown.includes("&lt;tag&gt;&amp; value"));
  assert.ok(!unknown.includes("hljs-keyword"));
  const empty = renderMarkdown("~~~go\n\n~~~", "docs/example.md", "example.md", new Map()).html;
  assert.ok(empty.includes("language-go"));
});

/** 目的：双语操作指南使用独立实拍并可打开原图；前置仓库文档与 JPEG，验证语言配对、真实文件、渲染和旧说明移除，纯读取无需清理。 */
test("客户指南的调试与用量实拍分别使用中英文资源", async () => {
  for (const [file, locale] of [["user-guide.md", "en"], ["user-guide.zh-CN.md", "zh"]]) {
    const source = await readFile(new URL(`../../docs/${file}`, import.meta.url), "utf8");
    for (const name of ["playground", "usage"]) {
      const path = `assets/workflow-guide/${name}-${locale}.jpg`;
      assert.ok(source.includes(`](${path})](${path})`), `${file}: ${name} 必须提供原图链接`);
      assert.ok(!source.includes(`workflow-guide/${name}-${locale === "en" ? "zh" : "en"}.jpg`), "不能混用截图语言");
      const bytes = await readFile(new URL(`../../docs/${path}`, import.meta.url));
      assert.equal(bytes.readUInt16BE(0), 0xffd8, "资源必须是真实 JPEG");
      const { html } = renderMarkdown(source, `docs/${file}`, file, new Map());
      assert.ok(html.includes(`${name}-${locale}.jpg`), "实际渲染必须保留图片");
    }
    assert.ok(source.includes("Reply with the word ready."), "中文和英文指南提示词均为英文");
    assert.ok(!source.includes("RPM/TPM limits remain available when editing"));
    assert.ok(!source.includes("RPM/TPM 限制在账号有权限时仍可通过编辑"));
  }
});
