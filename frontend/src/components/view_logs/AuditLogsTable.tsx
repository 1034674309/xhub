"use client";

import { ColumnFiltersState, OnChangeFn, PaginationState } from "@tanstack/react-table";
import { ScrollText } from "lucide-react";
import { useMemo, useState } from "react";

import {
  DataTable,
  DataTableFilterDrawer,
  DataTableFilterField,
  DataTableToolbar,
} from "@/components/shared/DataTable";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

import { AuditLogEntry, getAuditLogsTableColumns } from "./AuditLogsTableColumns";
import { useT, t, useI18n } from "@/i18n";
import { auditResourceType } from "./auditPresentation";

/** 审计列表的服务端数据、分页和筛选状态，以及由父面板处理的用户操作回调。 */
interface AuditLogsTableProps {
  data: AuditLogEntry[];
  rowCount: number;
  isLoading: boolean;
  isRefreshing: boolean;
  pagination: PaginationState;
  onPaginationChange: OnChangeFn<PaginationState>;
  columnFilters: ColumnFiltersState;
  onColumnFiltersChange: OnChangeFn<ColumnFiltersState>;
  searchValue?: string;
  onSearchChange?: (value: string) => void;
  onRefresh: () => void;
  onViewLog: (log: AuditLogEntry) => void;
}

const ALL_VALUE = "all";

// 模块加载时只保存协议值与翻译键；翻译由组件内的 Hook 完成，避免导入页面时触发非法 Hook 调用。
const ACTION_OPTIONS = ["created", "updated", "deleted", "rotated", "read", "bound"].map((value) => ({
  value,
  label: "audit.filters." + value,
}));
const TABLE_OPTIONS = [
  { label: "audit.resources.key", value: "LiteLLM_VerificationToken" },
  { label: "audit.resources.team", value: "LiteLLM_TeamTable" },
  { label: "audit.resources.user", value: "LiteLLM_UserTable" },
  { label: "audit.resources.organization", value: "LiteLLM_OrganizationTable" },
  { label: "audit.resources.model", value: "LiteLLM_ProxyModelTable" },
  { label: "audit.resources.project", value: "project" },
  { label: "audit.resources.team_member", value: "team_member" },
  { label: "audit.resources.route_template", value: "route_template" },
  { label: "audit.resources.request_log", value: "request_log" },
];

const FILTER_LABELS: Record<string, string> = {
  object_id: "Object ID",
  changed_by: "audit.operator",
  team_id: "Team ID",
  key_hash: "audit.keyID",
  action: "Action",
  table_name: "audit.resourceType",
};

const formatFilterValue = (columnId: string, value: unknown): string => {
  const raw = String(value);
  if (columnId === "action") {
    const label = ACTION_OPTIONS.find((option) => option.value === raw)?.label;
    return label ? t(label) : raw;
  }
  if (columnId === "table_name") {
    return auditResourceType({ table_name: raw });
  }
  return raw;
};

/** 根据 filtered 返回空列表或无匹配结果的本地化提示；由审计表格调用，无外部副作用。 */
function AuditLogsEmptyState({ filtered }: { filtered: boolean }) {
  const t = useT();
  return (
    <div className="flex flex-col items-center gap-1 py-6">
      <div className="mb-1 flex size-10 items-center justify-center rounded-lg bg-muted">
        <ScrollText className="size-5 text-muted-foreground" />
      </div>
      <div className="text-sm font-medium text-foreground">
        {filtered ? t("No matching audit logs") : t("No audit logs yet")}
      </div>
      <div className="max-w-xs text-center text-sm text-muted-foreground">
        {filtered
          ? t("No audit log entries match your filters.")
          : t("Administrative changes to keys, teams, users, and models will appear here.")}
      </div>
    </div>
  );
}

/** 接收父面板的服务端列表及操作回调，返回可分页、筛选和查看详情的审计表格；语言切换时重新翻译筛选项，不修改用户输入。 */
export function AuditLogsTable({
  data,
  rowCount,
  isLoading,
  isRefreshing,
  pagination,
  onPaginationChange,
  columnFilters,
  onColumnFiltersChange,
  searchValue,
  onSearchChange,
  onRefresh,
  onViewLog,
}: AuditLogsTableProps) {
  const t = useT();
  const { locale } = useI18n();
  const ACTION_FILTER_ITEMS = [
    { value: ALL_VALUE, label: t("All Actions") },
    ...ACTION_OPTIONS.map((option) => ({ value: option.value, label: t(option.label) })),
  ];
  const TABLE_FILTER_ITEMS = [
    { value: ALL_VALUE, label: t("audit.allResources") },
    ...TABLE_OPTIONS.map((option) => ({ value: option.value, label: t(option.label) })),
  ];
  const [filtersOpen, setFiltersOpen] = useState(false);
  const columns = useMemo(() => getAuditLogsTableColumns({ onViewLog }), [onViewLog, locale]);
  const hasActiveSearch = Boolean(searchValue?.trim());

  return (
    <DataTable
      data={data}
      columns={columns}
      getRowId={(row) => row.id}
      paginationMode="server"
      pagination={pagination}
      onPaginationChange={onPaginationChange}
      rowCount={rowCount}
      filterMode="server"
      columnFilters={columnFilters}
      onColumnFiltersChange={onColumnFiltersChange}
      isLoading={isLoading}
      loadingMessage={t("Loading audit logs…")}
      noDataMessage={<AuditLogsEmptyState filtered={columnFilters.length > 0 || hasActiveSearch} />}
      size="compact"
      toolbar={(table) => (
        <>
          <DataTableToolbar
            table={table}
            searchValue={searchValue}
            onSearchChange={onSearchChange}
            searchPlaceholder={t("audit.search")}
            onRefresh={onRefresh}
            isRefreshing={isRefreshing}
            onOpenFilters={() => setFiltersOpen(true)}
            filterLabels={Object.fromEntries(Object.entries(FILTER_LABELS).map(([key, label]) => [key, t(label)]))}
            formatFilterValue={formatFilterValue}
            showViewOptions={false}
          />
          <DataTableFilterDrawer
            table={table}
            open={filtersOpen}
            onOpenChange={setFiltersOpen}
            title={t("Filters")}
            description={t("Narrow down audit log entries")}
            applyLabel={t("common.applyFilters")}
            resetLabel={t("Reset")}
          >
            {({ get, set }) => (
              <>
                <DataTableFilterField label={t("Object ID")}>
                  <Input
                    value={(get("object_id") as string) ?? ""}
                    onChange={(event) => set("object_id", event.target.value)}
                    placeholder={t("Enter object ID…")}
                  />
                </DataTableFilterField>
                <DataTableFilterField label={t("audit.operator")}>
                  <Input
                    value={(get("changed_by") as string) ?? ""}
                    onChange={(event) => set("changed_by", event.target.value)}
                    placeholder={t("Enter user ID…")}
                  />
                </DataTableFilterField>
                <DataTableFilterField label={t("Team ID")}>
                  <Input
                    value={(get("team_id") as string) ?? ""}
                    onChange={(event) => set("team_id", event.target.value)}
                    placeholder={t("Enter team ID…")}
                  />
                </DataTableFilterField>
                <DataTableFilterField label={t("audit.keyID")}>
                  <Input
                    value={(get("key_hash") as string) ?? ""}
                    onChange={(event) => set("key_hash", event.target.value)}
                    placeholder={t("Enter key ID…")}
                  />
                </DataTableFilterField>
                <DataTableFilterField label={t("Action")}>
                  <Select
                    items={ACTION_FILTER_ITEMS}
                    value={(get("action") as string) ?? ALL_VALUE}
                    onValueChange={(value) => set("action", value === ALL_VALUE ? undefined : value)}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder={t("All Actions")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_VALUE}>{t("All Actions")}</SelectItem>
                      {ACTION_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.label)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </DataTableFilterField>
                <DataTableFilterField label={t("audit.resourceType")}>
                  <Select
                    items={TABLE_FILTER_ITEMS}
                    value={(get("table_name") as string) ?? ALL_VALUE}
                    onValueChange={(value) => set("table_name", value === ALL_VALUE ? undefined : value)}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder={t("audit.allResources")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_VALUE}>{t("audit.allResources")}</SelectItem>
                      {TABLE_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.label)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </DataTableFilterField>
              </>
            )}
          </DataTableFilterDrawer>
        </>
      )}
    />
  );
}
