// 部署验收配置：仅连接显式指定的真实控制台和网关，不启动夹具或修改既有业务数据。
const path = require("node:path");
const { defineConfig } = require("../../frontend/node_modules/@playwright/test");
const reportDir = process.env.XHUB_DEPLOY_REPORT_DIR || path.resolve(__dirname, "../../output/deployment-smoke");

module.exports = defineConfig({
  testDir: __dirname,
  testMatch: "*.test.cjs",
  workers: 1,
  retries: 0,
  timeout: 60000,
  expect: { timeout: 15000 },
  outputDir: path.join(reportDir, "artifacts"),
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: path.join(reportDir, "html") }],
    ["json", { outputFile: path.join(reportDir, "results.json") }],
  ],
  use: {
    baseURL: process.env.XHUB_DEPLOY_CONSOLE_URL,
    screenshot: "only-on-failure",
    // 登录请求含部署凭据，部署验收不保存网络 trace。
    trace: "off",
  },
});
