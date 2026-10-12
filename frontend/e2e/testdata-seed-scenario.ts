import { execFileSync } from "node:child_process";

import fs from "node:fs";

import os from "node:os";

import path from "node:path";

import { expect, test } from "@playwright/test";

import { GATEWAY, MASTER, loginAdmin, sessionBearer, stableGoto, watchGateway } from "./helpers";
/** 用途：注册独立场景「testdata 仅构造资源，控制台可见且不执行 Codex 或媒体模型」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC244() {


test("testdata 仅构造资源，控制台可见且不执行 Codex 或媒体模型", async ({ page }) => {
  // 建数需要空租户环境，必须单独选择本文件；完整浏览器套件的其他用例会先创建组织。
  test.skip(process.env.E2E_TESTDATA_SEED !== "1", "请使用 E2E_TESTDATA_SEED=1 单独运行 testdata-seed.spec.ts");
  test.setTimeout(120_000);
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "xhub-testdata-"));
  try {
    try {
      execFileSync("python3", [path.resolve(__dirname, "../../scripts/e2e.py"),
      "--gateway", GATEWAY, "--directory", directory, "--seed-only"], {
      env: { ...process.env, E2E_DATASET_ADMIN: "admin", E2E_DATASET_PASSWORD: MASTER,
        FENNO_AI_API_KEY: "", QINIU_API_KEY: "" }, timeout: 90_000,
      });
    } catch (error) {
      const reportPath = path.join(directory, "report.json");
      const detail = fs.existsSync(reportPath) ? JSON.parse(fs.readFileSync(reportPath, "utf8")).error : String(error);
      throw new Error("testdata 建数失败：" + detail);
    }
    const access = JSON.parse(fs.readFileSync(path.join(directory, "access.json"), "utf8"));
    const report = JSON.parse(fs.readFileSync(path.join(directory, "report.json"), "utf8"));
    expect(access.keys, "应构造 81 把成员个人密钥").toHaveLength(81);
    expect(access.admin_key).toMatchObject({ owner_type: "personal", key_alias: "管理员个人密钥", team_id: null });
    expect(report.total_key_count).toBe(82);
    expect(report.status).toBe("seeded");
    expect(report.calls).toEqual([]);
    expect(report.probe_attempts).toEqual([]);
    expect(report.agent_conversations).toBeUndefined();
    expect(report.media_tasks).toBeUndefined();
    const guard = watchGateway(page);
    await loginAdmin(page);
    await stableGoto(page, "/api-keys");
    await expect(page.getByText("管理员个人密钥", { exact: true }).first()).toBeVisible();
    const logs = await page.request.get(GATEWAY + "/spend/logs/ui?page_size=200", {
      headers: { Authorization: "Bearer " + await sessionBearer(page) },
    });
    expect(logs.status()).toBe(200);
    expect((await logs.json()).data, "建数不应产生任何数据面调用日志").toEqual([]);
    await stableGoto(page, "/organizations");
    for (const organization of access.organizations) {
      await expect(page.getByText(organization.name, { exact: true }).first()).toBeVisible();
    }
    await stableGoto(page, "/models-and-endpoints");
    await page.getByRole("tab", { name: "全部模型", exact: true }).click();
    await expect(page.getByText("gpt-5.6-sol", { exact: true }).first()).toBeVisible();
    await expect(page.getByText("z-ai/glm-5", { exact: true }).first()).toBeVisible();
    guard.assertOk();
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});
}
