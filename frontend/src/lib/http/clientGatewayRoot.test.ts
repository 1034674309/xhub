import { describe, expect, it } from "vitest";
import { resolveClientGatewayRoot } from "./resolveApiBase";

/** 验证客户端文档地址在同源、公开地址和反向代理部署中均可供CLI使用；纯函数测试无网络或数据清理。 */
describe("resolveClientGatewayRoot", () => {
  /** 正常与边界地址保留代理前缀，调用方传入当前后台地址和页面源，返回绝对网关根而非浏览器相对路径。 */
  it.each([
    ["/gw", "http://console.example:3000", "http://console.example:3000/gw"],
    ["/gw/prefix/v1/", "https://console.example", "https://console.example/gw/prefix"],
    ["https://gateway.example/prefix/v1/", "https://console.example", "https://gateway.example/prefix"],
    ["http://gateway.example:4000", undefined, "http://gateway.example:4000"],
    [undefined, "https://console.example", "https://console.example/gw"],
  ])("resolves %s at %s", (registeredBase, pageOrigin, expected) => {
    expect(resolveClientGatewayRoot({ registeredBase, pageOrigin })).toBe(expected);
  });
  /** 无合法HTTP源或包含认证/查询参数的配置不能生成可复制示例；返回空值让阅读器保留占位符，不泄漏用户配置。 */
  it.each([
    ["/gw", undefined],
    ["/gw", "invalid"],
    ["javascript:alert(1)", "https://console.example"],
    ["https://user:secret@gateway.example", undefined],
    ["https://gateway.example?token=x", undefined],
    ["https://gateway.example/#fragment", undefined],
  ])("rejects invalid client base %s", (registeredBase, pageOrigin) => {
    expect(resolveClientGatewayRoot({ registeredBase, pageOrigin })).toBe("");
  });
});
