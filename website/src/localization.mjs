/** 根据仓库路径判断文档语言；参数为源路径，返回 zh/en；构建与单元测试调用，无文件副作用。 */
export function documentLanguage(source) {
  if (/\.en\.md$|\/en\//.test(source)) return "en";
  if (/zh-CN|_cn\.md$|^docs\/development\/|^docs\/(documentation-center|quota-management)\.md$/.test(source)) return "zh";
  return "en";
}

/** 为文档查找同一主题的指定语言源；参数为路径、目标语言和存在路径集合，返回路径或 null；绝不以首页代替缺失翻译。 */
export function localizedSource(source, lang, known) {
  if (!["zh", "en"].includes(lang)) throw new TypeError("不支持的文档语言");
  if (documentLanguage(source) === lang && known.has(source)) return source;
  const candidates = lang === "en"
    ? [source.replace("/zh-CN/", "/en/"), source.replace(".zh-CN.md", ".md"), source.replace("_cn.md", ".md"), source.replace(/\.md$/, ".en.md")]
    : [source.replace("/en/", "/zh-CN/"), source.replace(".en.md", ".md"), source.replace(/\.md$/, ".zh-CN.md"), source.replace(/\.md$/, "_cn.md")];
  return candidates.find((candidate) => known.has(candidate) && documentLanguage(candidate) === lang) || null;
}
