import { expect, test } from "@playwright/test";

import { execFile } from "node:child_process";

import { readFile } from "node:fs/promises";

import path from "node:path";

import { promisify } from "node:util";

import { GATEWAY, loginAdmin, sessionBearer, stableGoto, t } from "./helpers";
/** 用途：注册独立场景「基准工具使用页面创建密钥并验证并发用量与清理」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC003() {


test("基准工具使用页面创建密钥并验证并发用量与清理", async ({ page }, testInfo) => {
  test.setTimeout(180_000);
  await loginAdmin(page);
  const headers = { Authorization: "Bearer " + await sessionBearer(page) };
  const alias = "e2e-benchmark-" + Date.now();
  let key = "";
  try {
    await page.getByTestId("create-key-button").click();
    await page.getByLabel(t("Key Name")).fill(alias);
    const generated = page.waitForResponse((response) => new URL(response.url()).pathname === "/key/generate" && response.request().method() === "POST");
    await page.getByRole("button", { name: t("pages.apiKeys.createSubmit"), exact: true }).click();
    const response = await generated;
    expect(response.status()).toBe(200);
    key = (await response.json()).key;
    expect(key).toBeTruthy();
    await expect(page.getByRole("dialog").locator("pre").filter({ hasText: /sk-/ })).toBeVisible();
    await page.keyboard.press("Escape");
    const previous = await page.request.get(GATEWAY + "/spend/logs/ui?model=gpt-4o-mini&status_filter=success&page_size=100", { headers });
    expect(previous.status()).toBe(200);
    const previousIDs = new Set<string>((await previous.json()).data.map((log: { request_id: string }) => log.request_id));
    const output = testInfo.outputPath("benchmark");
    // 密钥只通过子进程环境传递，不写命令参数、报告或浏览器持久存储。
    await promisify(execFile)("go", ["run", "./benchmarks", "-base-url", GATEWAY, "-model", "gpt-4o-mini", "-concurrency", "1,4", "-requests", "6", "-warmup", "0", "-max-error-rate", "0", "-out", output], {
      cwd: path.resolve(__dirname, "../.."), env: { ...process.env, XHUB_BENCHMARK_KEY: key }, timeout: 120_000,
    });
    const report = JSON.parse(await readFile(path.join(output, "report.json"), "utf8"));
    expect(report.Passed).toBe(true);
    expect(report.Stages).toHaveLength(8);
    for (const stage of report.Stages) {
      expect(stage.attempted).toBe(6);
      expect(stage.failed, "CLI 场景应得到真实有效响应：" + stage.scenario).toBe(0);
      if (stage.scenario === "stream") expect(stage.ttft.samples).toBe(6);
      if (stage.scenario === "auth-reject") expect(stage.http_statuses["401"]).toBe(6);
    }
    // 管理接口轮询真实落库结果，随后浏览器打开同一条日志，避免只验证压测报告。
    let logs: { request_id: string; prompt_tokens: number; completion_tokens: number }[] = [];
    await expect.poll(async () => {
      const stored = await page.request.get(GATEWAY + "/spend/logs/ui?model=gpt-4o-mini&status_filter=success&page_size=100", { headers });
      expect(stored.status()).toBe(200);
      logs = (await stored.json()).data.filter((log: { request_id: string }) => !previousIDs.has(log.request_id));
      return logs.length;
    }, { message: "24 次真实普通/流式推理应全部持久化", timeout: 15_000 }).toBe(24);
    for (const log of logs) {
      expect(log.prompt_tokens + log.completion_tokens, "推理日志应记录真实 token").toBeGreaterThan(0);
    }
    await stableGoto(page, "/logs");
    await page.getByPlaceholder(/按 ID 搜索日志|Search logs by ID/).fill(logs[0].request_id);
    const row = page.getByRole("row").filter({ hasText: logs[0].request_id });
    await expect(row).toBeVisible();
    await row.click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await stableGoto(page, "/api-keys");
    await page.getByText(alias, { exact: true }).first().click();
    await page.getByRole("button", { name: t("More key actions") }).click();
    await page.getByRole("menuitem", { name: t("Delete Key") }).click();
    await page.getByPlaceholder(alias).fill(alias);
    await page.getByRole("button", { name: t("common.delete") }).click();
    await expect(page.getByText(alias)).toHaveCount(0);
    const rejected = await page.request.post(GATEWAY + "/v1/chat/completions", {
      headers: { Authorization: "Bearer " + key }, data: { model: "gpt-4o-mini", messages: [{ role: "user", content: "deleted benchmark key" }] },
    });
    expect(rejected.status(), "页面删除密钥后数据面应立即拒绝").toBe(401);
  } finally {
    if (key) {
      const removed = await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [key] } });
      // 页面已删除时后台返回 404；两种状态均证明本次密钥已不存在。
      expect([200, 404], "清理本次基准密钥或确认已删除").toContain(removed.status());
    }
  }
});
}
