import { expect, test } from "@playwright/test";

import { GATEWAY, loginAdmin, sessionBearer, stableGoto, t, watchGateway } from "./helpers";
/** 用途：注册独立场景「users can be edited and their team access follows membership changes」；无参数、无返回值。前置真实浏览器、私有数据库和本地上游；验证页面及原始契约，保留原套件前后置清理，运行器最终回收专属 schema。异常由 Playwright 单独记录。 */
export function registerSC245() {


test("users can be edited and their team access follows membership changes", async ({ page }) => {
  test.setTimeout(120_000);
  const guard = watchGateway(page);
  await loginAdmin(page);
  const headers = { Authorization: "Bearer " + await sessionBearer(page) };
  const teamResponse = await page.request.get(GATEWAY + "/team/list", { headers });
  expect(teamResponse.ok()).toBeTruthy();
  const teams = await teamResponse.json();
  const team = teams.find((row: { team_alias: string }) => row.team_alias === "e2e-fixture-team");
  const created = await page.request.post(GATEWAY + "/user/new", { headers, data: { user_email: "user-edit@xhub.local", user_alias: "编辑前", password: "user-edit-password", user_role: "user" } });
  expect(created.ok(), await created.text()).toBeTruthy();
  const account = await created.json();
  try {
    await stableGoto(page, "/users");
    await page.getByTestId(`user-actions-${account.user_id}`).click();
    await page.getByRole("menuitem", { name: t("pages.users.editUser"), exact: true }).click();
    await expect(page.getByLabel(t("pages.users.alias"))).toHaveValue("编辑前");
    await page.getByLabel(t("pages.users.alias")).fill("编辑后");
    await page.getByLabel(t("pages.users.email")).fill("user-edited@xhub.local");
    await page.getByLabel(t("pages.users.maxBudget")).fill("25");
    const saved = page.waitForResponse((response) => response.url().includes("/user/update") && response.request().method() === "POST");
    await page.getByRole("button", { name: t("pages.users.saveChanges"), exact: true }).click();
    expect((await saved).status()).toBe(200);
    await expect(page.getByRole("heading", { name: "user-edited@xhub.local" })).toBeVisible();
    await expect(page.getByText("编辑后", { exact: true })).toBeVisible();
    await page.reload();
    await page.getByRole("button", { name: t("pages.users.editUser"), exact: true }).click();
    await expect(page.getByLabel(t("pages.users.alias"))).toHaveValue("编辑后");
    await page.getByLabel(t("pages.users.alias")).fill("未保存");
    await page.getByRole("button", { name: t("common.cancel"), exact: true }).click();
    await expect(page.getByText("编辑后", { exact: true })).toBeVisible();
    await page.getByRole("tab", { name: t("common.overview"), exact: true }).click();
    await page.getByRole("button", { name: t("pages.users.addTeam"), exact: true }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("combobox", { name: t("pages.users.team"), exact: true }).click();
    await page.getByRole("option", { name: team.team_alias, exact: true }).click();
    await dialog.getByRole("button", { name: t("pages.users.addToTeam"), exact: true }).click();
    await expect(dialog).toHaveCount(0);
    const roleControl = page.getByRole("combobox", { name: `${t("pages.users.memberRole")} — ${team.team_alias}`, exact: true });
    await expect(roleControl).toContainText(t("Team member"));
    await roleControl.click();
    await page.getByRole("option", { name: t("Team admin"), exact: true }).click();
    await expect(roleControl).toContainText(t("Team admin"));
    const memberships = await page.request.get(GATEWAY + "/team/list?user_id=" + account.user_id, { headers });
    expect((await memberships.json()).find((row: { team_id: string }) => row.team_id === team.team_id).user_role).toBe("team_admin");
    await roleControl.click();
    await page.getByRole("option", { name: t("Team member"), exact: true }).click();
    await expect(roleControl).toContainText(t("Team member"));
    const keyResponse = await page.request.post(GATEWAY + "/key/generate", { headers, data: { user_id: account.user_id, team_id: team.team_id, key_alias: "user-edit-key" } });
    expect(keyResponse.ok(), await keyResponse.text()).toBeTruthy();
    const key = await keyResponse.json();
    const inference = await page.request.post(GATEWAY + "/v1/chat/completions", { headers: { Authorization: "Bearer " + key.key }, data: { model: "gpt-4o-mini", messages: [{ role: "user", content: "membership access" }] } });
    expect(inference.ok(), await inference.text()).toBeTruthy();
    expect((await inference.json()).choices[0].message.content).toMatch(/ok/);
    await page.getByRole("button", { name: t("Remove from {value0}").replace("{value0}", team.team_alias), exact: true }).click();
    await page.getByRole("dialog").getByRole("button", { name: t("common.delete"), exact: true }).click();
    await expect(roleControl).toHaveCount(0);
    const revoked = await page.request.post(GATEWAY + "/v1/chat/completions", { headers: { Authorization: "Bearer " + key.key }, data: { model: "gpt-4o-mini", messages: [{ role: "user", content: "removed" }] } });
    expect([401, 403]).toContain(revoked.status());
    await page.getByRole("button", { name: t("Back to Users"), exact: true }).click();
    await expect(page.getByText("user-edited@xhub.local", { exact: true }).first()).toBeVisible();
    guard.assertOk();
  } finally {
    const cleanup = await page.request.post(GATEWAY + "/user/delete", { headers, data: { user_id: account.user_id } });
    expect(cleanup.ok(), await cleanup.text()).toBeTruthy();
  }
});
}
