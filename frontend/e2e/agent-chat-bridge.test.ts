import { expect, test, type Page } from "@playwright/test";
import { GATEWAY, UPSTREAM, loginAdmin, sessionBearer, stableGoto, t } from "./helpers";

/** 通过真实浏览器同源代理发送 Responses 正文，返回状态、原始响应与账单 ID；参数为页面、登录令牌和正文。E2E 工具续轮调用，不拦截网络、不伪造响应，持久日志由运行器隔离 schema 清理。 */
async function requestAgent(page: Page, bearer: string, body: Record<string, unknown>) {
  return page.evaluate(async ({ bearer, body }) => {
    const response = await fetch("/gw/v1/responses", { method: "POST", headers: { Authorization: "Bearer " + bearer, "Content-Type": "application/json" }, body: JSON.stringify(body) });
    return { status: response.status, text: await response.text(), callID: response.headers.get("x-litellm-call-id")! };
  }, { bearer, body });
}

/** 提取 Responses JSON 或 SSE 的最终对象；参数为原始正文和流开关，返回完整输出项，缺少完成事件时抛错。仅供浏览器回传真实工具历史，不写任何外部数据。 */
function completedAgent(raw: string, stream: boolean) {
  if (!stream) return JSON.parse(raw);
  const line = raw.split("\n").find(line => line.startsWith("data:") && line.includes('"type":"response.completed"'));
  if (!line) throw new Error("Agent stream did not complete");
  return JSON.parse(line.slice(5)).response;
}

/** 验证保存 Chat 上游模型、调试台实际调用、Codex 参数及 namespace/custom 的 JSON/SSE 工具续轮、日志和记账。前置浏览器/真实 Go 后台/隔离数据库及本地 Chat-only 夹具；finally 删除部署凭据，运行器回收历史账单，不使用外部 Key。 */
test("Codex tools round trip over a saved Chat deployment", async ({ page }) => {
  test.setTimeout(120000);
  await loginAdmin(page);
  const bearer = await sessionBearer(page);
  const headers = { Authorization: "Bearer " + bearer };
  const name = "e2e-codex-chat-bridge";
  const credential = "e2e-codex-chat-credential";
  let modelID = "";
  expect((await page.request.post(GATEWAY + "/credentials", { headers, data: {
    credential_name: credential, credential_info: { custom_llm_provider: "openai" }, credential_values: { api_base: UPSTREAM + "/v1", api_key: "sk-fake" },
  } })).status()).toBe(200);
  try {
    const created = await page.request.post(GATEWAY + "/model/new", { headers, data: {
      model_name: name, litellm_params: { model: "e2e-agent-chat-tools", custom_llm_provider: "openai", litellm_credential_name: credential, input_cost_per_token: 0.000001, output_cost_per_token: 0.000002 },
      model_info: { transport: "bypass_openai_chat", endpoint_types: ["responses"], pricing_source: "manual" },
    } });
    expect(created.status(), await created.text()).toBe(200);
    modelID = (await created.json()).model_info.id;
    await page.evaluate(() => sessionStorage.setItem("streamingEnabled", "false"));
    await stableGoto(page, "/playground");
    await page.getByPlaceholder(t("Select a Model"), { exact: true }).click();
    await page.getByRole("option", { name, exact: true }).click();
    await page.getByPlaceholder(t("Select an endpoint"), { exact: true }).click();
    await page.getByRole("option").filter({ hasText: /\/v1\/responses$/ }).click();
    await page.getByPlaceholder(t("Type your message... (Shift+Enter for new line)")).filter({ visible: true }).fill("Reply with a short greeting.");
    const pending = page.waitForResponse(r => /\/responses$/.test(new URL(r.url()).pathname) && r.request().method() === "POST");
    await page.getByRole("button", { name: t("Send message") }).click();
    const uiReply = await pending;
    expect(uiReply.status()).toBe(200);
    await expect(page.getByText("e2e-ok", { exact: true }).last()).toBeVisible();
    const callIDs = [uiReply.headers()["x-litellm-call-id"]];
    for (const custom of [false, true]) {
      for (const stream of [false, true]) {
        const tool = custom ? { type: "custom", name: "read", description: "Read the requested file." } : { type: "function", name: "read", parameters: { type: "object", properties: {} } };
        const options = { model: name, stream, reasoning: { effort: "medium", summary: "auto", context: "all_turns" }, include: ["reasoning.encrypted_content"], client_metadata: { session_id: "client-only" } };
        const tools = [{ type: "namespace", name: "fs", tools: [tool] }];
        const declaration = custom ? [{ type: "additional_tools", tools }] : [];
        const toolOptions = custom ? {} : { tools };
        const first = await requestAgent(page, bearer, { ...options, ...toolOptions, input: [...declaration, { role: "user", content: "Read the file." }] });
        expect(first.status, first.text).toBe(200);
        const completed = completedAgent(first.text, stream);
        const call = completed.output.find((item: { type: string }) => item.type === (custom ? "custom_tool_call" : "function_call"));
        expect(call).toMatchObject({ name: "read", namespace: "fs", call_id: "call_1" });
        expect(custom ? call.input : call.arguments).toBe(custom ? "Read the file." : "{}");
        if (custom && stream) expect(first.text).toContain("response.custom_tool_call_input.delta");
        const second = await requestAgent(page, bearer, { ...options, ...toolOptions, input: [...declaration, { role: "user", content: "Read the file." }, ...completed.output, { type: custom ? "custom_tool_call_output" : "function_call_output", call_id: call.call_id, output: "file-value" }] });
        expect(second.status, second.text).toBe(200);
        expect(completedAgent(second.text, stream).output).toEqual(expect.arrayContaining([expect.objectContaining({ type: "message", content: [expect.objectContaining({ type: "output_text", text: "e2e-ok" })] })]));
        callIDs.push(first.callID, second.callID);
      }
    }
    for (const callID of callIDs) {
      expect(callID).toBeTruthy();
      await expect.poll(async () => {
        const response = await page.request.get(GATEWAY + "/spend/logs/ui/" + callID, { headers });
        if (!response.ok()) return null;
        const bill = await response.json();
        return { model: bill.model, status: bill.status, prompt: bill.prompt_tokens, completion: bill.completion_tokens, spend: Number(bill.spend) };
      }).toEqual({ model: name, status: "success", prompt: 8, completion: 2, spend: 0.000012 });
    }
    const rejected = await requestAgent(page, bearer, { model: name, input: "Hello", include: ["unknown_native_field"] });
    expect(rejected.status).toBe(400);
    const unsupportedTool = await requestAgent(page, bearer, { model: name, input: "Hello", tools: [{ type: "custom", name: "run", format: { type: "unknown_native_format" } }] });
    expect(unsupportedTool.status).toBe(400);
    await stableGoto(page, "/logs");
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
  } finally {
    if (modelID) expect((await page.request.post(GATEWAY + "/model/delete", { headers, data: { id: modelID } })).status()).toBe(200);
    expect((await page.request.delete(GATEWAY + "/credentials/" + credential, { headers })).status()).toBe(200);
  }
});
