import fs from "fs";
import path from "path";
import { discoverPages } from "./routes";
import { reportDir } from "./report";

/** 用途：读取当前运行记录，缺文件返回空；供退出门禁使用，只读文件。 */
function readLines(name: string): string[] {
  const file = path.join(reportDir, name);
  if (!fs.existsSync(file)) return [];
  return fs
    .readFileSync(file, "utf8")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

/** 用途：保存页面与目录路由报告；拒绝缺失或失败结果，无业务写入。场景结果由 reporter 核验。 */
export default function globalTeardown() {
  const pages = new Map<string, string>();
  const paths = new Map<string, string>();
  for (const line of readLines("pages.txt")) {
    const [, result, route, businessPath] = line.split(" ");
    if (businessPath) paths.set(businessPath, paths.get(businessPath) === "fail" ? "fail" : result);
    if (route) pages.set(route, pages.get(route) === "fail" ? "fail" : result);
  }
  const discovered = discoverPages();
  const missing = discovered.filter((item) => pages.get(item.route) !== "pass" || item.paths.some(route => paths.get(route) !== "pass")).map((item) => item.route);
  const routes = readLines("routes.txt");
  const routeFails = routes.filter((line) => line.startsWith("route fail")).length;
  const chains = readLines("chains.txt");
  const body = [
    "console routes",
    ...discovered.map((item) => `page ${pages.get(item.route) || "missing"} ${item.route}`),
    `console_routes=${discovered.length}`,
    "",
    "catalog routes",
    ...routes,
    `catalog_routes=${routes.length}`,
    `misaligned=${routeFails}`,
    "",
    ...chains,
    "",
  ].join("\n");
  const out = path.join(reportDir, "e2e-report.txt");
  fs.writeFileSync(out, body);
  if (process.env.E2E_FULL_COVERAGE === "1" && missing.length > 0) {
    throw new Error(`report missing console routes: ${missing.join(", ")}`);
  }
  if (process.env.E2E_FULL_COVERAGE === "1" && (!routes.length || routeFails > 0)) {
    throw new Error("目录路由缺失或失败");
  }
}
