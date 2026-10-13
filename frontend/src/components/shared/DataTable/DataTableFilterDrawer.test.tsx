import type { ColumnDef, ColumnFiltersState } from "@tanstack/react-table";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { DataTable } from "./DataTable";
import { DataTableFilterDrawer } from "./DataTableFilterDrawer";
import { DataTableToolbar } from "./DataTableToolbar";
import { I18nProvider, useI18n } from "@/i18n";

/** 测试用语言切换按钮，无参数，返回切换控件；用于验证打开的抽屉跟随语言更新，无外部副作用。 */
function LocaleControl() {
  const { setLocale } = useI18n();
  return <button onClick={() => setLocale("zh-CN")}>Switch locale</button>;
}

interface Person {
  id: string;
  name: string;
}

const DATA: Person[] = [
  { id: "a", name: "Alice" },
  { id: "b", name: "Bob" },
  { id: "c", name: "Carol" },
];

const columns: ColumnDef<Person, unknown>[] = [
  {
    accessorKey: "name",
    header: "Name",
    meta: { title: "Name" },
    filterFn: (row, columnId, value) => row.getValue<string>(columnId) === value,
    cell: ({ row }) => <span data-testid="name-cell">{row.original.name}</span>,
  },
];

const names = (): (string | null)[] => screen.getAllByTestId("name-cell").map((el) => el.textContent);

function Harness({ initialFilters }: { initialFilters?: ColumnFiltersState }) {
  const [open, setOpen] = useState(false);
  return (
    <DataTable
      data={DATA}
      columns={columns}
      filterMode="client"
      defaultColumnFilters={initialFilters}
      toolbar={(table) => (
        <>
          <DataTableToolbar table={table} onOpenFilters={() => setOpen(true)} />
          <DataTableFilterDrawer table={table} open={open} onOpenChange={setOpen} title="Filters">
            {({ get, set }) => (
              <input
                aria-label="name filter"
                data-testid="draft-name"
                value={(get("name") as string | undefined) ?? ""}
                onChange={(event) => set("name", event.target.value)}
              />
            )}
          </DataTableFilterDrawer>
        </>
      )}
    />
  );
}

describe("DataTableFilterDrawer", () => {
  /** 前置英文 Provider 和打开的抽屉；切换中文后默认按钮更新且草稿不变，测试框架卸载并清理本地状态。 */
  it("translates default actions while preserving the draft", async () => {
    const user = userEvent.setup();
    render(<I18nProvider initialLocale="en"><LocaleControl /><Harness /></I18nProvider>);
    await user.click(screen.getByTestId("datatable-filters-trigger"));
    fireEvent.change(await screen.findByTestId("draft-name"), { target: { value: "Bob" } });
    expect(screen.getByRole("button", { name: "Apply Filters", exact: true })).toBeVisible();
    // 模态抽屉外的测试按钮被隐藏，因此通过其事件切换 Provider 状态。
    fireEvent.click(screen.getByText("Switch locale"));
    expect(screen.getByRole("button", { name: "应用筛选", exact: true })).toBeVisible();
    expect(screen.getByTestId("draft-name")).toHaveValue("Bob");
  });
  it("stages edits and only commits them to the table on Apply", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(names()).toEqual(["Alice", "Bob", "Carol"]);

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    fireEvent.change(await screen.findByTestId("draft-name"), { target: { value: "Bob" } });

    expect(names()).toEqual(["Alice", "Bob", "Carol"]);
    expect(screen.queryByTestId("filter-chip-name")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("filter-drawer-apply"));
    expect(names()).toEqual(["Bob"]);
    expect(screen.getByTestId("filter-chip-name")).toHaveTextContent("Bob");
  });

  it("seeds the draft from committed filters when opened", async () => {
    const user = userEvent.setup();
    render(<Harness initialFilters={[{ id: "name", value: "Bob" }]} />);
    expect(names()).toEqual(["Bob"]);

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    expect(await screen.findByTestId("draft-name")).toHaveValue("Bob");
  });

  it("reset clears the committed filters and the draft", async () => {
    const user = userEvent.setup();
    render(<Harness initialFilters={[{ id: "name", value: "Bob" }]} />);

    await user.click(screen.getByTestId("datatable-filters-trigger"));
    await user.click(await screen.findByTestId("filter-drawer-reset"));

    expect(names()).toEqual(["Alice", "Bob", "Carol"]);
    expect(screen.queryByTestId("filter-chip-name")).not.toBeInTheDocument();
    expect(screen.getByTestId("draft-name")).toHaveValue("");
  });
});
