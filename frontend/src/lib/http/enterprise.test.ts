import { afterEach, describe, expect, it, vi } from "vitest";
const get = vi.hoisted(() => vi.fn());
vi.mock("./client", () => ({createApiClient: () => ({get})}));
vi.mock("./runtime", () => ({getAuthHeaderName: () => "Authorization", getAuthToken: () => "test-admin-token", getRequestBaseUrl: () => "/gw"}));
import { readSSOMetadata } from "./enterprise";

describe("SAML metadata admin contract", () => {
  afterEach(() => vi.resetAllMocks());
  // 前置管理员接口返回XML；验证配置ID编码、鉴权和原文保持，纯mock无需外部清理。
  it("reads saved XML through the authenticated metadata endpoint", async () => {
    const xml = '<EntityDescriptor entityID="urn:original"/>';
    get.mockResolvedValue({metadata: xml});
    expect(await readSSOMetadata("id/with space")).toBe(xml);
    expect(get).toHaveBeenCalledWith("/admin/enterprise/sso/id%2Fwith%20space/metadata", {accessToken:"test-admin-token"});
  });
  // 前置空或非法接口形状；验证拒绝下载伪XML，afterEach清理mock，不发送网络。
  it.each([{}, {metadata:""}, {metadata:"   "}, {metadata:42}])("rejects missing metadata: %j", async (result) => {
    get.mockResolvedValue(result);
    await expect(readSSOMetadata("saved")).rejects.toThrow("Invalid SAML metadata response");
  });
  // 前置鉴权/网络失败；验证原错误交给表单展示固定翻译，不伪造下载成功。
  it("propagates API failure", async () => {
    get.mockRejectedValue(new Error("denied"));
    await expect(readSSOMetadata("saved")).rejects.toThrow("denied");
  });
});
