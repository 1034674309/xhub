import fs from "fs";
import path from "path";

export interface DiscoveredPage {
  route: string;
  removed: boolean;
  paths: string[];
}

/** 用途：发现页面并提供可执行路径；参数为页面根目录，返回有序清单。
 * 门禁调用；未知动态路由抛错，只读文件，不把条件 notFound 当成退役。 */
export function discoverPages(appDir = path.resolve(__dirname, "../src/app")): DiscoveredPage[] {
  const pages: DiscoveredPage[] = [];
  /** 递归读取页面文件；参数为目录，无返回，仅填充清单，组目录不构成 URL。 */
  const walk = (dir: string) => {
    for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, ent.name);
      if (ent.isDirectory()) {
        walk(full);
        continue;
      }
      if (ent.name !== "page.tsx") continue;
      const rel = path.relative(appDir, dir);
      const route =
        "/" +
        rel
          .split(path.sep)
          .filter((seg) => seg && !(seg.startsWith("(") && seg.endsWith(")")))
          .join("/");
      const paths = route === "/docs/[[...slug]]" ? ["/docs", "/docs/product/quickstart", "/docs/api/chat-completions"] : [route];
      if (paths.some((value) => value.includes("["))) throw new Error("动态路由缺少业务路径: " + route);
      pages.push({
        route: route === "/" ? "/" : route.replace(/\/$/, ""),
        removed: false,
        paths,
      });
    }
  };
  walk(appDir);
  pages.sort((a, b) => a.route.localeCompare(b.route));
  return pages;
}
