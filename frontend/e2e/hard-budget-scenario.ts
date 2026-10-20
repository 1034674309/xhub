import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { translate } from "../src/i18n/translate";
import { GATEWAY, UPSTREAM, loginAdmin, sessionBearer, stableGoto } from "./helpers";

/** 注册双语硬额度闭环；前置隔离无 Redis 网关、生产构建和本地模型，UI 创建/编辑/删除 Key。
 * API 实际调用验证默认 cap、余额不足、PG pending 保留和恢复，页面显示可核对占用；finally 恢复故障并删除专属资源。 */
export function registerHardBudgetScenario(locale: "en" | "zh-CN") {
  test("hard budget reserves and recovers through key management " + locale, async ({ page }) => {
    test.setTimeout(120_000);
    await loginAdmin(page);
    const headers = { Authorization: "Bearer " + (await sessionBearer(page)) };
    await page
      .getByRole("button", { name: locale === "en" ? "English" : translate("en", "language.zhCN"), exact: true })
      .click();
    const tr = (key: string) => translate(locale, key);
    const control = JSON.parse(readFileSync(join(process.env.E2E_RUN_DIR!, "enterprise-fixture.json"), "utf8")).control;
    const model = "pending-browser-hard-budget-" + locale.toLowerCase() + "-" + Date.now(),
      alias = "预算-budget-" + locale;
    let id = "",
      modelID = "",
      failed = false;
    try {
      const createdModel = await page.request.post(GATEWAY + "/model/new", {
        headers,
        data: {
          model_name: model,
          litellm_params: {
            model,
            custom_llm_provider: "openai",
            api_base: UPSTREAM,
            api_key: "sk-fake",
            input_cost_per_token: 0.00001,
            output_cost_per_token: 0.00002,
          },
          model_info: {
            transport: "bypass_openai_chat",
            pricing_source: "manual",
            endpoint_types: ["chat"],
            max_input_tokens: 1024,
            max_output_tokens: 8192,
            supported_parameters: ["max_tokens"],
          },
        },
      });
      expect(createdModel.status(), await createdModel.text()).toBe(200);
      modelID = (await createdModel.json()).model_info.id;
      await page.getByTestId("create-key-button").click();
      await page.getByLabel(tr("Key Name")).fill(alias);
      await page.getByLabel(tr("Max Budget (USD)"), { exact: true }).fill("0.01");
      const minted = page.waitForResponse((r) => r.url().includes("/key/generate") && r.request().method() === "POST");
      await page.getByRole("button", { name: tr("pages.apiKeys.createSubmit"), exact: true }).click();
      const key = await (await minted).json();
      id = key.token_id;
      await page.keyboard.press("Escape");
      let requestSequence = 0;
      const invoke = () =>
        page.request.post(GATEWAY + "/v1/chat/completions", {
          headers: { Authorization: "Bearer " + key.key },
          data: {
            model,
            messages: [{ role: "user", content: "Verify strict budget recovery request " + ++requestSequence + "." }],
          },
        });
      const refused = await invoke();
      expect(refused.status()).toBe(429);
      expect((await refused.json()).error.type).toBe("budget_reservation_exceeded");
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(page.getByText(translate(locale, "hardBudget.contract", { cap: 4096 }))).toBeVisible();
      await page.getByLabel(tr("Max Budget (USD)"), { exact: true }).fill("0.1");
      const updated = page.waitForResponse((r) => r.url().includes("/key/update") && r.request().method() === "POST");
      await page.getByRole("button", { name: tr("Save Changes"), exact: true }).click();
      expect((await updated).status()).toBe(200);
      const injected = await page.request.post(control + "/billing-pending/fail", { data: { model } });
      expect(injected.status(), await injected.text()).toBe(200);
      failed = true;
      const paid = await invoke();
      expect(paid.status(), await paid.text()).toBe(200);
      expect(paid.headers()["x-xhub-max-output-tokens"]).toBe("4096");
      expect(paid.headers()["x-xhub-budget-input-bound"]).toBe("1024");
      const pendingInfo = await (await page.request.get(GATEWAY + "/key/info?key=" + id, { headers })).json();
      expect(pendingInfo.info.budget_pending).toBeCloseTo(0.09216, 9);
      expect(pendingInfo.info.budget_reservations).toHaveLength(1);
      expect(pendingInfo.info.budget_reservations[0].state).toBe("settling");
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(
        page.getByText(pendingInfo.info.budget_reservations[0].reservation_id, { exact: true }),
      ).toBeVisible();
      await expect(page.getByText(tr("hardBudget.reconcile"))).toBeVisible();
      // 仅中断本测试浏览器的详情读取，验证列表缓存不会把未知占用显示为零；恢复真实接口后点击重试回读原预占。
      const detailRoute = /\/key\/info(?:\?|$)/;
      await page.route(detailRoute, (route) =>
        route.fulfill({
          status: 503,
          contentType: "application/json",
          body: JSON.stringify({
            error: { type: "quota_unavailable", message: "Budget storage temporarily unavailable" },
          }),
        }),
      );
      await page.reload();
      await expect(page.getByRole("alert").filter({ hasText: tr("hardBudget.detailsLoadFailed") })).toBeVisible({
        timeout: 20000,
      });
      await expect(page.getByText(translate(locale, "hardBudget.pending", { amount: 0 }))).toHaveCount(0);
      await expect(page.getByRole("button", { name: tr("Save Changes"), exact: true })).toHaveCount(0);
      await page.unroute(detailRoute);
      await page.getByRole("button", { name: tr("hardBudget.retry"), exact: true }).click();
      await expect(
        page.getByText(pendingInfo.info.budget_reservations[0].reservation_id, { exact: true }),
      ).toBeVisible();
      expect((await invoke()).status()).toBe(429);
      const recovered = await page.request.post(control + "/billing-pending/recover", { data: { model } });
      expect(recovered.status(), await recovered.text()).toBe(200);
      failed = false;
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(page.getByText(translate(locale, "hardBudget.pending", { amount: 0 }))).toBeVisible();
      expect((await invoke()).status()).toBe(200);
      // 同一计量的跨帧累计允许更新；只有同帧别名矛盾才保留待核对占用。
      const growing = await page.request.post(GATEWAY + "/v1/chat/completions", {
        headers: { Authorization: "Bearer " + key.key },
        data: { model, stream: true, messages: [{ role: "user", content: "Verify cumulative usage." }] },
      });
      expect(growing.status()).toBe(200);
      expect(await growing.text()).toContain("e2e-budget-growth");
      await expect
        .poll(async () => {
          const info = await (await page.request.get(GATEWAY + "/key/info?key=" + id, { headers })).json();
          return info.info.budget_pending;
        })
        .toBe(0);
      const growthLog = await page.request.get(GATEWAY + "/spend/logs/ui/" + growing.headers()["x-litellm-call-id"], {
        headers,
      });
      expect(growthLog.status()).toBe(200);
      expect((await growthLog.json()).spend).toBeCloseTo(0.00012, 9);
      const uncertain = await page.request.post(GATEWAY + "/v1/chat/completions", {
        headers: { Authorization: "Bearer " + key.key },
        data: { model, stream: true, messages: [{ role: "user", content: "Return contradictory usage." }] },
      });
      expect(uncertain.status()).toBe(200);
      expect(await uncertain.text()).toContain("e2e-budget-conflict");
      await expect
        .poll(async () => {
          const info = await (await page.request.get(GATEWAY + "/key/info?key=" + id, { headers })).json();
          return info.info.budget_reservations?.[0]?.state;
        })
        .toBe("uncertain");
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(page.getByText(tr("hardBudget.reconcile"))).toBeVisible();
      await expect(page.getByRole("listitem").filter({ hasText: tr("hardBudget.states.uncertain") })).toBeVisible();
      expect((await invoke()).status()).toBe(429);
      await page.getByRole("button", { name: tr("More key actions"), exact: true }).click();
      await page.getByRole("menuitem", { name: tr("Delete Key"), exact: true }).click();
      await page.getByRole("dialog").getByPlaceholder(alias, { exact: true }).fill(alias);
      await page
        .getByRole("dialog")
        .getByRole("button", { name: tr("common.delete"), exact: true })
        .click();
      await expect(page.getByTestId("create-key-button")).toBeVisible();
      expect((await invoke()).status()).toBe(401);
      id = "";
    } finally {
      if (failed)
        expect((await page.request.post(control + "/billing-pending/recover", { data: { model } })).status()).toBe(200);
      if (id)
        expect((await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [id] } })).status()).toBe(
          200,
        );
      if (modelID)
        expect((await page.request.post(GATEWAY + "/model/delete", { headers, data: { id: modelID } })).status()).toBe(
          200,
        );
    }
  });
}
