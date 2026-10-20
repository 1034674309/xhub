import { expect, test } from "@playwright/test";
import { GATEWAY, loginAdmin, sessionBearer } from "./helpers";
import { translate, type Locale } from "../src/i18n/translate";

/** 注册一个语言的完整浏览器场景；参数locale选择界面语言，测试体管理临时资源，调用本身不请求后台。 */
export function registerClientScenario(locale: Locale) {
  /** 验证两语CLI文档可复制绝对URL并用该地址完成真实Responses/Messages调用和账单回读。
   * 前置生产构建控制台、真实后台与离线上游；同源/gw由portable构建验证，默认构建覆盖公开网关地址。
   * 只创建本例Key，finally删除；私有schema由运行器回收，不依赖外部供应商或真实CLI安装。
   */
  test(`client documentation URLs execute real agent APIs ${locale}`, async ({ page }) => {
    await loginAdmin(page);
    const headers = { Authorization: "Bearer " + (await sessionBearer(page)) };
    let key = "";
    try {
      const issued = await page.request.post(GATEWAY + "/key/generate", {
        headers,
        data: { key_alias: `e2e-cli-docs-${locale}`, models: ["gpt-4o-mini"] },
      });
      expect(issued.status()).toBe(200);
      key = (await issued.json()).key;
      for (const slug of ["codex", "claude-code"]) {
        await page.goto("/docs/tools-agents-clients/" + slug);
        if (locale === "en" && (await page.getByRole("button", { name: "English", exact: true }).isVisible()))
          await page.getByRole("button", { name: "English", exact: true }).click();
        const code = page.getByLabel(translate(locale, "docs.example"), { exact: true });
        // 组件完成hydration后必须替换占位符，不能把服务端静态示例算作测试通过。
        await expect(code).not.toContainText("https://YOUR_GATEWAY_HOST");
        const text = await code.textContent();
        const url = text?.match(slug === "codex" ? /base_url\s*=\s*"([^"]+)"/ : /ANTHROPIC_BASE_URL="([^"]+)"/)?.[1];
        expect(url).toBeTruthy();
        expect(new URL(url!).protocol).toMatch(/^https?:$/);
        if (process.env.E2E_PORTABLE_CONSOLE === "1") expect(url).toBe(new URL(page.url()).origin + "/gw");
        else expect(url).toBe(GATEWAY);
        const path = slug === "codex" ? "/responses" : "/v1/messages";
        const requestBody =
          slug === "codex"
            ? { model: "gpt-4o-mini", input: "Reply with a greeting.", max_output_tokens: 32 }
            : { model: "gpt-4o-mini", messages: [{ role: "user", content: "Reply with a greeting." }], max_tokens: 32 };
        // 通过浏览器所见URL真实访问，portable测试域名由浏览器解析规则路由到本机。
        const result = await page.evaluate(
          async ({ target, key, body }) => {
            // 浏览器直接执行文档地址，不能用内部API客户端替换或纠正待验证URL。
            // eslint-disable-next-line no-restricted-syntax
            const response = await fetch(target, {
              method: "POST",
              headers: {
                Authorization: "Bearer " + key,
                "Content-Type": "application/json",
                "anthropic-version": "2023-06-01",
              },
              body: JSON.stringify(body),
            });
            return {
              status: response.status,
              callID: response.headers.get("x-litellm-call-id"),
              body: await response.text(),
            };
          },
          { target: url! + path, key, body: requestBody },
        );
        expect(result.status, result.body).toBe(200);
        expect(result.body).toContain("e2e-ok");
        expect(result.callID).toBeTruthy();
        await expect
          .poll(async () => {
            const detail = await page.request.get(GATEWAY + "/spend/logs/ui/" + result.callID, { headers });
            return detail.ok() ? (await detail.json()).status : null;
          })
          .toBe("success");
      }
    } finally {
      if (key)
        expect((await page.request.post(GATEWAY + "/key/delete", { headers, data: { keys: [key] } })).status()).toBe(
          200,
        );
    }
  });
}
