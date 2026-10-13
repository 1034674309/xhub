const { test, expect } = require("../../frontend/node_modules/@playwright/test");

/** 用途：取得必填部署参数；参数为环境变量名，返回非空字符串。
 * 调用场景为真实部署验收；缺失即失败，避免误连默认环境，无副作用或敏感值输出。
 */
function required(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing deployment setting: ${name}`);
  return value;
}

const languages = [
  { locale: "zh-CN", title: "登录", username: "请输入用户名", password: "请输入密码", submit: "登录" },
  { locale: "en", title: "Login", username: "Enter your username", password: "Enter your password", submit: "Login" },
];

for (const language of languages) {
  /** 前置真实 Docker 服务及独立管理员凭据；验证中英文切换、输入保留、登录错误、
   * 成功登录、管理接口、数据面鉴权及退出失效。只创建测试会话，finally 注销并关闭浏览器上下文。
   */
  test(`部署验收 ${language.locale}：浏览器登录到真实后台并验证 API 鉴权`, async ({ page }) => {
    const consoleURL = required("XHUB_DEPLOY_CONSOLE_URL");
    const gatewayURL = required("XHUB_DEPLOY_GATEWAY_URL");
    const email = required("XHUB_DEPLOY_ADMIN_EMAIL");
    const password = required("XHUB_DEPLOY_ADMIN_PASSWORD");
    const failures = [];
    page.on("pageerror", error => failures.push(error.message));
    let token;
    try {
      const response = await page.goto(`${consoleURL}/login`);
      expect(response.status(), "控制台登录页必须成功响应").toBe(200);
      await expect(page.getByRole("heading", { name: "登录", exact: true })).toBeVisible();
      // 现有登录页在语言变化时重挂载；部署验收先选择语言，再验证真实凭据流程。
      await page.getByRole("button", { name: "English", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Login", exact: true })).toBeVisible();
      if (language.locale === "zh-CN") {
        await page.getByRole("button", { name: "中文", exact: true }).click();
      }
      await expect(page.getByRole("heading", { name: language.title, exact: true })).toBeVisible();
      await page.getByPlaceholder(language.username).fill(email);
      await page.getByPlaceholder(language.password).fill("deployment-wrong-password");
      const rejected = page.waitForResponse(res => /\/login$/.test(new URL(res.url()).pathname) && res.request().method() === "POST");
      await page.getByRole("button", { name: language.submit, exact: true }).click();
      expect((await rejected).status(), "错误密码必须由真实后台拒绝").toBe(401);
      await expect(page.getByTestId("login-card").getByRole("alert")).toBeVisible();
      await expect(page.getByPlaceholder(language.username)).toHaveValue(email);
      await page.getByPlaceholder(language.password).fill(password);
      const accepted = page.waitForResponse(res => /\/login$/.test(new URL(res.url()).pathname) && res.request().method() === "POST" && res.status() === 200);
      await page.getByRole("button", { name: language.submit, exact: true }).click();
      token = (await (await accepted).json()).token;
      expect(token, "登录必须返回可用于真实管理接口的会话").toBeTruthy();
      await expect(page).toHaveURL(/\/ui(?:\/|\?|$)/);
      const headers = { Authorization: `Bearer ${token}` };
      const models = await page.request.get(`${consoleURL}/gw/model/info`, { headers });
      expect(models.status(), "控制台代理必须连接容器内网关并通过管理鉴权").toBe(200);
      expect((await models.json()).data, "模型管理接口必须返回模型数组").toEqual(expect.any(Array));
      const health = await page.request.get(`${gatewayURL}/health/liveliness`);
      expect(health.status(), "公开网关必须可访问").toBe(200);
      const denied = await page.request.post(`${gatewayURL}/v1/chat/completions`, {
        data: { model: "deployment-unconfigured-model", messages: [{ role: "user", content: "Hello" }] },
      });
      expect(denied.status(), "数据面必须拒绝无密钥调用，不能因部署配置绕过鉴权").toBe(401);
      const ended = await page.request.post(`${consoleURL}/gw/logout`, { headers });
      expect(ended.status(), "会话清理必须成功").toBe(200);
      const replay = await page.request.get(`${consoleURL}/gw/model/info`, { headers });
      expect(replay.status(), "退出后的会话必须失效").toBe(401);
      token = undefined;
      expect(failures, "控制台不能出现未捕获的运行错误").toEqual([]);
    } finally {
      if (token) {
        const cleanup = await page.request.post(`${consoleURL}/gw/logout`, { headers: { Authorization: `Bearer ${token}` } });
        expect(cleanup.status(), "失败路径也必须清理部署验收会话").toBe(200);
      }
    }
  });
}
