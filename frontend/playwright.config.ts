import path from "path";
import { defineConfig, devices } from "@playwright/test";
const uiPort = process.env.E2E_UI_PORT || "3100";
const gatewayPort = process.env.E2E_GW_PORT || "4100";
const upstreamPort = process.env.E2E_UP_PORT || "4110";
const gateway = "http://127.0.0.1:" + gatewayPort;
// 调用方可为并发隔离运行指定专属目录，避免配置、schema 和报告相互覆盖。
const runDir = process.env.E2E_RUN_DIR ? path.resolve(process.env.E2E_RUN_DIR) : path.resolve(__dirname, `../.e2e/run-${process.pid}-${Date.now()}`);
Object.assign(process.env, {
  E2E_GATEWAY: gateway, E2E_UPSTREAM: "http://127.0.0.1:" + upstreamPort,
  E2E_RUN_DIR: runDir, E2E_GW_PORT: gatewayPort, E2E_UP_PORT: upstreamPort,
});
export default defineConfig({
  // 业务场景只从独立 TypeScript 文件注册；reporter 的 node:test 回归不能混入浏览器结果。
  testMatch: "**/*.test.ts",
  outputDir: path.join(runDir, "test-results"),
  testDir: "./e2e", globalSetup: "./e2e/global-setup.ts", globalTeardown: "./e2e/global-teardown.ts",
  timeout: 60_000, expect: { timeout: 15_000 },
  fullyParallel: false, workers: 1, retries: 0, forbidOnly: !!process.env.CI,
  reporter: [["./e2e/checklist-reporter.cjs"], ["html", { open: "never", outputFolder: path.join(runDir, "html") }],
    ["json", { outputFile: path.join(runDir, "results.json") }],
    ["junit", { outputFile: path.join(runDir, "junit.xml") }]],
  use: { baseURL: "http://127.0.0.1:" + uiPort, screenshot: "only-on-failure",
    trace: "retain-on-failure", video: "off", ...devices["Desktop Chrome"] },
  webServer: [
    { command: "bash ../e2e/start-gateway.sh", url: gateway + "/health/liveliness",
      reuseExistingServer: false, timeout: 180_000 },
    { command: "npx next start -p " + uiPort + " -H 127.0.0.1",
      url: "http://127.0.0.1:" + uiPort + "/ui/login/", reuseExistingServer: false, timeout: 180_000,
      // 独立构建目录避免其他聊天重建前端后，浏览器登录和直接接口落在不同网关。
      env: { E2E_BUILD_DIR: process.env.E2E_BUILD_DIR || ".next-e2e", XHUB_GATEWAY_ORIGIN: gateway } },
  ],
});
