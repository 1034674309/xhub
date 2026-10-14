import { afterEach, describe, expect, it } from "vitest";
import { randomId } from "./randomId";

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("randomId", () => {
  const original = globalThis.crypto;

  afterEach(() => {
    Object.defineProperty(globalThis, "crypto", { value: original, configurable: true });
  });

  /** 前置 HTTPS 原生 UUID 接口；验证保留其返回值，afterEach 恢复 crypto，无持久数据。 */
  it("uses native UUID in secure contexts", () => {
    Object.defineProperty(globalThis, "crypto", { value: { randomUUID: () => "native-uuid" }, configurable: true });
    expect(randomId()).toBe("native-uuid");
  });

  /** 前置普通 HTTP 仅提供 getRandomValues；验证 UUID v4 格式，afterEach 恢复 crypto，无持久数据。 */
  it("returns a uuid when randomUUID is missing", () => {
    Object.defineProperty(globalThis, "crypto", {
      value: { getRandomValues: (bytes: Uint8Array) => bytes.fill(1) },
      configurable: true,
    });
    expect(randomId()).toMatch(uuid);
  });

  /** 前置旧环境缺少 crypto；验证兼容生成器不崩溃，afterEach 恢复全局对象，无持久数据。 */
  it("returns a uuid when crypto is unavailable", () => {
    Object.defineProperty(globalThis, "crypto", { value: undefined, configurable: true });
    expect(randomId()).toMatch(uuid);
  });
});
