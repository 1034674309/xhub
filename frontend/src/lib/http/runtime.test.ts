import { afterEach, describe, expect, it, vi } from "vitest";
import { getAuthHeaderName, getRequestBaseUrl } from "./runtime";

describe("runtime request config defaults", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("resolves the default base URL from NEXT_PUBLIC_BASE_URL before a getter is registered", () => {
    vi.stubEnv("NEXT_PUBLIC_BASE_URL", "https://proxy.example.com/");
    expect(getRequestBaseUrl()).toBe("https://proxy.example.com");
  });

  /** 前置未设置构建地址；验证 typed client 经同源代理，afterEach 恢复环境，无持久数据。 */
  it("defaults to the same-origin gateway proxy when NEXT_PUBLIC_BASE_URL is unset", () => {
    vi.stubEnv("NEXT_PUBLIC_BASE_URL", "");
    expect(getRequestBaseUrl()).toBe("/gw");
  });

  it("defaults the auth header name to Authorization", () => {
    expect(getAuthHeaderName()).toBe("Authorization");
  });
});
