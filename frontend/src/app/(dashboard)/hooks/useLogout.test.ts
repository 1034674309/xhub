import { renderHook, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
const mocks = vi.hoisted(() => ({ clear: vi.fn(), returnUrl: vi.fn(), error: vi.fn() }));
vi.mock("@/utils/cookieUtils", () => ({ clearTokenCookies: mocks.clear }));
vi.mock("@/utils/returnUrlUtils", () => ({ clearStoredReturnUrl: mocks.returnUrl, getLoginUrl: () => "/ui/login/" }));
vi.mock("@/components/networking", () => ({ getProxyBaseUrl: () => "https://gateway.example" }));
vi.mock("./proxySettings/useProxySettings", () => ({ default: () => ({ PROXY_LOGOUT_URL: "" }) }));
vi.mock("@/lib/toast", () => ({ toast: { error: mocks.error } }));
vi.mock("@/i18n", () => ({ t: (key: string) => key }));
import { useLogout } from "./useLogout";
/** 验证退出顺序、匿名边界与失败恢复；前置模拟浏览器和请求，核对后台撤销后才清理；每次还原 mock，无业务数据。 */
describe("真实会话退出", () => {
  beforeEach(() => { vi.clearAllMocks(); });
  afterEach(() => { vi.unstubAllGlobals(); });
  /** 正常退出等待撤销响应，避免仅清 cookie 后旧凭据仍可重放。 */
  it("后台成功后清理凭据", async () => {
    let resolve!: (response: Response) => void;
    const request = vi.fn(() => new Promise<Response>(done => { resolve = done; }));
    vi.stubGlobal("fetch", request);
    const { result } = renderHook(() => useLogout("session-token"));
    const pending = result.current();
    expect(mocks.clear).not.toHaveBeenCalled();
    resolve(new Response("{}", { status: 200 }));
    await act(async () => { await pending; });
    expect(request).toHaveBeenCalledWith("https://gateway.example/auth/logout", { method: "POST", headers: { Authorization: "Bearer session-token" } });
    expect(mocks.clear).toHaveBeenCalledOnce();
    expect(mocks.returnUrl).toHaveBeenCalledOnce();
  });
  for (const failure of ["http", "network"]) {
    /** 请求失败不能伪装退出成功；保留凭据、显示错误，下一次点击成功后恢复清理。 */
    it("失败保留会话并可重试 " + failure, async () => {
      const request = vi.fn();
      if (failure === "http") request.mockResolvedValueOnce(new Response("", { status: 500 }));
      else request.mockRejectedValueOnce(new Error("offline"));
      request.mockResolvedValueOnce(new Response("{}", { status: 200 }));
      vi.stubGlobal("fetch", request);
      const { result } = renderHook(() => useLogout("session-token"));
      await act(async () => { await result.current(); });
      expect(mocks.clear).not.toHaveBeenCalled();
      expect(mocks.error).toHaveBeenCalledWith("session.logoutFailed");
      await act(async () => { await result.current(); });
      expect(mocks.clear).toHaveBeenCalledOnce();
    });
  }
  /** 无会话不发送空 bearer；只回收浏览器残留，避免制造无效鉴权请求。 */
  it("匿名退出只清理本地", async () => {
    const request = vi.fn(); vi.stubGlobal("fetch", request);
    const { result } = renderHook(() => useLogout(null));
    await act(async () => { await result.current(); });
    expect(request).not.toHaveBeenCalled(); expect(mocks.clear).toHaveBeenCalledOnce();
  });
});
