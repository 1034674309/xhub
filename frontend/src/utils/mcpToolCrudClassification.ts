import { t } from "@/i18n";
export type CrudOp = "read" | "create" | "update" | "delete" | "unknown";

const DELETE_RE = /\b(delete|remove|destroy|purge|drop|erase|unlink)\b/i;
const CREATE_RE = /\b(create|add|insert|new|post|submit|register|make|generate|write|upload)\b/i;
const UPDATE_RE = /\b(update|edit|modify|change|patch|put|set|rename|move|transform)\b/i;
const READ_RE = /\b(get|read|list|fetch|search|find|query|retrieve|show|view|check|describe|info)\b/i;

export interface MCPToolEntry {
  name: string;
  description?: string;
}

/**
 * Classifies a tool by its name first; falls back to description only when
 * the name alone yields no match. This prevents incidental phrasing in
 * free-form descriptions (e.g. "removes noise from…") from promoting a safe
 * tool into a high-risk bucket.
 *
 * READ is checked before DELETE/UPDATE so that tools like `get_removed_entries`
 * or `list_deleted_items` — where the primary verb is a read operation — are
 * not silently blocked by the delete-by-default policy for new servers.
 */
export function classifyToolOp(name: string, description = ""): CrudOp {
  const nameLower = name.toLowerCase();
  if (READ_RE.test(nameLower)) return "read";
  if (DELETE_RE.test(nameLower)) return "delete";
  if (UPDATE_RE.test(nameLower)) return "update";
  if (CREATE_RE.test(nameLower)) return "create";

  // Only consult description when the name is unrecognised.
  if (description) {
    const descLower = description.toLowerCase();
    if (READ_RE.test(descLower)) return "read";
    if (DELETE_RE.test(descLower)) return "delete";
    if (UPDATE_RE.test(descLower)) return "update";
    if (CREATE_RE.test(descLower)) return "create";
  }

  return "unknown";
}

export function groupToolsByCrud(tools: MCPToolEntry[]): Record<CrudOp, MCPToolEntry[]> {
  const groups: Record<CrudOp, MCPToolEntry[]> = {
    read: [],
    create: [],
    update: [],
    delete: [],
    unknown: [],
  };
  for (const tool of tools) {
    const op = classifyToolOp(tool.name, tool.description);
    groups[op].push(tool);
  }
  return groups;
}

export const CRUD_GROUP_META: Record<
  CrudOp,
  { label: string; description: string; risk: "low" | "medium" | "high" | "unknown" }
> = {
  read: {
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Read"); },
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get description() { return t("Safe operations — fetch, list, search. No side effects."); },
    risk: "low",
  },
  create: {
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Create"); },
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get description() { return t("Add new resources — insert, upload, register."); },
    risk: "medium",
  },
  update: {
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Update"); },
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get description() { return t("Modify existing resources — edit, patch, rename."); },
    risk: "medium",
  },
  delete: {
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Delete"); },
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get description() { return t("Destructive operations — remove, purge, destroy."); },
    risk: "high",
  },
  unknown: {
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get label() { return t("Other"); },
    /** 渲染时读取当前语言，避免初始化冻结文案；无写入副作用。 */ get description() { return t("Operations that could not be automatically classified."); },
    risk: "unknown",
  },
};
