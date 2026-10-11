"use client";
import React, { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {enterprisePublicOrigin, type DirectoryOrganization} from "@/lib/http/directory";
import SettingsSelect from "./SettingsSelect";
import { useT } from "@/i18n";
import { archiveAction, createSSO, readEnterprise, saveEnterprise, type ArchiveStatus, type EnterpriseConfiguration as Configuration } from "@/lib/http/enterprise";

/** 展示 SSO 或 Kodo 管理表单；id 为目录连接，省略时配置归档；返回国际化可访问界面。
 * sso 支持无 SCIM 的独立创建，organizations 为组织选项；publicOrigin 为初始公开源，initialConfiguration 为脱密快照。
 * embedded 去除嵌套卡片，onSaved 接收成功后的脱密配置，关闭弹窗卸载本表单并清除秘密。
 * 配置可显式重载，说明与队列明细默认折叠；不把秘密存入 localStorage，保存后清空凭据，失败保留用户输入以便修正。
 */
export default function EnterpriseConfiguration({ id, sso = false, organizations = [], publicOrigin = "", initialConfiguration, embedded = false, onSaved }: { id?: string; sso?: boolean; organizations?: DirectoryOrganization[]; publicOrigin?: string; initialConfiguration?: Configuration; embedded?: boolean; onSaved?: (configuration: Configuration) => void }) {
 const t = useT(); const oidc = sso || !!id;
 const [publicURL,setPublicURL]=useState(publicOrigin);
 const origin=enterprisePublicOrigin(publicURL);
 const [provider, setProvider] = useState("oidc");
 const path = oidc ? "/admin/enterprise/sso/" + encodeURIComponent(id ?? "") : "/admin/archive/kodo";
 const fields = oidc ? ["issuer", "client_id", "client_secret", "redirect_uri", "return_uri", "identity_claim"] : ["bucket", "region", "prefix", "access_key", "secret_key"];
 const [value, setValue] = useState<Configuration>(initialConfiguration ?? (oidc ? { identity_claim: "sub", enabled: false } : { region: "z0", prefix: "billing", interval_seconds: 2, batch_size: 20, enabled: false }));
 const [status, setStatus] = useState<ArchiveStatus | null>(null);
 const [busy, setBusy] = useState(false); const [result, setResult] = useState<"success" | "error" | null>(null);

 /** 加载配置及归档状态；无参数，返回 Promise，首次未配置显示默认值，接口错误显示固定翻译。 */
 async function load() { setBusy(true); setResult(null); try {
  if (oidc) { const configuration = await readEnterprise<Configuration>(path); setValue(configuration); }
  else { const snapshot = await readEnterprise<ArchiveStatus>(path); setStatus(snapshot); if (snapshot.configuration) setValue(snapshot.configuration); }
 } catch { setResult("error"); } finally { setBusy(false); } }
 /** 保存原始输入；event 为表单事件，返回 Promise；成功后清空密码，失败不会重放写入。 */
 async function save(event: React.FormEvent) { event.preventDefault(); setBusy(true); setResult(null); try {
  const saved = oidc && !id ? await createSSO(value) : (await saveEnterprise(path, value), value); const copy = { ...saved }; for (const key of ["client_secret", "access_key", "secret_key"]) delete copy[key]; setValue(copy); onSaved?.(copy); setResult("success");
 } catch { setResult("error"); } finally { setBusy(false); } }
 /** 执行归档操作；action 为探针/失败重试，返回 Promise，失败显示可观察提示，不泄露服务响应。 */
 async function action(action: "test" | "retry") { setBusy(true); setResult(null); try { await archiveAction(action); setResult("success"); } catch { setResult("error"); } finally { setBusy(false); } }
 /** 由管理员明确生成回调地址；使用已校验公开源，不自动覆盖已加载的生产配置。 */
 function generateURLs() { setValue((old) => ({ ...old, redirect_uri: origin + "/auth/enterprise/callback", return_uri: origin + "/ui/login" })); }
 return <section className={(oidc || embedded ? "" : "rounded-xl border bg-card p-6 ") + "[&_label]:flex [&_label]:flex-col [&_label]:gap-2 [&_label]:text-sm [&_summary]:cursor-pointer [&_summary]:text-sm [&_summary]:font-medium"} aria-label={embedded ? undefined : t(oidc ? "enterprise.oidcTitle" : "enterprise.archiveTitle")}>
  <h3 className="text-base font-semibold">{t(oidc ? "enterprise.oidcTitle" : "enterprise.archiveTitle")}</h3>
  <p className="mt-1 text-sm text-muted-foreground">{t(oidc ? "enterprise.oidcBrief" : "enterprise.archiveBrief")}</p>
  <details className="my-4 text-muted-foreground"><summary>{t("enterprise.setupHelp")}</summary><p className="mt-3 text-sm">{t(oidc ? "enterprise.oidcHint" : "enterprise.archiveHint")}</p><ol className="mt-3 space-y-2 text-sm">{(oidc ? ["oidcStep1", "oidcStep2", "oidcStep3"] : ["archiveStep1", "archiveStep2", "archiveStep3"]).map((step) => <li key={step}>{t("enterprise." + step)}</li>)}</ol>{oidc && <p className="mt-3 text-sm">{t("enterprise.providerHint")}</p>}</details>
  <div className="flex flex-wrap gap-2">
  {oidc && <label className="w-full max-w-xl">{t("enterprise.publicURL")}<Input type="url" value={publicURL} placeholder="https://console.example.com" onChange={(event)=>setPublicURL(event.target.value)} /></label>}
  {oidc && <Button variant="outline" type="button" disabled={!origin || busy} onClick={generateURLs}>{t("enterprise.generateURLs")}</Button>}
  {(!oidc || id) && <Button variant="outline" type="button" disabled={busy} onClick={load}>{t("enterprise.load")}</Button>}
  </div>
  <form onSubmit={save} className="my-5 grid max-w-3xl gap-4 sm:grid-cols-2">
   {oidc && !id && <label>{t("directory.organization")}<SettingsSelect required value={String(value.organization_id ?? "")} onChange={(event)=>setValue((old)=>({...old,organization_id:event.target.value}))}><option value="">{t("directory.choose")}</option>{organizations.map((org)=><option key={org.organization_id} value={org.organization_id}>{org.organization_alias || org.organization_id}</option>)}</SettingsSelect></label>}
   {oidc && <label>{t("enterprise.ssoName")}<Input required value={String(value.name ?? "")} onChange={(event)=>setValue((old)=>({...old,name:event.target.value}))} /></label>}
   {oidc && <label className="sm:col-span-2">{t("enterprise.provider")}<SettingsSelect value={provider} onChange={(event) => { setProvider(event.target.value); setValue((old) => ({ ...old, identity_claim: event.target.value === "entra" ? "oid" : "sub" })); }}><option value="oidc">OpenID Connect</option><option value="entra">Microsoft Entra ID</option><option value="okta">Okta</option><option value="keycloak">Keycloak</option><option value="auth0">Auth0</option></SettingsSelect></label>}
   {fields.map((field) => <label key={field}>{t("enterprise." + field)}{field === "region" ? <SettingsSelect value={String(value.region ?? "z0")} onChange={(event) => setValue((old) => ({ ...old, region: event.target.value }))}>{["z0", "cn-east-2", "z1", "z2", "na0", "as0"].map((region) => <option key={region} value={region}>{t("enterprise.regions." + region)}</option>)}</SettingsSelect> : <Input autoComplete="off" type={field.endsWith("secret") || field === "secret_key" || field === "access_key" ? "password" : field === "issuer" || field.endsWith("uri") ? "url" : "text"} value={String(value[field] ?? "")} onChange={(event) => setValue((old) => ({ ...old, [field]: event.target.value }))} />}</label>)}
   {!oidc && <>
    <label>{t("enterprise.interval")}<SettingsSelect value={String(value.interval_seconds ?? 2)} onChange={(event) => setValue((old) => ({...old, interval_seconds: Number(event.target.value)}))}>{[2, 30, 60, 300, 900, 3600, 86400].map((seconds) => <option key={seconds} value={seconds}>{t("enterprise.intervals." + seconds)}</option>)}{![2, 30, 60, 300, 900, 3600, 86400].includes(Number(value.interval_seconds ?? 2)) && <option value={String(value.interval_seconds)}>{t("enterprise.intervalCustom", {seconds: value.interval_seconds})}</option>}</SettingsSelect></label>
    <label>{t("enterprise.batchSize")}<Input type="number" min="1" max="200" step="1" required value={String(value.batch_size ?? 20)} onChange={(event) => setValue((old) => ({...old, batch_size: event.target.value === "" ? "" : Number(event.target.value)}))} /></label>
    <div className="space-y-2 rounded-lg bg-muted/40 p-4 text-sm text-muted-foreground sm:col-span-2">
      <p>{t("enterprise.scheduleHint")}</p><p>{t("enterprise.formatHint")}</p>
      <p>{t("enterprise.contentHint")}</p><p>{t("enterprise.archiveSafety")}</p>
      <p className="break-all">{t("enterprise.objectPath", {prefix: value.prefix || "billing"})}</p>
    </div>
   </>}
   <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4 sm:col-span-2"><label className="!flex-row items-center"><Switch checked={!!value.enabled} onCheckedChange={(checked) => setValue((old) => ({ ...old, enabled: checked }))} />{t("enterprise.enabled")}</label>
   <Button type="submit" disabled={busy}>{t("enterprise.save")}</Button></div>
  </form>
  {!oidc && <div className="flex flex-wrap gap-2"><Button variant="outline" type="button" disabled={busy} onClick={() => action("test")}>{t("enterprise.test")}</Button><Button variant="outline" type="button" disabled={busy} onClick={() => action("retry")}>{t("enterprise.retry")}</Button></div>}
  {result && <p className={"mt-3 text-sm " + (result === "error" ? "text-destructive" : "text-muted-foreground")} role={result === "error" ? "alert" : "status"}>{t("enterprise." + result)}</p>}
  {status && <div className="mt-4 rounded-md bg-muted/40 p-3 text-sm" aria-label={t("enterprise.queue")}><p>{t("enterprise.counts", { pending: status.counts.pending ?? 0, processing: status.counts.processing ?? 0, failed: status.counts.failed ?? 0, done: status.counts.done ?? 0 })}</p><details className="mt-3"><summary>{t("enterprise.queueDetails")}</summary><ul className="mt-2 space-y-2 break-all text-muted-foreground">{status.events.map((event) => <li key={event.id}>{t("enterprise.state." + event.state)} · {event.object_key}</li>)}</ul></details></div>}
 </section>;
}
