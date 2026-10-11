import React from "react";
import {cleanup, render, screen} from "@testing-library/react";
import {afterEach, expect, it, vi} from "vitest";
import AdminPanel from "./AdminPanel";
vi.mock("@/components/Settings/AdminSettings/DirectorySettings", () => ({default: () => <div data-testid="settings-tabs" />}));
afterEach(cleanup);

// 前置为无后台依赖的管理员布局；验证背景由主题控制、页签置于画布内且取消嵌套页头卡片；清理组件。
it("管理员画布使用主题白色背景并覆盖主区域间隙", () => {
  render(<AdminPanel />);
  const canvas = screen.getByTestId("admin-settings-canvas");
  expect(canvas).toHaveClass("bg-background", "-m-3", "sm:-m-6");
  expect(canvas).toContainElement(screen.getByTestId("settings-tabs"));
  expect(canvas.querySelector("header")).not.toHaveClass("border");
});
