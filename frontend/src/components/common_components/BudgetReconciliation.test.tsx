import { render, screen, waitFor, cleanup, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { I18nProvider, translate } from "@/i18n";
import { BudgetReconciliation, type ReconciliationReservation } from "./BudgetReconciliation";

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock("../networking", () => ({ apiClient: api }));
const row: ReconciliationReservation = { reservation_id: "attempt-user", call_id: "call-test", key_id: "key-test", amount: 0.01, state: "uncertain", started_at: "2026-10-21T00:00:00Z", fingerprint: "a".repeat(64), model: "model-original", supplier: "fixture", ledger_cost: 0.0002, can_reconcile: true, blocked_reason: "" };
/** 构造已授权列表响应；参数为条目与页码，无网络副作用，测试结束由mock重置。 */
const response = (rows = [row], page = 1, total = rows.length) => ({ reservations: rows, total, page, page_size: 50 });
/** 渲染独立对账区域；参数为语言、权限、刷新回调，返回测试DOM，卸载清理由afterEach负责。 */
function setup(locale: "en" | "zh-CN", allowed = true, onReconciled = vi.fn().mockResolvedValue(undefined)) {
  render(<I18nProvider initialLocale={locale}><BudgetReconciliation keyId="key-test" accessToken="session" allowed={allowed} onReconciled={onReconciled} /></I18nProvider>);
  return onReconciled;
}
beforeEach(() => { vi.resetAllMocks(); localStorage.clear(); document.cookie = "xhub_locale=; max-age=0; path=/"; api.get.mockResolvedValue(response()); });
afterEach(cleanup);

/** 前置双语管理员与已核实无收费结果；必填依据阻止空提交，成功保留调用已有费用并使用后台审计回执刷新余额，模型与证据原文不变。 */
it.each(["zh-CN", "en"] as const)("%s reconciles no-charge and displays actual audit receipt", async locale => {
  const tr = (key: string, values?: Record<string, string | number>) => translate(locale, key, values);
  const refreshed = setup(locale);
  await userEvent.click(await screen.findByRole("button", { name: tr("budgetReconciliation.openAttempt", { attempt: row.reservation_id }) }));
  await userEvent.click(screen.getByRole("button", { name: tr("budgetReconciliation.confirm"), exact: true }));
  expect(screen.getByRole("alert")).toHaveTextContent(tr("budgetReconciliation.invalidEvidence")); expect(api.post).not.toHaveBeenCalled();
  expect(screen.getByText(tr("budgetReconciliation.deltaHelp"))).toBeVisible();
  await userEvent.type(screen.getByLabelText(tr("budgetReconciliation.evidence")), "Invoice 用户 checked.");
  api.post.mockImplementation(async () => { api.get.mockResolvedValue(response([])); return { already_resolved: false, resolution: { reservation_id: row.reservation_id, call_id: row.call_id, outcome: "no_charge", verified_cost_usd: 0, cost_delta_usd: 0, previous_cost_usd: 0.0002, updated_cost_usd: 0.0002, actor_id: "admin-record", resolved_at: "2026-10-21T00:01:00Z", evidence: "Invoice 用户 checked." } }; });
  await userEvent.click(screen.getByRole("button", { name: tr("budgetReconciliation.confirm"), exact: true }));
  await waitFor(() => expect(refreshed).toHaveBeenCalledOnce());
  expect(api.post).toHaveBeenCalledWith("/budget/reservations/attempt-user/reconcile", { accessToken: "session", body: { outcome: "no_charge", verified_cost_usd: 0, evidence: "Invoice 用户 checked.", expected_fingerprint: row.fingerprint } });
  expect(await screen.findByText(tr("budgetReconciliation.completed"))).toBeVisible();
  expect(screen.getByText(tr("budgetReconciliation.costChange", { before: 0.0002, after: 0.0002, delta: 0 }))).toBeVisible();
  expect(screen.getByText(tr("budgetReconciliation.evidenceSaved", { evidence: "Invoice 用户 checked." }))).toBeVisible();
  expect(screen.getByText(tr("budgetReconciliation.receipt", { attempt: row.reservation_id, actor: "admin-record", time: "2026-10-21T00:01:00Z" }))).toBeVisible();
  expect(await screen.findByText(tr("budgetReconciliation.empty"))).toBeVisible();
});

/** 前置双语已收费结果；零/非法费用不提交，费用超过hold显示具体差额说明，409保留输入且重读新指纹后才能继续。 */
it.each(["zh-CN", "en"] as const)("%s validates charged amounts and handles stale fingerprints without release", async locale => {
  const tr = (key: string, values?: Record<string, string | number>) => translate(locale, key, values);
  setup(locale);
  await userEvent.click(await screen.findByRole("button", { name: tr("budgetReconciliation.openAttempt", { attempt: row.reservation_id }) }));
  await userEvent.selectOptions(screen.getByLabelText(tr("budgetReconciliation.outcome")), "charged");
  await userEvent.type(screen.getByLabelText(tr("budgetReconciliation.evidence")), "Verified charge.");
  await userEvent.type(screen.getByLabelText(tr("budgetReconciliation.cost")), "0");
  await userEvent.click(screen.getByRole("button", { name: tr("budgetReconciliation.confirm"), exact: true }));
  expect(screen.getByRole("alert")).toHaveTextContent(tr("budgetReconciliation.invalidCost")); expect(api.post).not.toHaveBeenCalled();
  await userEvent.clear(screen.getByLabelText(tr("budgetReconciliation.cost"))); await userEvent.type(screen.getByLabelText(tr("budgetReconciliation.cost")), "0.02");
  expect(screen.getByText(tr("budgetReconciliation.overHold", { cost: "0.02", amount: 0.01 }))).toBeVisible();
  api.post.mockRejectedValueOnce({ status: 409 });
  await userEvent.click(screen.getByRole("button", { name: tr("budgetReconciliation.confirm"), exact: true }));
  expect(await screen.findByText(tr("budgetReconciliation.conflict"))).toBeVisible();
  expect(screen.getByLabelText(tr("budgetReconciliation.evidence"))).toHaveValue("Verified charge.");
  expect(screen.getByRole("button", { name: tr("budgetReconciliation.confirm"), exact: true })).toBeDisabled();
  api.get.mockResolvedValue(response([{ ...row, fingerprint: "b".repeat(64) }]));
  await userEvent.click(screen.getByRole("button", { name: tr("budgetReconciliation.reload") }));
  await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument(); expect(screen.queryByText(tr("budgetReconciliation.completed"))).not.toBeInTheDocument();
});

/** 前置读取失败及后台阻止对账的记录；双语错误不能伪装空列表，重试成功后只显示原因、不能提交在途预占。 */
it.each(["zh-CN", "en"] as const)("%s separates load errors from blocked reservations", async locale => {
  const tr = (key: string) => translate(locale, key);
  api.get.mockRejectedValueOnce(new Error("offline")); setup(locale);
  expect(await screen.findByText(tr("budgetReconciliation.loadFailed"))).toBeVisible();
  expect(screen.queryByText(tr("budgetReconciliation.empty"))).not.toBeInTheDocument();
  api.get.mockResolvedValue(response([{ ...row, can_reconcile: false, blocked_reason: "active_attempt" }]));
  await userEvent.click(screen.getByRole("button", { name: tr("hardBudget.retry") }));
  expect(await screen.findByText(tr("budgetReconciliation.blocked.active_attempt"))).toBeVisible();
  expect(screen.queryByRole("button", { name: /attempt-user/ })).not.toBeInTheDocument(); expect(api.post).not.toHaveBeenCalled();
});

/** 非平台管理员不读取列表也不渲染操作，前置明确无权限；组件卸载回收DOM，无外部数据。 */
it("does not query or render reconciliation without platform admin permission", () => {
  setup("en", false); expect(screen.queryByRole("region")).not.toBeInTheDocument(); expect(api.get).not.toHaveBeenCalled(); expect(api.post).not.toHaveBeenCalled();
});

/** 前置多页及未知状态；分页使用真实页码、不可操作记录不出现确认按钮；自动清理DOM。 */
it("paginates and keeps unknown states read-only", async () => {
  api.get.mockResolvedValueOnce(response([row], 1, 51)).mockResolvedValueOnce(response([{ ...row, state: "future", blocked_reason: "new_reason" }], 2, 51));
  setup("en"); await userEvent.click(await screen.findByRole("button", { name: "Next" }));
  await waitFor(() => expect(api.get).toHaveBeenLastCalledWith("/budget/reservations", expect.objectContaining({ query: { key_id: "key-test", state: "uncertain", page: 2, page_size: 50 } })));
  expect(await screen.findByText("This reservation cannot currently be reconciled.")).toBeVisible();
  expect(screen.getByRole("button", { name: "Next" })).toBeDisabled(); expect(api.post).not.toHaveBeenCalled();
});

/** 前置会话权限变化或网络写失败；显示本地化错误且不报告成功，输入保留供核对；无后台操作。 */
it.each([400, 403, 500])("shows explicit write failure for HTTP %s", async status => {
  setup("en"); await userEvent.click(await screen.findByRole("button", { name: "Review reservation attempt-user" }));
  await userEvent.type(screen.getByLabelText("Verification evidence (required)"), "Checked."); api.post.mockRejectedValueOnce({ status });
  await userEvent.click(screen.getByRole("button", { name: "Confirm reconciliation" }));
  expect(await within(screen.getByRole("dialog")).findByRole("alert")).toBeVisible(); expect(screen.queryByText("Reconciliation recorded.")).not.toBeInTheDocument();
});

/** 后台返回已处理回执时不伪造重复成功，刷新失败明确区分已记账与余额未知；实际费用按用户输入提交，无自动退款。 */
it("preserves verified charged amount and audit receipt when balance refresh fails", async () => {
  setup("en", true, vi.fn().mockRejectedValue(new Error("balance unavailable")));
  await userEvent.click(await screen.findByRole("button", { name: "Review reservation attempt-user" }));
  await userEvent.selectOptions(screen.getByLabelText("Verified outcome"), "charged");
  await userEvent.type(screen.getByLabelText("Verified actual cost (USD)"), "0.0025");
  await userEvent.type(screen.getByLabelText("Verification evidence (required)"), "Supplier invoice 42.");
  api.post.mockResolvedValueOnce({ already_resolved: true, resolution: { reservation_id: row.reservation_id, call_id: row.call_id, outcome: "charged", verified_cost_usd: 0.0025, cost_delta_usd: 0.0025, previous_cost_usd: 0.0002, updated_cost_usd: 0.0027, actor_id: "operator", resolved_at: "2026-10-21T00:00:00Z", evidence: "Supplier invoice 42." } });
  await userEvent.click(screen.getByRole("button", { name: "Confirm reconciliation" }));
  expect(await screen.findByText("This exact reconciliation was already recorded; it was not charged again.")).toBeVisible();
  expect(screen.getByText("Call total $0.0002 → $0.0027; this attempt adds $0.0025.")).toBeVisible();
  expect(await screen.findByText("The latest Key balance could not be loaded. Refresh the details before using the displayed balance.")).toBeVisible();
  expect(api.post).toHaveBeenCalledWith("/budget/reservations/attempt-user/reconcile", expect.objectContaining({ body: { outcome: "charged", verified_cost_usd: 0.0025, evidence: "Supplier invoice 42.", expected_fingerprint: row.fingerprint } }));
});
