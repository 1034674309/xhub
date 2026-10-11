import React from "react";
import {cleanup, fireEvent, render, screen, waitFor, within} from "@testing-library/react";
import {afterEach, beforeEach, expect, it, vi} from "vitest";
import {I18nProvider} from "@/i18n";
import * as api from "@/lib/http/enterprise";
import IntegrationOverview from "./IntegrationOverview";
vi.mock("@/lib/http/enterprise");
beforeEach(() => {vi.resetAllMocks(); localStorage.clear(); document.cookie = "xhub_locale=; max-age=0; path=/";});
afterEach(cleanup);

// 前置独立 SSO 快照，中英文验证删除取消、失败保留和成功回调；不调用 SCIM，清理组件/mock。
it.each(["zh-CN", "en"] as const)("%s SSO 删除有独立确认和失败恢复", async (locale) => {
  vi.mocked(api.readEnterprise).mockResolvedValue({name:"原始 Login", enabled:false});
  vi.mocked(api.deleteSSO).mockRejectedValueOnce(new Error("private delete error")).mockResolvedValueOnce();
  const removed = vi.fn();
  render(<I18nProvider initialLocale={locale}><IntegrationOverview id="sso-id" name="old-name" onDeleted={removed}/></I18nProvider>);
  const label = locale === "en" ? "Delete SSO for 原始 Login" : "删除 原始 Login 的单点登录";
  fireEvent.click(await screen.findByRole("button", {name:label}));
  fireEvent.click(screen.getByRole("button", {name:locale === "en" ? "Cancel" : "取消"}));
  expect(api.deleteSSO).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", {name:label}));
  const confirm = screen.getByRole("button", {name:locale === "en" ? "Delete connection" : "删除连接"});
  fireEvent.click(confirm);
  await screen.findByRole("alert");
  expect(removed).not.toHaveBeenCalled();
  expect(screen.queryByText("private delete error")).not.toBeInTheDocument();
  fireEvent.click(confirm);
  await waitFor(()=>expect(removed).toHaveBeenCalledOnce());
  expect(api.deleteSSO).toHaveBeenLastCalledWith("sso-id");
  expect(api.saveEnterprise).not.toHaveBeenCalled();
});

// 前置已有 SCIM 但 OIDC 返回404；验证真实 SSO 空态和配置入口、无假开关；清理组件和模拟接口。
it("目录存在不等于 SSO 已配置", async () => {
  vi.mocked(api.readEnterprise).mockRejectedValue({status:404});
  render(<I18nProvider initialLocale="zh-CN"><IntegrationOverview id="directory" name="研发 Directory" /></I18nProvider>);
  expect(await screen.findByRole("heading", {name:"尚未配置单点登录"})).toBeVisible();
  expect(screen.queryByRole("switch")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", {name:"配置 研发 Directory 的单点登录"}));
  expect(screen.getByRole("dialog", {name:"配置单点登录"})).toBeVisible();
  expect(screen.getByLabelText("OIDC 签发者")).toBeVisible();
  fireEvent.change(screen.getByLabelText("客户端密钥"), {target:{value:"unsaved-secret"}});
  fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", {name:"关闭"}));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  fireEvent.click(screen.getByRole("button", {name:"配置 研发 Directory 的单点登录"}));
  expect(screen.getByLabelText("客户端密钥")).toHaveValue("");
  expect(api.saveEnterprise).not.toHaveBeenCalled();
});

// 前置英文归档读取失败；验证失败不伪装空态，不提供危险写入入口；组件和 mock 在测试后清理。
it("读取失败禁止用默认配置覆盖已有数据", async () => {
  vi.mocked(api.readEnterprise).mockRejectedValue({status:500});
  render(<I18nProvider initialLocale="en"><IntegrationOverview /></I18nProvider>);
  expect(await screen.findByRole("alert")).toHaveTextContent("Operation failed");
  expect(screen.queryByRole("heading", {name:"No archive configuration found"})).not.toBeInTheDocument();
  expect(screen.queryByRole("switch")).not.toBeInTheDocument();
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});

// 前置真实形状的停用 OIDC 快照；验证开关成功才更新、失败保留、编辑自动填充且原名称不翻译；清理 DOM/mock。
it("SSO 独立开关成功持久化失败保留状态", async () => {
  const cfg = {issuer:"https://id.example.com", client_id:"original-client", enabled:false};
  vi.mocked(api.readEnterprise).mockResolvedValue(cfg);
  vi.mocked(api.saveEnterprise).mockResolvedValue();
  render(<I18nProvider initialLocale="en"><IntegrationOverview id="dir" name="研发 Directory" /></I18nProvider>);
  const toggle = await screen.findByRole("switch", {name:"Enable SSO for 研发 Directory"});
  expect(toggle).not.toBeChecked();
  fireEvent.click(toggle);
  await waitFor(() => expect(toggle).toBeChecked());
  expect(api.saveEnterprise).toHaveBeenCalledWith("/admin/enterprise/sso/dir", {...cfg, enabled:true});
  vi.mocked(api.saveEnterprise).mockRejectedValue(new Error("secret failure"));
  fireEvent.click(toggle);
  await screen.findByRole("alert");
  expect(toggle).toBeChecked();
  expect(screen.queryByText("secret failure")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", {name:"Configure SSO for 研发 Directory"}));
  expect(screen.getByLabelText("Client ID")).toHaveValue("original-client");
  expect(toggle).toHaveAttribute("aria-disabled", "true");
});

// 前置归档尚未配置；验证中文空态进入表单、成功后显示独立摘要开关且凭据已清空；测试无真实云调用并清理组件。
it("归档空态保存后变为已配置摘要", async () => {
  vi.mocked(api.readEnterprise).mockResolvedValue({configuration:null, counts:{}, events:[]});
  vi.mocked(api.saveEnterprise).mockResolvedValue();
  render(<I18nProvider initialLocale="zh-CN"><IntegrationOverview /></I18nProvider>);
  fireEvent.click(await screen.findByRole("button", {name:"配置 Kodo 归档"}));
  fireEvent.change(screen.getByLabelText("Kodo 存储空间"), {target:{value:"original-bucket"}});
  fireEvent.change(screen.getByLabelText("Kodo 秘密密钥"), {target:{value:"private-secret"}});
  fireEvent.click(screen.getByRole("button", {name:"保存集成配置"}));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByRole("switch", {name:"启用计费日志归档"})).not.toBeChecked();
  fireEvent.click(screen.getByRole("button", {name:"配置 Kodo 归档"}));
  expect(screen.getByText("original-bucket")).toBeVisible();
  expect(screen.getByLabelText("Kodo 存储空间")).toHaveValue("original-bucket");
  expect(screen.getByLabelText("Kodo 秘密密钥")).toHaveValue("");
});
