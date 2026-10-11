"use client";
import { useEffect, useState } from "react";
import { useT } from "@/i18n";
import { enterpriseLoginOptions } from "@/lib/http/enterprise";

/** 加载企业登录入口；无参数，返回登录链接或空界面；失败保留本地登录，不存储任何身份秘密。 */
export default function EnterpriseLogin() {
 const t = useT(); const [options, setOptions] = useState<{id: string; name: string; url: string}[]>([]);
 useEffect(() => { let alive = true; enterpriseLoginOptions().then((rows) => { if (alive) setOptions(rows); }).catch(() => {}); return () => { alive = false; }; }, []);
 return <div>{options.map((option) => <a className="my-3 block rounded border p-3" key={option.id} href={option.url}>{t("enterprise.login", { name: option.name })}</a>)}</div>;
}
