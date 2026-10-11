import React from "react";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import SettingsSelect from "./SettingsSelect";

afterEach(cleanup);

/** 前置为带标签的原生下拉；验证名称、默认值、选择回调和装饰图标无额外可访问控件，结束清理 DOM。 */
it("主题下拉保留标签和原生选择行为", async () => {
  const change = vi.fn();
  render(<label>Region<SettingsSelect defaultValue="z0" onChange={change}><option value="z0">East</option><option value="z1">North</option></SettingsSelect></label>);
  const select = screen.getByRole("combobox", {name: "Region"});
  expect(select).toHaveValue("z0");
  await userEvent.setup().selectOptions(select, "z1");
  expect(select).toHaveValue("z1");
  expect(change).toHaveBeenCalledOnce();
  expect(screen.queryByRole("img")).not.toBeInTheDocument();
});

/** 前置为必填空值及禁用控件；验证浏览器校验拒绝空值，禁用后不触发变更，结束清理 DOM。 */
it("空值校验和禁用边界保持原生语义", async () => {
  const change = vi.fn();
  const {rerender} = render(<label>Team<SettingsSelect required defaultValue="" onChange={change}><option value="">Choose</option><option value="team">Team</option></SettingsSelect></label>);
  expect((screen.getByRole("combobox") as HTMLSelectElement).checkValidity()).toBe(false);
  rerender(<label>Team<SettingsSelect disabled defaultValue="" onChange={change}><option value="">Choose</option><option value="team">Team</option></SettingsSelect></label>);
  await userEvent.setup().selectOptions(screen.getByRole("combobox"), "team");
  expect(screen.getByRole("combobox")).toBeDisabled();
  expect(change).not.toHaveBeenCalled();
});
