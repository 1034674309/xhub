import { describe, expect, it, vi } from "vitest";
import type { Page } from "@playwright/test";
// 导航辅助函数自身不启动 Playwright；使用可控制的页面边界验证重试和失败传播。
vi.mock("@playwright/test", () => ({ expect: vi.fn() }));
import { stableGoto } from "../e2e/helpers";

/** 构造确定性导航边界；statuses 为每次响应，url 为最终页面；返回 Page 形状，无浏览器或业务副作用。 */
function navigation(statuses: (number | Error | undefined)[], url = "http://local/ui/teams/") {
  const goto = vi.fn().mockImplementation(async () => {
    const status = statuses.shift();
    if (status instanceof Error) throw status;
    return status === undefined ? null : { status: () => status };
  });
  return { goto, waitForLoadState: vi.fn().mockResolvedValue(undefined), waitForTimeout: vi.fn().mockResolvedValue(undefined), url: () => url };
}
/** 验证正常、边界和失败导航；纯函数边界模拟，无数据与清理需求。 */
describe("真实页面导航契约", () => {
  /** 正常 200 只访问一次并保留业务路径。 */
  it("成功导航一次", async () => {
    const page = navigation([200]); await stableGoto(page as unknown as Page, "/teams");
    expect(page.goto).toHaveBeenCalledTimes(1); expect(page.goto).toHaveBeenCalledWith("/ui/teams/", { waitUntil: "commit" });
  });
  /** 短暂服务错误和被中断导航可恢复，最多三次。 */
  it("短暂失败恢复", async () => {
    const page = navigation([new Error("navigation interrupted"), 503, 200]);
    await stableGoto(page as unknown as Page, "/teams"); expect(page.goto).toHaveBeenCalledTimes(3);
  });
  for (const failure of [404, 500, undefined, new Error("navigation interrupted")]) {
    /** 所有尝试失败必须传播，不能吞异常或将空响应当成成功。 */
    it("持续失败不能伪通过 " + String(failure), async () => {
      const page = navigation([failure, failure, failure]);
      await expect(stableGoto(page as unknown as Page, "/teams")).rejects.toThrow(); expect(page.goto).toHaveBeenCalledTimes(3);
    });
  }
  /** 200 登录页面也不能作为受保护业务页面成功。 */
  it("登录降级失败", async () => {
    const page = navigation([200, 200, 200], "http://local/ui/login/");
    await expect(stableGoto(page as unknown as Page, "/teams")).rejects.toThrow(/页面契约/);
  });
});
