import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import { GATEWAY, UPSTREAM, loginAdmin, sessionBearer, stableGoto } from "./helpers";

for (const failure of [false, true]) {
  /** 前置真实浏览器、网关、隔离数据库与本地上游；创建个人密钥并发送成功/未知模型失败请求。
   * 参数 page 为浏览器；核对响应、账单与密钥累计费用一致，失败不扣费，刷新和删除凭证不改变历史账单。
   * 验证日志保留 Hash、现存及已删除密钥均无可点击详情入口，点击行仍能打开日志。
   * 成功分支创建显式计价的专用模型，不依赖默认模型的目录价格；finally 撤销新密钥并删除专用模型，
   * 运行器删除独立 schema 中的日志与辅助组织；失败抛出，不使用外部供应商凭据。 */
  test("日志密钥 Hash 仅展示且删除密钥后仍可查看日志：" + (failure ? "错误日志" : "普通日志"), async ({ page }) => {
    await loginAdmin(page);
    const headers = { Authorization: "Bearer " + await sessionBearer(page) };
    const generated = await page.request.post(GATEWAY + "/key/generate", {
      headers, data: { key_alias: "e2e-log-hash-" + Date.now() },
    });
    expect(generated.status(), await generated.text()).toBe(200);
    const key = await generated.json();
    const model = "e2e-log-hash-" + key.token_id;
    const deploymentID = "log-hash-dep-" + key.token_id;
    let modelCreated = false;
    let deleted = false;
    try {
      if (!failure) {
        const created = await page.request.post(GATEWAY + "/model/new", { headers, data: {
          model_name: model,
          litellm_params: { model: "log-hash-upstream", api_base: UPSTREAM, api_key: "sk-fake", deployment_id: deploymentID, custom_llm_provider: "custom", input_cost_per_token: 0.000001, output_cost_per_token: 0.000002 },
          model_info: { id: deploymentID, transport: "bypass_openai_chat", endpoint_types: ["chat"], pricing_source: "manual" },
        } });
        expect(created.status(), await created.text()).toBe(200);
        modelCreated = true;
      }
      const initial = await page.request.get(GATEWAY + "/key/info?key=" + key.token_id, { headers });
      expect(initial.status(), await initial.text()).toBe(200);
      const beforeSpend = Number((await initial.json()).info.spend);
      let usage: { prompt_tokens: number; completion_tokens: number } | undefined;
      const inference = await page.request.post(GATEWAY + "/v1/chat/completions", {
        headers: { Authorization: "Bearer " + key.key },
        data: { model: failure ? "e2e-log-hash-missing-model" : model, messages: [{ role: "user", content: "log hash regression" }] },
      });
      if (failure) {
        expect(inference.status()).toBeGreaterThanOrEqual(400);
      } else {
        expect(inference.status(), await inference.text()).toBe(200);
        const body = await inference.json();
        expect(body.choices[0].message.content).toContain("ok");
        usage = body.usage;
      }
      const responseCost = Number(inference.headers()["x-litellm-response-cost"] ?? 0);
      if (failure) expect(responseCost, "失败请求不得扣费").toBe(0);
      else {
        expect(responseCost, "本地计价上游成功请求应产生费用").toBeGreaterThan(0);
        expect(responseCost, "响应费用必须按专用模型价格和实际用量计算").toBeCloseTo(usage!.prompt_tokens * 0.000001 + usage!.completion_tokens * 0.000002, 10);
      }
      const id = inference.headers()["x-litellm-call-id"];
      expect(id).toBeTruthy();
      await expect.poll(async () => (await page.request.get(GATEWAY + "/spend/logs/ui/" + id, { headers })).status()).toBe(200);
      await expect.poll(async () => {
        const info = await page.request.get(GATEWAY + "/key/info?key=" + key.token_id, { headers });
        expect(info.status()).toBe(200);
        return Number((await info.json()).info.spend) - beforeSpend;
      }, { message: "密钥累计费用必须与本次响应金额一致" }).toBeCloseTo(responseCost, 10);
      const hash = createHash("sha256").update(key.key).digest("hex");
      const keyInfoRequests: string[] = [];
      page.on("request", (request) => {
        // 只观察浏览器发出的请求；测试的 APIRequestContext 用于验证真实契约，不会进入此事件。
        if (/\/(?:v2\/)?key\/info(?:\?|$)/.test(request.url())) keyInfoRequests.push(request.url());
      });

      for (const revoke of [false, true]) {
        if (revoke) {
          const removed = await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [key.token_id] } });
          expect(removed.status(), await removed.text()).toBe(200);
          deleted = true;
          const missing = await page.request.get(GATEWAY + "/key/info?key=" + key.token_id, { headers });
          expect(missing.status(), "删除后的密钥详情必须返回 404").toBe(404);
        }
        const stored = await page.request.get(GATEWAY + "/spend/logs/ui/" + id, { headers });
        expect(stored.status()).toBe(200);
        const bill = await stored.json();
        expect(bill.metadata.user_api_key, "历史日志必须保留原密钥 Hash").toBe(hash);
        expect(bill.api_key, "账单必须保留原凭证身份").toBe(key.token_id);
        expect(Number(bill.spend), "刷新或删除凭证不得重新定价或重复扣费").toBeCloseTo(responseCost, 10);
        if (usage) {
          expect(bill.prompt_tokens, "日志输入用量必须等于响应").toBe(usage.prompt_tokens);
          expect(bill.completion_tokens, "日志输出用量必须等于响应").toBe(usage.completion_tokens);
        }
        await stableGoto(page, "/logs");
        if (failure) await page.getByRole("tab", { name: /错误日志|Error Logs/, exact: true }).click();
        await page.getByPlaceholder(/按 ID 搜索日志|Search logs by ID/).fill(id);
        const row = page.getByRole("row").filter({ hasText: id });
        await expect(row).toHaveCount(1);
        const cell = row.getByRole("cell", { name: hash, exact: true });
        await expect(cell).toBeVisible();
        await expect(cell.getByRole("button")).toHaveCount(0);
        await expect(cell.getByRole("link")).toHaveCount(0);
        await cell.getByText(hash, { exact: true }).click();
        await expect(page.getByRole("dialog")).toBeVisible();
        await expect(page).toHaveURL(new RegExp("log_id=" + id));
        expect(keyInfoRequests, "日志页不得请求密钥详情").toEqual([]);
        await expect(page.getByText(/获取密钥信息失败|Failed to fetch key info/)).toHaveCount(0);
      }
    } finally {
      try {
        if (!deleted) {
          const cleanup = await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [key.token_id] } });
          expect(cleanup.status(), await cleanup.text()).toBe(200);
        }
      } finally {
        if (modelCreated) {
          const cleanup = await page.request.post(GATEWAY + "/model/delete", { headers, data: { id: deploymentID } });
          expect(cleanup.status(), await cleanup.text()).toBe(200);
        }
      }
    }
  });
}
