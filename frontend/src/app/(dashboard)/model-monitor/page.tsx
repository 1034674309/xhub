"use client";
import useAuthorized from "@/app/(dashboard)/hooks/useAuthorized";
import { ModelMonitorView } from "./view";

/** ModelMonitor 是模型监控路由，无参数；登录状态由宿主提供，返回完整报表页面，无持久化写入。 */
export default function ModelMonitor() {
  const { accessToken } = useAuthorized();
  return <ModelMonitorView accessToken={accessToken} />;
}
