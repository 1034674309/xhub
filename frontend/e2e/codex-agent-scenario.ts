import { expect, test } from "@playwright/test";

import { execFileSync } from "node:child_process";

import { mkdtempSync, rmSync } from "node:fs";

import { tmpdir } from "node:os";

const { verifyCodexLogs } = require("../../e2e/codex_browser.cjs");

import path from "node:path";

import { GATEWAY, loginAdmin, sessionBearer } from "./helpers";
/** 用途：注册独立场景「Codex agent simulator three turns and browser logs」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC004() {


test("Codex agent simulator three turns and browser logs", async ({ page }) => {
  test.setTimeout(120000);
  await loginAdmin(page);
  const admin = await sessionBearer(page);
  const headers = { Authorization: "Bearer " + admin };
  const settings = await page.request.get(GATEWAY + "/config/list?config_type=general_settings", { headers });
  expect(settings.status()).toBe(200);
  const logging = (await settings.json()).find((field: any) => field.field_name === "store_prompts_in_spend_logs");
  // 正文存储是内部配置，管理页面的字段目录可能不展示；独立 schema 中未声明时按无数据库覆盖清理。
  const jwt = (await page.context().cookies()).find(cookie => cookie.name === "token")!.value;
  const userID = JSON.parse(Buffer.from(jwt.split(".")[1], "base64url").toString()).user_id;
  const created = await page.request.post(GATEWAY + "/key/generate", { headers, data: {
    owner_type: "personal", user_id: userID, key_alias: "codex-agent-e2e", models: ["e2e-codex-agent"],
  }});
  expect(created.status()).toBe(200);
  const key = await created.json();
  const directory = mkdtempSync(path.join(tmpdir(), "xhub-codex-"));
  try {
    // 显式保存正文以验证真实日志；退出时恢复原值或删除本例新增的数据库覆盖。
    const enabled = await page.request.post(GATEWAY + "/config/update", { headers,
      data: { general_settings: { store_prompts_in_spend_logs: true } } });
    expect(enabled.status()).toBe(200);
    const session = JSON.parse(execFileSync("python3", [path.resolve(__dirname, "../../e2e/codex_simulator.py")], {
      input: JSON.stringify({ gateway: GATEWAY, directory, admin, key: { ...key,
        user_id: key.user_id, team_id: key.team_id || "", project_id: key.project_id || "",
        organization_id: key.organization_id || "", profile: "inherit", call_model: "e2e-codex-agent",
      }}), encoding: "utf8", timeout: 90000,
    }));
    expect(session.agent_type).toBe("codex");
    expect(session.turns).toHaveLength(3);
    expect(new Set(session.turns.map((turn: any) => turn.call_id)).size).toBe(3);
    for (const turn of session.turns) {
      const detail = await page.request.get(GATEWAY + "/spend/logs/ui/" + turn.call_id, { headers });
      expect(detail.status()).toBe(200);
      const bill = await detail.json();
      expect(bill.session_id).toBe(session.session_id);
      expect(bill.api_key).toBe(key.token_id);
      expect(bill.prompt_tokens).toBe(8);
      expect(bill.completion_tokens).toBe(2);
      expect(Number(bill.spend)).toBeCloseTo(0.000012, 10);
    }
    const checks = await verifyCodexLogs(page, new URL(page.url()).origin, GATEWAY, admin, [session]);
    expect(checks).toHaveLength(1);
  } finally {
    try {
      const restored = logging?.stored_in_db
        ? await page.request.post(GATEWAY + "/config/update", { headers,
          data: { general_settings: { store_prompts_in_spend_logs: logging.field_value } } })
        : await page.request.post(GATEWAY + "/config/field/delete", { headers,
          data: { config_type: "general_settings", field_name: "store_prompts_in_spend_logs" } });
      expect(restored.status()).toBe(200);
    } finally {
      try {
        const removed = await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [key.token_id] }});
        expect(removed.ok(), "Codex 测试密钥应清理成功").toBeTruthy();
      } finally {
        rmSync(directory, { recursive: true, force: true });
      }
    }
  }
});
}
