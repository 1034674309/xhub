import { chooseKeyTeam, chooseOrganization } from "./helpers";

import { expect, test } from "@playwright/test";

import { loginAdmin, t, uiPath } from "./helpers";
/** 用途：注册独立场景「Virtual Keys shows named table and Create Key」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC042() {


test("Virtual Keys shows named table and Create Key", async ({ page }) => {
  await loginAdmin(page);
  await expect(page.getByRole("button", { name: t("pages.apiKeys.create") })).toBeVisible();
  await expect(page.getByPlaceholder(t("pages.apiKeys.searchPlaceholder"))).toBeVisible();
});
}
/** 用途：注册独立场景「Create New Key issues a secret and lists the alias」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC043() {


test("Create New Key issues a secret and lists the alias", async ({ page }) => {
  await loginAdmin(page);
  await page.getByTestId("create-key-button").click();
  await chooseKeyTeam(page);
  await expect(page.getByRole("heading", { name: t("pages.apiKeys.create") })).toBeVisible();
  await page.getByLabel(t("Key Name")).fill("e2e-virtual-key");
  await page.getByRole("button", { name: t("pages.apiKeys.createSubmit"), exact: true }).click();
  await expect(page.getByText(t("pages.apiKeys.saveKey"))).toBeVisible({ timeout: 15_000 });
  await expect(page.getByRole("dialog").locator("pre").filter({ hasText: /sk-/ })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByText("e2e-virtual-key").first()).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Models page lists the configured model」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC044() {


test("Models page lists the configured model", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/models-and-endpoints"));
  await expect(page.getByText("gpt-4o-mini").first()).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Playground shows a model picker」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC045() {


test("Playground shows a model picker", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/playground"));
  await expect(page.locator(`input[placeholder="${t("Select a Model")}"]`)).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Internal Users lists the logged-in admin」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC046() {


test("Internal Users lists the logged-in admin", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/users"));
  await expect(page.getByText("admin").first()).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Create Team lists the new alias」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC047() {


test("Create Team lists the new alias", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/teams"));
  await page.getByTestId("create-team-button").click();
  await chooseOrganization(page);
  await expect(page.getByRole("heading", { name: t("pages.teams.create") })).toBeVisible();
  await page.getByTestId("team-name-input").fill("e2e-team");
  await page.getByTestId("create-team-submit").click();
  await expect(page.getByText("e2e-team").first()).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Create User adds an account with a password」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC048() {


test("Create User adds an account with a password", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/users"));
  await page.getByRole("button", { name: `+ ${t("pages.users.invite")}` }).click();
  await expect(page.getByRole("heading", { name: t("pages.users.invite") })).toBeVisible();
  await page.getByLabel(t("pages.users.userEmail")).fill("e2e-user@example.com");
  await page.getByLabel(t("Initial password")).fill("e2e-pass-1234");
  await page.getByRole("dialog").getByRole("button", { name: t("pages.users.invite") }).click();
  await expect(page.getByText("e2e-user@example.com").first()).toBeVisible({ timeout: 15_000 });
});
}
/** 用途：注册独立场景「Playground sends a chat turn」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC049() {


test("Playground sends a chat turn", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/playground"));
  await page.locator(`input[placeholder="${t("Select a Model")}"]`).click();
  await page.getByRole("option", { name: /gpt-4o-mini/ }).click();
  await page.getByTestId("chat-composer-input").fill("hello from e2e");
  await page.getByTestId("chat-send-button").click();
  await expect(page.getByTestId("message-surface").filter({ hasText: "hello from e2e" })).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByTestId("message-surface").filter({ hasText: /ok|e2e-ok/ })).toBeVisible({
    timeout: 15_000,
  });
});
}
/** 用途：注册独立场景「Organizations create lists the new org」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC050() {


test("Organizations create lists the new org", async ({ page }) => {
  await loginAdmin(page);
  await page.goto(uiPath("/organizations"));
  await page.getByRole("button", { name: t("pages.organizations.create") }).click();
  await expect(page.getByRole("heading", { name: t("pages.organizations.createTitle") })).toBeVisible();
  await page.getByLabel(t("Organization Name")).fill("e2e-org");
  await page.getByRole("button", { name: t("pages.organizations.createTitle"), exact: true }).click();
  await expect(page.getByText("e2e-org").first()).toBeVisible({ timeout: 15_000 });
});
}
