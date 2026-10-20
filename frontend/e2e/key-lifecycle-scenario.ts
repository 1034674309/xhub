import { expect, test } from "@playwright/test";
import { translate } from "../src/i18n/translate";
import { GATEWAY, loginAdmin, sessionBearer, stableGoto } from "./helpers";

/** 注册 Key 生命周期双语闭环；locale 为界面语言，无返回值。
 * 前置私有数据库、真实网关与本地模型上游；浏览器创建、编辑到期、轮换恢复、回读限额、调用模型和删除。
 * 不支持的管理类型、自动轮换、宽限期必须可见且真实 API 拒绝；finally 删除仅本场景创建的 Key，运行器回收 schema。 */
export function registerKeyLifecycleScenario(locale: "en" | "zh-CN") {
  test("key lifecycle settings and atomic rotation " + locale, async ({ page }) => {
    test.setTimeout(120_000);
    await loginAdmin(page);
    const headers = { Authorization: "Bearer " + (await sessionBearer(page)) };
    await page
      .getByRole("button", { name: locale === "en" ? "English" : translate("en", "language.zhCN"), exact: true })
      .click();
    const tr = (key: string) => translate(locale, key);
    const alias = "lifecycle-original-用户-" + locale + "-" + Date.now();
    let id = "";
    try {
      await page.getByTestId("create-key-button").click();
      await page.getByLabel(tr("Key Name")).fill(alias);
      await expect(page.getByText(tr("keyEditor.rotationUnsupported"))).toBeVisible();
      await expect(page.getByRole("switch")).toHaveCount(0);
      await page.getByRole("combobox", { name: tr("Key Type"), exact: true }).click();
      await expect(page.getByRole("option", { name: /Management|管理/ })).toHaveCount(0);
      await page.getByRole("option", { name: tr("AI APIs") }).click();
      await page.getByRole("textbox", { name: tr("keyEditor.expire"), exact: true }).fill("30d");
      const mintedPromise = page.waitForResponse(
        (r) => r.url().includes("/key/generate") && r.request().method() === "POST",
      );
      await page.getByRole("button", { name: tr("pages.apiKeys.createSubmit"), exact: true }).click();
      const minted = await mintedPromise;
      expect(minted.status()).toBe(200);
      const original = await minted.json();
      id = original.token_id;
      expect(original).toMatchObject({ key_type: "llm_api", auto_rotate: false, rotation_interval: null });
      await page.keyboard.press("Escape");
      const invalidRotation = await page.request.post(GATEWAY + "/key/" + id + "/regenerate", {
        headers: { ...headers, "Content-Type": "application/json" },
        data: '{"duration":',
      });
      expect(invalidRotation.status()).toBe(400);
      expect(
        (
          await page.request.get(GATEWAY + "/v1/models", { headers: { Authorization: "Bearer " + original.key } })
        ).status(),
      ).toBe(200);
      const body = { model: "gpt-4o-mini", messages: [{ role: "user", content: "Verify key lifecycle access." }] };
      const first = await page.request.post(GATEWAY + "/v1/chat/completions", {
        headers: { Authorization: "Bearer " + original.key },
        data: body,
      });
      expect(first.status(), await first.text()).toBe(200);
      expect((await first.json()).choices[0].message.content).toMatch(/ok/);
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(page.getByLabel(tr("Key Alias"))).toHaveValue(alias);
      await expect(page.getByText(tr("keyEditor.apiOnly"))).toBeVisible();
      await page.getByRole("textbox", { name: tr("keyEditor.expire"), exact: true }).fill("1s");
      const updatedPromise = page.waitForResponse(
        (r) => r.url().includes("/key/update") && r.request().method() === "POST",
      );
      await page.getByRole("button", { name: tr("Save Changes"), exact: true }).click();
      expect((await updatedPromise).status()).toBe(200);
      await expect
        .poll(async () =>
          (
            await page.request.get(GATEWAY + "/v1/models", { headers: { Authorization: "Bearer " + original.key } })
          ).status(),
        )
        .toBe(401);
      await stableGoto(page, "/api-keys?key=" + id);
      await page.getByRole("button", { name: tr("More key actions"), exact: true }).click();
      await page.getByRole("menuitem", { name: tr("Regenerate Key"), exact: true }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog.getByText(tr("keyEditor.immediateRotation"))).toBeVisible();
      await expect(dialog.getByLabel(tr("Grace Period"))).toHaveCount(0);
      await dialog.getByRole("button", { name: tr("Regenerate"), exact: true }).click();
      await expect(dialog.getByText(tr("keyEditor.expiredDurationRequired"))).toBeVisible();
      await dialog.getByLabel(tr("Expire Key")).fill("1h");
      await dialog.getByLabel(tr("Max Budget (USD)")).fill("1");
      await dialog.getByLabel(tr("RPM Limit")).fill("10");
      await dialog.getByLabel(tr("TPM Limit")).fill("10000");
      const rotatedPromise = page.waitForResponse(
        (r) => r.url().includes("/regenerate") && r.request().method() === "POST",
      );
      await dialog.getByRole("button", { name: tr("Regenerate"), exact: true }).click();
      const rotatedResponse = await rotatedPromise;
      expect(rotatedResponse.status(), await rotatedResponse.text()).toBe(200);
      const rotated = await rotatedResponse.json();
      expect(rotated).toMatchObject({ key_alias: alias, max_budget: 1, rpm_limit: 10, tpm_limit: 10000 });
      expect(Date.parse(rotated.expires)).toBeGreaterThan(Date.now() + 3_500_000);
      await dialog
        .getByRole("button", { name: tr("Close"), exact: true })
        .last()
        .click();
      await stableGoto(page, "/api-keys?key=" + id);
      await expect(page.getByLabel(tr("Max Budget (USD)"))).toHaveValue("1");
      await expect(page.getByLabel(tr("Key Alias"))).toHaveValue(alias);
      const snapshot = await (await page.request.get(GATEWAY + "/key/info?key=" + id, { headers })).json();
      expect(snapshot.info).toMatchObject({
        max_budget: 1,
        rpm_limit: 10,
        tpm_limit: 10000,
        key_type: "llm_api",
        auto_rotate: false,
      });
      for (const unsupported of [
        { key_type: "management" },
        { auto_rotate: true, rotation_interval: "30d" },
        { grace_period: "60s" },
      ]) {
        const rejected = await page.request.post(GATEWAY + "/key/" + id + "/regenerate", {
          headers,
          data: unsupported,
        });
        expect(rejected.status(), await rejected.text()).toBe(400);
      }
      expect(
        (
          await page.request.get(GATEWAY + "/v1/models", { headers: { Authorization: "Bearer " + original.key } })
        ).status(),
      ).toBe(401);
      const inferred = await page.request.post(GATEWAY + "/v1/chat/completions", {
        headers: { Authorization: "Bearer " + rotated.key },
        data: body,
      });
      expect(inferred.status(), await inferred.text()).toBe(200);
      expect((await inferred.json()).choices[0].message.content).toMatch(/ok/);
      await page.getByRole("button", { name: tr("More key actions"), exact: true }).click();
      await page.getByRole("menuitem", { name: tr("Delete Key"), exact: true }).click();
      await page.getByRole("dialog").getByPlaceholder(alias, { exact: true }).fill(alias);
      await page
        .getByRole("dialog")
        .getByRole("button", { name: tr("common.delete"), exact: true })
        .click();
      await expect(page.getByTestId("create-key-button")).toBeVisible();
      expect(
        (
          await page.request.get(GATEWAY + "/v1/models", { headers: { Authorization: "Bearer " + rotated.key } })
        ).status(),
      ).toBe(401);
      id = "";
    } finally {
      if (id)
        expect((await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [id] } })).status()).toBe(
          200,
        );
    }
  });
}
