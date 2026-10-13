import { expect, test } from "@playwright/test";
import { GATEWAY, UPSTREAM, loginAdmin, sessionBearer, stableGoto } from "../../helpers";

/** 验证备注从真实请求经数据库到日志抽屉且支持中英文；前置测试网关和本地供应商。
 * 临时模型和密钥只属本用例，finally 删除，运行器清理数据库日志；不替代真实供应商 Affinity 验收。
 */
test("日志备注与请求和报告标记一致", async ({ page }) => {
  await loginAdmin(page);
  const headers = { Authorization: "Bearer " + await sessionBearer(page) };
  const name = "log-label-" + Date.now();
  const label = "make e2e Affinity | 原生入口独立偏好 | " + name;
  let id = "", keyID = "";
  try {
    const model = await page.request.post(GATEWAY + "/model/new", { headers, data: {
      model_name: name, litellm_params: { model: "custom/label", api_base: UPSTREAM, api_key: "sk-fake", custom_llm_provider: "custom", input_cost_per_token: 0.000001, output_cost_per_token: 0.000002 },
      model_info: { transport: "bypass_openai_chat", endpoint_types: ["chat"], pricing_source: "manual" },
    } });
    expect(model.status(), await model.text()).toBe(200);
    id = (await model.json()).model_info.id;
    const key = await page.request.post(GATEWAY + "/key/generate", { headers, data: { key_alias: name, models: [name] } });
    expect(key.status()).toBe(200);
    const created = await key.json(); keyID = created.token_id;
    const chat = await page.request.post(GATEWAY + "/v1/chat/completions", { headers: {
      Authorization: "Bearer " + created.key, "X-Xhub-Label": encodeURIComponent(label),
    }, data: { model: name, session_id: name, messages: [{ role: "user", content: "Verify log note " + name }] } });
    expect(chat.status(), await chat.text()).toBe(200);
    const callID = chat.headers()["x-litellm-call-id"];
    await expect.poll(async () => {
      const log = await page.request.get(GATEWAY + "/spend/logs/ui/" + callID, { headers });
      return log.status() === 200 ? (await log.json()).label : "";
    }).toBe(label);
    await stableGoto(page, "/logs?session_id=" + name + "&log_id=" + callID);
    await expect(page.getByRole("region", { name: "备注", exact: true })).toContainText(label);
    // 抽屉打开时背景语言按钮不可交互；先关闭，再切换并按同一请求重新打开。
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "English", exact: true }).click();
    await stableGoto(page, "/logs?session_id=" + name + "&log_id=" + callID);
    await expect(page.getByRole("region", { name: "Label", exact: true })).toContainText(label);
  } finally {
    if (keyID) expect((await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [keyID] } })).status()).toBe(200);
    if (id) expect((await page.request.post(GATEWAY + "/model/delete", { headers, data: { id } })).status()).toBe(200);
  }
});
