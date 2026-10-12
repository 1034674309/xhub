import fs from "fs";
import path from "path";

const dir = path.resolve(process.env.E2E_RUN_DIR || path.join(__dirname, "../../.e2e/current"), "coverage");

/** 追加一条报告记录；参数为文件名和内容，无返回；调用方为场景记录，创建本轮目录，不访问业务数据。 */
function append(name: string, line: string) {
  fs.mkdirSync(dir, { recursive: true });
  fs.appendFileSync(path.join(dir, name), line.endsWith("\n") ? line : `${line}\n`);
}

/** 用途：追加页面实际路径结果；供独立浏览器场景调用，返回无，仅写当前运行报告。 */
export function recordPage(route: string, result: "pass" | "fail", businessPath = route) {
  append("pages.txt", `page ${result} ${route} ${businessPath}`);
}

/** 保存完整链路结果；参数为描述，无返回；供浏览器场景调用，只追加当前运行证据。 */
export function recordChain(line: string) {
  append("chains.txt", line);
}

export const reportDir = dir;
