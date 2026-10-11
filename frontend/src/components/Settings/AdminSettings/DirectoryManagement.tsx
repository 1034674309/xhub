"use client";

import React, { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import SettingsSelect from "./SettingsSelect";
import { useT } from "@/i18n";
import { getDirectoryManagement, saveDirectoryMapping, setDirectoryAssignment, type DirectoryTeam, type DirectoryManagement as Snapshot } from "@/lib/http/directory";

/** 管理一个连接的组映射和人工归属；id 指定连接，返回可访问表单。
 * 管理员显式加载最新快照；写入成功后重读，失败保留输入并展示本地化错误，不缓存身份数据。
 */
export default function DirectoryManagement({ id, teams = [] }: { id: string; teams?: DirectoryTeam[] }) {
 const t = useT();
 const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState(false);
 const [group, setGroup] = useState("");
 const [team, setTeam] = useState("");
 const [priority, setPriority] = useState("0");
 const [enabled, setEnabled] = useState(true);
 const [user, setUser] = useState("");
 const [mode, setMode] = useState("manual");
 const [override, setOverride] = useState("");

 /** 串行执行管理操作并刷新快照；action 可省略用于只读刷新，失败不宣称写入成功。 */
 async function run(action?: () => Promise<void>) {
  setBusy(true); setError(false);
  try { if (action) await action(); setSnapshot(await getDirectoryManagement(id)); }
  catch { setError(true); } finally { setBusy(false); }
 }

 /** 提交组映射表单；事件阻止导航，后端校验团队组织，失败不改用户归属。 */
 function mapping(event: React.FormEvent) {
  event.preventDefault(); void run(() => saveDirectoryMapping(id, group, team, Number(priority), enabled));
 }

 /** 提交人工覆盖表单；自动模式不传团队，人工空串明确固定无团队，失败保留输入。 */
 function assignment(event: React.FormEvent) {
  event.preventDefault(); void run(() => setDirectoryAssignment(user, mode, mode === "auto" ? "" : override));
 }

 return <div className="mt-4 space-y-4">
  <Button variant="outline" type="button" disabled={busy} onClick={() => void run()}>{t("directory.manage")}</Button>
  {error && <p role="alert" className="text-sm text-destructive">{t("directory.error")}</p>}
  {snapshot && <>
   <p className="text-sm text-muted-foreground">{t("directory.assignmentHint")}</p>
   <form onSubmit={mapping} className="grid max-w-3xl gap-4 sm:grid-cols-2">
    <label>{t("directory.group")}<SettingsSelect required value={group} onChange={(e) => setGroup(e.target.value)}><option value="">{t("directory.choose")}</option>{snapshot.groups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}</SettingsSelect></label>
    <label>{t("directory.team")}<SettingsSelect required value={team} onChange={(e) => setTeam(e.target.value)}><option value="">{t("directory.choose")}</option>{teams.map((item) => <option key={item.team_id} value={item.team_id}>{item.team_alias || item.team_id}</option>)}</SettingsSelect></label>
    <label>{t("directory.priority")}<Input type="number" min="0" step="1" required value={priority} onChange={(e) => setPriority(e.target.value)} /></label>
    <label className="!flex-row items-center"><Switch checked={enabled} onCheckedChange={setEnabled} />{t("directory.mappingEnabled")}</label>
    <Button className="w-fit sm:col-span-2" type="submit" disabled={busy}>{t("directory.saveMapping")}</Button>
   </form>
   <ul className="space-y-2 break-words text-sm text-muted-foreground" aria-label={t("directory.mappings")}>{snapshot.mappings.map((m) => <li key={m.group_id}>{snapshot.groups.find((g) => g.id === m.group_id)?.name} → {m.team_id} · {t("directory.priorityValue", { priority: m.priority })} · {t(m.enabled ? "directory.enabled" : "directory.disabled")}</li>)}</ul>
   <form onSubmit={assignment} className="grid max-w-3xl gap-4 border-t pt-4 sm:grid-cols-2">
    <label>{t("directory.user")}<SettingsSelect required value={user} onChange={(e) => setUser(e.target.value)}><option value="">{t("directory.choose")}</option>{snapshot.users.map((u) => <option key={u.user_id} value={u.user_id}>{u.userName}</option>)}</SettingsSelect></label>
    <label>{t("directory.mode")}<SettingsSelect value={mode} onChange={(e) => setMode(e.target.value)}><option value="manual">{t("directory.manual")}</option><option value="auto">{t("directory.auto")}</option></SettingsSelect></label>
    {mode === "manual" && <label>{t("directory.overrideTeam")}<SettingsSelect value={override} onChange={(e) => setOverride(e.target.value)}><option value="">{t("directory.noTeam")}</option>{teams.map((item) => <option key={item.team_id} value={item.team_id}>{item.team_alias || item.team_id}</option>)}</SettingsSelect></label>}
    <Button className="w-fit sm:col-span-2" type="submit" disabled={busy}>{t("directory.saveAssignment")}</Button>
   </form>
   <ul className="space-y-2 break-words text-sm text-muted-foreground" aria-label={t("directory.assignments")}>{snapshot.users.map((u) => <li key={u.user_id}>{u.userName} · {u.team_id || t("directory.noTeam")} · {t(u.mode === "manual" ? "directory.manual" : "directory.auto")} · {t(u.active ? "directory.enabled" : "directory.disabled")}</li>)}</ul>
  </>}
 </div>;
}
