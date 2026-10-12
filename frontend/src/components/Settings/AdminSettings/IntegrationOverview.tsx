"use client";

import React, { useEffect, useState } from "react";
import { Archive, Inbox, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { useT } from "@/i18n";
import { deleteSSO, readEnterprise, saveEnterprise, type ArchiveStatus, type EnterpriseConfiguration as Configuration } from "@/lib/http/enterprise";
import EnterpriseConfiguration from "./EnterpriseConfiguration";

/** 读取真实集成状态并展示空态、摘要和编辑表单；id/name 指向 SSO，省略则为全局 Kodo。
 * publicOrigin 用于生成回调，onDeleted 通知列表刷新，SSO 与归档均使用独立弹窗；读取失败禁止写入，开关成功才变更摘要。
 * 凭据只存在编辑表单内存；摘要使用脱密快照，组件卸载阻止异步读取更新。
 */
export default function IntegrationOverview({id, name, publicOrigin = "", onDeleted}: {id?: string; name?: string; publicOrigin?: string; onDeleted?:()=>void}) {
  const t = useT();
  const oidc = !!id;
  const path = oidc ? "/admin/enterprise/sso/" + encodeURIComponent(id!) : "/admin/archive/kodo";
  const [configuration, setConfiguration] = useState<Configuration | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const [editing, setEditing] = useState(false);
  const [deleting,setDeleting]=useState(false);

  /** 删除当前 SSO；无参数，返回 Promise，失败保持确认窗及原卡片，不触及 SCIM，确认按钮调用。 */
  async function remove() {
    if (!id || busy) return;setBusy(true);setError(false);
    try {await deleteSSO(id);setDeleting(false);onDeleted?.();}catch{setError(true);}finally{setBusy(false);}
  }
  useEffect(() => {
    let alive = true;
    setLoading(true); setError(false);
    readEnterprise<Configuration | ArchiveStatus>(path).then((snapshot) => {
      if (alive) setConfiguration(oidc ? snapshot as Configuration : (snapshot as ArchiveStatus).configuration);
    }).catch((failure: unknown) => {
      if (!alive) return;
      // 尚未保存的 OIDC 返回标准 404，权限、网络和存储失败不可伪装为空态。
      if (oidc && (failure as {status?: number})?.status === 404) setConfiguration(null);
      else setError(true);
    }).finally(() => {if (alive) setLoading(false);});
    return () => {alive = false;};
  }, [path, oidc]);

  /** 保存已加载配置的启用位；无参数，返回 Promise。后台继续校验 discovery/凭据，失败保持原状态，空配置不允许启用。 */
  async function toggle() {
    if (!configuration) return;
    setBusy(true); setError(false);
    const next = {...configuration, enabled: !configuration.enabled};
    try {await saveEnterprise(path, next); setConfiguration(next);} catch {setError(true);} finally {setBusy(false);}
  }

  const summaryFields = oidc ? ["issuer", "client_id"] : ["bucket", "prefix"];
  const title = t(oidc ? "enterprise.ssoHeading" : "enterprise.archiveTitle");
  const Icon = oidc ? ShieldCheck : Archive;
  const displayName = String(configuration?.name ?? name ?? "");
  const action = oidc ? t("enterprise.configureName", {name: displayName}) : t("enterprise.configureArchive");
  return <section className="rounded-xl border bg-card p-6" aria-label={oidc ? t("enterprise.ssoOverviewName", {name: displayName}) : title}>
    <div className="flex items-start gap-3"><Icon aria-hidden="true" className="mt-1 size-5 shrink-0 text-muted-foreground" /><div className="min-w-0"><h3 className="text-lg font-semibold">{title}</h3><p className="mt-1 break-words text-sm text-muted-foreground">{oidc ? t("enterprise.ssoFor", {name: displayName}) : t("enterprise.archiveBrief")}</p></div></div>
    {loading ? <p role="status" className="mt-6 text-sm text-muted-foreground">{t("enterprise.loadingConfiguration")}</p> : <>
      {configuration ? <div className="mt-6 space-y-4 border-t pt-4"><div className="flex flex-wrap items-center justify-between gap-3"><span className="text-sm text-muted-foreground">{t(configuration.enabled ? "directory.enabled" : "directory.disabled")}</span><Switch checked={!!configuration.enabled} disabled={busy || editing} onCheckedChange={toggle} aria-label={oidc ? t("enterprise.ssoEnableName", {name: displayName}) : t("enterprise.archiveEnable")} /></div><dl className="grid gap-4 text-sm sm:grid-cols-2">{summaryFields.map((key) => <div key={key}><dt className="text-muted-foreground">{t("enterprise." + key)}</dt><dd className="mt-1 break-all font-medium">{String(configuration[key] ?? "")}</dd></div>)}</dl><Button variant="outline" onClick={() => setEditing(!editing)} aria-label={action}>{t(editing ? "enterprise.closeConfiguration" : "enterprise.editConfiguration")}</Button></div> : !error && <div className="mt-6 flex min-h-72 flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-6 text-center"><Inbox aria-hidden="true" className="mb-2 size-10 text-muted-foreground/40" /><h4 className="text-lg font-semibold">{t(oidc ? "enterprise.ssoEmpty" : "enterprise.archiveEmpty")}</h4><p className="max-w-xl text-sm text-muted-foreground">{t(oidc ? "enterprise.ssoEmptyHint" : "enterprise.archiveEmptyHint")}</p><Button className="mt-2" onClick={() => setEditing(true)} aria-label={action}>{t(oidc ? "enterprise.configureSSO" : "enterprise.configureArchive")}</Button></div>}
    </>}
    {oidc && configuration && <Button className="mt-3 text-destructive" variant="outline" disabled={busy} onClick={()=>{setError(false);setDeleting(true);}} aria-label={t("enterprise.deleteSSOName",{name: displayName})}>{t("directory.delete")}</Button>}
    <Dialog open={editing} onOpenChange={setEditing}><DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl"><DialogTitle>{t(oidc ? "enterprise.configureSSO" : "enterprise.configureArchive")}</DialogTitle><DialogDescription>{t(oidc ? "enterprise.ssoFor" : "enterprise.archiveBrief", {name: displayName})}</DialogDescription>{editing && <EnterpriseConfiguration id={id} embedded publicOrigin={publicOrigin} initialConfiguration={configuration ?? undefined} onSaved={(saved) => {setConfiguration(saved); setError(false);setEditing(false);}} />}</DialogContent></Dialog>
    <Dialog open={deleting} onOpenChange={(open)=>{if(!busy)setDeleting(open);}}><DialogContent><DialogTitle>{t("enterprise.deleteSSOName",{name: displayName})}</DialogTitle><DialogDescription>{t("enterprise.deleteSSOHint")}</DialogDescription>{error && <p role="alert">{t("enterprise.error")}</p>}<div className="flex justify-end gap-2"><Button variant="outline" disabled={busy} onClick={()=>setDeleting(false)}>{t("directory.cancel")}</Button><Button variant="destructive" disabled={busy} onClick={remove}>{t("directory.confirmDelete")}</Button></div></DialogContent></Dialog>
    {error && !deleting && <p role="alert" className="mt-4 text-sm text-destructive">{t("enterprise.error")}</p>}
  </section>;
}
