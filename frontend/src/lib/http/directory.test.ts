import { expect, it } from "vitest";
import { enterprisePublicOrigin } from "./directory";

// 前置为纯 URL 输入；验证公开源标准化和危险/不完整输入拒绝，无网络或持久化，无需清理。
it("公开接入源只接受不带路径的 HTTPS 地址", () => {
 expect(enterprisePublicOrigin("https://console.example.com/")).toBe("https://console.example.com");
 expect(enterprisePublicOrigin("https://console.example.com:8443")).toBe("https://console.example.com:8443");
 for (const raw of ["", "bad", "http://console.example.com", "https://localhost", "https://127.0.0.1", "https://[::1]", "https://user:secret@console.example.com", "https://console.example.com/path", "https://console.example.com?query=1", "https://console.example.com#fragment"]) expect(enterprisePublicOrigin(raw), raw).toBe("");
});
