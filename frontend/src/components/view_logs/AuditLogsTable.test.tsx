import type { ColumnFiltersState, PaginationState } from "@tanstack/react-table";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider, LOCALE_COOKIE, translate, useI18n } from "@/i18n";

import { AuditLogsTable } from "./AuditLogsTable";
import type { AuditLogEntry } from "./AuditLogsTableColumns";

const ROWS: AuditLogEntry[] = [
  {
    id: "log-1",
    updated_at: "2026-07-20T12:00:00Z",
    changed_by: "default_user_id",
    actor_name: "Alice",
    actor_email: "alice@example.com",
    object_name: "Support team",
    changed_by_api_key: "sk-hash-abc",
    action: "created",
    table_name: "LiteLLM_TeamTable",
    object_id: "team-obj-123",
    before_value: {},
    updated_values: { foo: "bar" },
  },
  {
    id: "log-2",
    updated_at: "2026-07-20T11:00:00Z",
    changed_by: "user-42",
    changed_by_api_key: "sk-hash-def",
    action: "deleted",
    table_name: "LiteLLM_UserTable",
    object_id: "user-obj-456",
    before_value: { a: 1 },
    updated_values: {},
  },
];

const FIRST_PAGE: PaginationState = { pageIndex: 0, pageSize: 50 };

/** 用途：以可覆盖的服务端数据渲染表格；参数为属性覆盖，返回回调供断言；组件测试调用，由全局 cleanup 卸载，无后台写入。 */
function renderTable(overrides: Partial<React.ComponentProps<typeof AuditLogsTable>> = {}) {
  const props: React.ComponentProps<typeof AuditLogsTable> = {
    data: ROWS,
    rowCount: ROWS.length,
    isLoading: false,
    isRefreshing: false,
    pagination: FIRST_PAGE,
    onPaginationChange: vi.fn(),
    columnFilters: [],
    onColumnFiltersChange: vi.fn(),
    onRefresh: vi.fn(),
    onViewLog: vi.fn(),
    ...overrides,
  };
  render(<AuditLogsTable {...props} />);
  return props;
}

describe("AuditLogsTable", () => {
  it("shows readable actions, resource names and operators with four columns", () => {
    renderTable();
    expect(screen.getByText("Create Team")).toBeInTheDocument();
    expect(screen.getByText("Delete User")).toBeInTheDocument();
    expect(screen.getByText("Support team")).toBeInTheDocument();
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(screen.getAllByRole("columnheader")).toHaveLength(4);
    expect(screen.queryByText("API Key (Hash)")).not.toBeInTheDocument();
    expect(screen.queryByText("sk-hash-abc")).not.toBeInTheDocument();
  });

  it("opens the detail drawer from the Object ID identity cell with the full row", async () => {
    const user = userEvent.setup();
    const props = renderTable();

    await user.click(screen.getByText("Support team"));

    expect(props.onViewLog).toHaveBeenCalledTimes(1);
    expect(props.onViewLog).toHaveBeenCalledWith(ROWS[0]);
  });

  it("drives the shared footer from the server rowCount and reports page changes", async () => {
    const user = userEvent.setup();
    const onPaginationChange = vi.fn();
    renderTable({ rowCount: 120, onPaginationChange });

    // ceil(120 / 50) = 3 pages, proving rowCount (not data length) feeds the footer
    expect(screen.getByTestId("pagination-page")).toHaveTextContent("Page 1 of 3");

    await user.click(screen.getByTestId("pagination-next"));
    expect(onPaginationChange).toHaveBeenCalledTimes(1);
  });

  it("shows skeleton rows while loading and no data rows", () => {
    renderTable({ isLoading: true, data: [] });

    expect(screen.getAllByTestId("skeleton-row").length).toBeGreaterThan(0);
    expect(screen.queryByText("No audit logs yet")).not.toBeInTheDocument();
  });

  it("uses a distinct empty state for unfiltered vs filtered-empty results", () => {
    const { unmount } = render(
      <AuditLogsTable
        data={[]}
        rowCount={0}
        isLoading={false}
        isRefreshing={false}
        pagination={FIRST_PAGE}
        onPaginationChange={vi.fn()}
        columnFilters={[]}
        onColumnFiltersChange={vi.fn()}
        onRefresh={vi.fn()}
        onViewLog={vi.fn()}
      />,
    );
    expect(screen.getByText("No audit logs yet")).toBeInTheDocument();
    unmount();

    renderTable({ data: [], rowCount: 0, columnFilters: [{ id: "action", value: "created" }] });
    expect(screen.getByText("No matching audit logs")).toBeInTheDocument();
  });

  it("renders the toolbar search box from the search props and forwards typed input", () => {
    const onSearchChange = vi.fn();
    renderTable({ searchValue: "team-", onSearchChange });

    const input = screen.getByPlaceholderText("Search names, emails, IDs or event codes…");
    expect(input).toHaveValue("team-");

    fireEvent.change(input, { target: { value: "team-7" } });
    expect(onSearchChange).toHaveBeenCalledWith("team-7");
  });

  it("treats an active search as a filter for the empty state", () => {
    const emptySearchResult = { data: [], rowCount: 0, searchValue: "zzz", onSearchChange: vi.fn() };
    renderTable(emptySearchResult);

    expect(screen.getByText("No matching audit logs")).toBeInTheDocument();
  });

  it("renders active filter chips with human-readable labels", () => {
    const filters: ColumnFiltersState = [{ id: "action", value: "created" }];
    renderTable({ columnFilters: filters });

    const chip = screen.getByTestId("filter-chip-action");
    expect(chip).toHaveTextContent("Action:");
    expect(chip).toHaveTextContent("Created");
  });

  it("commits a text filter through the filter drawer and reports it to the parent", async () => {
    const user = userEvent.setup();
    const onColumnFiltersChange = vi.fn();
    renderTable({ onColumnFiltersChange });

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    fireEvent.change(await screen.findByPlaceholderText("Enter object ID…"), { target: { value: "obj-9" } });
    await user.click(screen.getByTestId("filter-drawer-apply"));

    expect(onColumnFiltersChange).toHaveBeenCalledTimes(1);
    const arg = onColumnFiltersChange.mock.calls[0][0];
    const committed = typeof arg === "function" ? arg([]) : arg;
    expect(committed).toEqual([{ id: "object_id", value: "obj-9" }]);
  });

  it("shows the human label on both filter triggers while unfiltered", async () => {
    const user = userEvent.setup();
    renderTable();

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    const [action, table] = await screen.findAllByRole("combobox");

    expect(action).toHaveTextContent("All Actions");
    expect(table).toHaveTextContent("All resource types");
  });

  it("shows the human label on the filter triggers for an applied filter", async () => {
    const user = userEvent.setup();
    renderTable({
      columnFilters: [
        { id: "action", value: "created" },
        { id: "table_name", value: "LiteLLM_TeamTable" },
      ],
    });

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    const [action, table] = await screen.findAllByRole("combobox");

    expect(action).toHaveTextContent("Created");
    expect(table).toHaveTextContent("Team");
  });
});

/** 用途：提供真实 Provider 的语言切换入口；无参数，返回测试按钮；只供本文件使用，写入语言偏好由 afterEach 清理。 */
function LocaleControls() {
  const { setLocale } = useI18n();
  return <button onClick={() => setLocale("zh-CN")}>Switch locale</button>;
}

describe("AuditLogsTable Hook and locale regression", () => {
  afterEach(() => {
    localStorage.removeItem(LOCALE_COOKIE);
    document.cookie = `${LOCALE_COOKIE}=; path=/; max-age=0`;
  });

  /** 目的：验证真实模块可在组件渲染前导入；前置不模拟 React/i18n；结果为正常导出，非法模块级 Hook 会直接使文件加载失败；无持久化数据。 */
  it("imports the audit table without calling hooks at module scope", async () => {
    const auditModule = await import("./AuditLogsTable");
    expect(auditModule.AuditLogsTable).toBeTypeOf("function");
  });

  /** 目的：验证英文到中文切换同步更新筛选项、按钮和无匹配状态并保留用户输入；前置真实 Provider、空数据和已应用筛选；全局卸载与 afterEach 清理偏好。 */
  it("updates filters and empty state on locale changes without changing user input", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider initialLocale="en">
        <LocaleControls />
        <AuditLogsTable
          data={[]} rowCount={0} isLoading={false} isRefreshing={false}
          pagination={FIRST_PAGE} onPaginationChange={vi.fn()}
          columnFilters={[{ id: "action", value: "created" }]}
          onColumnFiltersChange={vi.fn()} onRefresh={vi.fn()} onViewLog={vi.fn()}
          searchValue="用户 original input" onSearchChange={vi.fn()}
        />
      </I18nProvider>,
    );
    expect(screen.getByText(translate("en", "No matching audit logs"))).toBeVisible();
    await user.click(screen.getByTestId("datatable-filters-trigger"));
    expect(screen.getAllByRole("combobox")[0]).toHaveTextContent(translate("en", "audit.filters.created"));
    await user.click(screen.getByTestId("filter-drawer-apply"));
    await user.click(screen.getByRole("button", { name: "Switch locale" }));
    expect(screen.getByPlaceholderText(translate("zh-CN", "audit.search"))).toHaveValue("用户 original input");
    expect(screen.getByText(translate("zh-CN", "No matching audit logs"))).toBeVisible();
    expect(screen.getByTestId("filter-chip-action")).toHaveTextContent(translate("zh-CN", "audit.filters.created"));
    await user.click(screen.getByTestId("datatable-filters-trigger"));
    expect(screen.getAllByRole("combobox")[0]).toHaveTextContent(translate("zh-CN", "audit.filters.created"));
    expect(screen.getByTestId("filter-drawer-apply")).toHaveTextContent(translate("zh-CN", "Apply Filters"));
    expect(screen.getByTestId("filter-drawer-reset")).toHaveTextContent(translate("zh-CN", "Reset"));
  });

  /** 目的：未知动作作为边界输入不抛错或伪造翻译；前置服务端返回未知筛选值；结果保留原始协议值；全局 cleanup 卸载，无后台数据。 */
  it("preserves unknown action filter values", () => {
    renderTable({ columnFilters: [{ id: "action", value: "future-action" }] });
    expect(screen.getByTestId("filter-chip-action")).toHaveTextContent("future-action");
  });
});
