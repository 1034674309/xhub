import { expect, test, type Page } from "@playwright/test";
import { GATEWAY, UPSTREAM, loginAdmin, sessionBearer, stableGoto } from "./helpers";
import { translate } from "../src/i18n/translate";

const PNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNg+M8AAAICAQB7CYF4AAAAAElFTkSuQmCC";

/** 浏览器同源发起真实 API 请求；参数为页面、会话、路径和正文，返回状态/正文/账单 ID；读取完正文再返回，不模拟网络。 */
async function browserPost(page: Page, bearer: string, path: string, body: Record<string, unknown>) {
  return page.evaluate(
    async ({ bearer, path, body }) => {
      const reply = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: "Bearer " + bearer },
        body: JSON.stringify(body),
      });
      return { status: reply.status, raw: await reply.text(), callID: reply.headers.get("x-litellm-call-id")! };
    },
    { bearer, path, body },
  );
}

for (const locale of ["zh-CN", "en"] as const)
  for (const stream of [false, true]) {
    /** 前置真实浏览器/Go 后台/PostgreSQL 和校验图片字节的本地上游；中英文上传、预览、三协议调用、JSON/SSE、日志计费与错误边界。
     * 每个模型都验证原生及桥接上传；提示不包含答案，finally 删除模型/连接，运行器回收身份、账单和 schema，不依赖付费凭据。 */
    test("Vision upload and protocol bridge " + locale + " " + stream, async ({ page }) => {
      test.setTimeout(120000);
      await loginAdmin(page);
      const t = (key: string) => translate(locale, key);
      if (locale === "en")
        await page.getByRole("button", { name: translate("zh-CN", "language.en"), exact: true }).click();
      const bearer = await sessionBearer(page),
        headers = { Authorization: "Bearer " + bearer };
      const credential = "vision-credential-" + stream,
        names = ["vision-chat-", "vision-responses-"].map((name) => name + stream);
      const ids: string[] = [],
        calls: string[] = [];
      expect(
        (
          await page.request.post(GATEWAY + "/credentials", {
            headers,
            data: {
              credential_name: credential,
              credential_info: { custom_llm_provider: "openai" },
              credential_values: { api_base: UPSTREAM + "/v1", api_key: "sk-fake" },
            },
          })
        ).status(),
      ).toBe(200);
      try {
        for (let index = 0; index < 2; index++) {
          const saved = await page.request.post(GATEWAY + "/model/new", {
            headers,
            data: {
              model_name: names[index],
              litellm_params: {
                model: "e2e-vision-" + index,
                custom_llm_provider: "openai",
                litellm_credential_name: credential,
                input_cost_per_token: 0.000001,
                output_cost_per_token: 0.000002,
              },
              model_info: {
                transport: index === 0 ? "bypass_openai_chat" : "bypass_openai_responses",
                endpoint_types: ["chat", "responses", "messages"],
                pricing_source: "manual",
              },
            },
          });
          expect(saved.status(), await saved.text()).toBe(200);
          ids.push((await saved.json()).model_info.id);
        }
        await page.evaluate((stream) => sessionStorage.setItem("streamingEnabled", JSON.stringify(stream)), stream);
        await stableGoto(page, "/playground");
        for (const name of names)
          for (const path of ["/v1/responses", "/v1/chat/completions"]) {
            const clear = page.getByRole("button", { name: t("Clear Chat"), exact: true });
            if (await clear.isVisible()) await clear.click();
            await page.getByPlaceholder(t("Select a Model"), { exact: true }).click();
            await page.getByRole("option", { name, exact: true }).click();
            await page.getByPlaceholder(t("Select an endpoint"), { exact: true }).click();
            await page
              .getByRole("option")
              .filter({ hasText: new RegExp(path.replaceAll("/", "\\/") + "$") })
              .click();
            if (path.endsWith("responses")) {
              const session = page.getByRole("switch", { name: t("Use API session management") });
              if (await session.isChecked()) await session.click();
            }
            const chooser = page.waitForEvent("filechooser");
            await page.getByRole("button", { name: t("Attach image or PDF"), exact: true }).click();
            await (
              await chooser
            ).setFiles({ name: "vision.png", mimeType: "image/png", buffer: Buffer.from(PNG, "base64") });
            await expect(page.getByAltText(t("Upload preview"))).toBeVisible();
            await page
              .getByPlaceholder(t("Type your message... (Shift+Enter for new line)"), { exact: true })
              .fill("Describe the pixel color in this image.");
            const pending = page.waitForResponse(
              (r) => new URL(r.url()).pathname.endsWith(path.replace(/^\/v1/, "")) && r.request().method() === "POST",
            );
            await page.getByRole("button", { name: t("Send message"), exact: true }).click();
            const reply = await pending;
            expect(reply.status()).toBe(200);
            await expect(page.getByText("green-pixel", { exact: true }).last()).toBeVisible();
            calls.push(reply.headers()["x-litellm-call-id"]);
          }
        // Messages 通过同源浏览器代理到真实后台，必须转换成上游能消费的图片，不能只允许一个白名单字段。
        const result = await browserPost(page, bearer, "/gw/v1/messages", {
          model: names[0],
          max_tokens: 256,
          stream,
          messages: [
            {
              role: "user",
              content: [
                { type: "text", text: "Describe this image." },
                { type: "image", source: { type: "base64", media_type: "image/png", data: PNG } },
              ],
            },
          ],
        });
        expect(result.status, result.raw).toBe(200);
        expect(result.raw).toContain("green-pixel");
        calls.push(result.callID);
        const invalid = await browserPost(page, bearer, "/gw/v1/responses", {
          model: names[0],
          input: [{ role: "user", content: [{ type: "input_image", image_url: "file:///private/image.png" }] }],
        });
        expect(invalid.status).toBe(400);
        for (const id of calls)
          await expect
            .poll(async () => {
              const reply = await page.request.get(GATEWAY + "/spend/logs/ui/" + id, { headers });
              if (!reply.ok()) return null;
              const bill = await reply.json();
              return { status: bill.status, spend: Number(bill.spend) };
            })
            .toEqual({ status: "success", spend: 0.000012 });
        await stableGoto(page, "/logs");
        await expect(page.getByText(names[0], { exact: true }).first()).toBeVisible();
      } finally {
        for (const id of ids)
          expect((await page.request.post(GATEWAY + "/model/delete", { headers, data: { id } })).status()).toBe(200);
        expect((await page.request.delete(GATEWAY + "/credentials/" + credential, { headers })).status()).toBe(200);
      }
    });
  }
