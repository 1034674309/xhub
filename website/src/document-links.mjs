import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { Marked } from "marked";
import { headingSlug } from "./core.mjs";

const excluded = /\/(?:testdata|assets|model-marketplace-proof)\/|\/(?:AGENTS|CLAUDE)\.md$|project-review|verification|validation-report/;

/** 返回公开范围内已跟踪与未忽略的新 Markdown；参数为仓库根目录，过滤已删除文件并去重，不写文件，供构建及链接审计共用。 */
export function publicMarkdownSources(root) {
  return [...new Set(execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard", "-z", "README.md", "README.zh-CN.md", "docs", "internal", "cmd/regression"], { cwd: root, encoding: "utf8" })
    .split("\0")
    .filter((source) => /\.md$/i.test(source) && !excluded.test(source) && existsSync(path.join(root, source))))]
    .sort();
}

/** 递归读取 Marked 词法树中的链接和图片；代码块没有子 token，因此不会产生误报。 */
export function markdownReferences(markdown) {
  const references = [];
  const visit = (value) => {
    if (!value || typeof value !== "object") return;
    if ((value.type === "link" || value.type === "image") && typeof value.href === "string") {
      references.push({ kind: value.type, href: value.href });
    }
    for (const [key, child] of Object.entries(value)) {
      if (key === "links" || key === "href" || key === "raw" || key === "text") continue;
      if (Array.isArray(child)) child.forEach(visit);
      else visit(child);
    }
  };
  new Marked({ gfm: true }).lexer(markdown).forEach(visit);
  return references;
}

/** 计算文档中实际可链接的标题锚点；重复标题按网站渲染规则追加序号。 */
function documentAnchors(markdown) {
  const anchors = new Set();
  const counts = new Map();
  const visit = (value) => {
    if (!value || typeof value !== "object") return;
    if (value.type === "heading" && typeof value.text === "string") {
      const base = headingSlug(value.text);
      const count = counts.get(base) || 0;
      counts.set(base, count + 1);
      anchors.add(count ? base + "-" + count : base);
    }
    for (const [key, child] of Object.entries(value)) {
      if (key === "links" || key === "raw" || key === "text") continue;
      if (Array.isArray(child)) child.forEach(visit);
      else visit(child);
    }
  };
  new Marked({ gfm: true }).lexer(markdown).forEach(visit);
  return anchors;
}

/** 审计公开 Markdown 的真实词法链接；返回统计与逐项错误，不发网络请求。 */
export function auditDocumentLinks(root) {
  const sources = publicMarkdownSources(root);
  const markdownBySource = new Map(sources.map((source) => [source, readFileSync(path.join(root, source), "utf8")]));
  const anchors = new Map([...markdownBySource].map(([source, markdown]) => [source, documentAnchors(markdown)]));
  const stats = { files: sources.length, links: 0, images: 0, externalIgnored: 0, anchorsChecked: 0, broken: 0 };
  const errors = [];
  for (const [source, markdown] of markdownBySource) {
    for (const reference of markdownReferences(markdown)) {
      if (reference.kind === "image") stats.images += 1;
      else stats.links += 1;
      const href = reference.href.trim();
      if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(href)) { stats.externalIgnored += 1; continue; }
      const hashAt = href.indexOf("#");
      const queryAt = href.indexOf("?");
      const end = [hashAt, queryAt].filter((index) => index >= 0).reduce((a, b) => Math.min(a, b), href.length);
      let file = href.slice(0, end);
      const fragment = hashAt >= 0 ? href.slice(hashAt + 1).split("?")[0] : "";
      try { file = decodeURIComponent(file); } catch { errors.push({ source, href, reason: "invalid URL encoding" }); continue; }
      const target = file ? path.posix.normalize(path.posix.join(path.posix.dirname(source), file)) : source;
      const targetPath = path.join(root, target);
      if (!existsSync(targetPath)) { errors.push({ source, href, target, reason: "missing target" }); continue; }
      if (fragment && /\.md$/i.test(target)) {
        stats.anchorsChecked += 1;
        let decoded;
        try { decoded = headingSlug(decodeURIComponent(fragment)); } catch { errors.push({ source, href, target, reason: "invalid anchor encoding" }); continue; }
        if (!anchors.get(target)?.has(decoded)) errors.push({ source, href, target, reason: "missing anchor" });
      }
    }
  }
  stats.broken = errors.length;
  return { generatedAt: new Date().toISOString(), parser: "marked lexer AST", scope: "tracked website Markdown plus README.md and README.zh-CN.md", stats, errors };
}
