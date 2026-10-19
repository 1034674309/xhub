import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

/** 验证站点声明、锁文件及安装说明覆盖 Mermaid 的最低运行环境；前置已安装依赖，结果为一致的 22.12.0 下限，纯读取无需清理。 */
test("网站最低 Node 版本与 Mermaid 12 保持一致", () => {
  const manifest = JSON.parse(readFileSync(new URL("../package.json", import.meta.url)));
  const lock = JSON.parse(readFileSync(new URL("../package-lock.json", import.meta.url)));
  const mermaid = JSON.parse(readFileSync(new URL("../node_modules/mermaid/package.json", import.meta.url)));
  assert.equal(manifest.engines.node, ">=22.12.0");
  assert.equal(lock.packages[""].engines.node, manifest.engines.node);
  assert.equal(mermaid.engines.node, ">=22.12.0");
  assert.match(readFileSync(new URL("../README.md", import.meta.url), "utf8"), /Node\.js 22\.12\.0/);
});
