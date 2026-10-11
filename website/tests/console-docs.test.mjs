import { test } from "node:test";
import assert from "node:assert/strict";
import { consoleDocuments } from "../src/console-docs.mjs";
/** 目的：双端同源文章、帮助、四分类与英文示例；前置真实仓库源，静态生成完整两语言且路径唯一，不写数据库，无需清理。 */
test("console documents synchronize help, categories and API contracts", async () => {
  const {documents, groups} = await consoleDocuments(new URL("../../", import.meta.url).pathname);
  assert.equal(groups.length, 8);
  const native = documents.find((d) => d.lang === "en" && d.source.endsWith("/api/native-bypass.md"));
  assert.equal(native.section, "bypass");
  assert.equal(native.protocol, "bypass");
  assert.ok(documents.filter((d) => d.source.includes("/api/native-")).every((d) => d.section === "bypass"));
  assert.ok(native.markdown.includes("(native-anthropic.md)"));
  assert.ok(documents.some((d) => d.source.endsWith("/api/native-vertex.md")));
  assert.equal(new Set(documents.map((d) => d.source)).size, documents.length);
  for (const lang of ["zh", "en"]) {
    const help = documents.filter((d) => d.lang === lang && d.source.includes("/help/"));
    assert.equal(help.length, 8);
    assert.ok(help.every((d) => !d.markdown.includes("undefined")));
    const providers = help.find((d) => d.source.endsWith("/help/models.md"));
    assert.ok(providers, "供应商操作帮助必须存在");
    // 双语帮助必须配对应语言的真实界面截图，不能在英文文章复用中文图；仅检查生成内容，无业务数据。
    for (const form of ["provider", "deployment"]) {
      assert.ok(providers.markdown.includes(`/assets/provider-guide/${form}-form-${lang}.jpg`), `${lang} 帮助缺少 ${form} 操作截图`);
    }
    for (const term of ["NewAPI", "Suno", "bypass_openai_chat", "RegisterModel", "internal/provider/all/all.go"]) {
      assert.ok(providers.markdown.includes(term), `${lang} 供应商帮助缺少实际契约 ${term}`);
    }
    // 目的：操作帮助聚焦页面接入，禁止重复启动字段；只读生成内容，无需清理。
    assert.doesNotMatch(providers.markdown, /model_list: \[\]|store_model_in_db: true|## UI configuration versus the startup file/);
    assert.doesNotMatch(providers.markdown, /os\.environ\/NEWAPI_KEY|~~~yaml/, `${lang} 帮助不得要求内联供应商密钥或手写部署 YAML`);
    assert.match(documents.find((d) => d.lang === lang && d.source.endsWith("/api/models.md")).markdown, /data\[\]\.id/);
    assert.ok(groups.filter((g) => g.lang === lang).every((g) => g.files.length > 0));
    for (const document of documents.filter((d) => d.lang === lang && d.group === "api")) {
      assert.ok(document.repositorySource.startsWith("docs/api/"));
      const guide = /\/(bypass|native-bypass|errors|fal)\.md$/.test(document.source);
      if (!guide) {
        assert.ok(document.markdown.includes(lang === "en" ? "## Request parameters" : "## 请求参数"), document.source);
        assert.ok(document.markdown.includes(lang === "en" ? "## Success response" : "## 成功响应"), document.source);
      }
      assert.ok(!document.markdown.includes("YOUR_GATEWAY_HOST"));
    }
  }
});
/** 目的：失败输入不悄悄降级成旧文档；前置不存在的仓库路径，构建明确失败，不产生临时文件。 */
test("missing source fails synchronization", async () => {
  await assert.rejects(consoleDocuments("/does-not-exist-xhub"));
});
