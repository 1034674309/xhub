import { describe, expect, it } from "vitest";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { requireAuthorizationError, requirePageSuccess } from "../e2e/assertions";
import { discoverPages } from "../e2e/routes";
const { evaluateScenarios } = createRequire(import.meta.url)("../e2e/scenario-coverage.cjs");
/** 验证鉴权正常、边界及失败输入；纯函数调用，无持久化或清理。 */
describe("E2E 鉴权契约", () => {
  /** 生命周期拒绝必须明确指定错误类型；同为 401 的错误不能互相冒充，无业务数据。 */
  it("区分封禁和无效凭据", () => {
    const body={error:{type:"key_blocked",code:"401",message:"Blocked"}};
    expect(() => requireAuthorizationError(401,body,401,"key_blocked")).not.toThrow();
    expect(() => requireAuthorizationError(401,body,401)).toThrow(/鉴权契约/);
    expect(() => requireAuthorizationError(500,body,401,"key_blocked")).toThrow(/鉴权契约/);
  });
  for (const [status,type] of [[401,"invalid_api_key"],[403,"forbidden"],[404,"not_found"]] as const) {
    /** 仅明确状态和完整 envelope 匹配可通过。 */
    it("接受明确拒绝 " + status, () => expect(() => requireAuthorizationError(status,{error:{type,code:String(status),message:"Denied"}},status)).not.toThrow());
  }
  for (const [status,body] of [[500,{error:{type:"forbidden",code:"403",message:"Denied"}}],[403,null],[403,{}],[403,{error:{type:"internal_server_error",code:"403",message:"Denied"}}],[403,{error:{type:"forbidden",code:"500",message:"Denied"}}],[403,{error:{type:"forbidden",code:"403",message:""}}]] as const) {
    /** 服务错误与错型或缺失字段不能伪通过；无副作用。 */
    it("拒绝伪通过 " + JSON.stringify([status,body]), () => expect(() => requireAuthorizationError(status,body,403)).toThrow(/鉴权契约/));
  }
});
/** 验证页面契约及动态解析；目录测试 finally 清理临时树。 */
describe("E2E 页面契约", () => {
  /** 正常真实路径返回 200 可通过，无清理。 */
  it("允许真实文档", () => expect(() => requirePageSuccess(200,"/docs/product/quickstart","https://x/docs/product/quickstart")).not.toThrow());
  for (const [status,route,url] of [[404,"/teams","https://x/teams"],[500,"/teams","https://x/teams"],[undefined,"/teams","https://x/teams"],[200,"/docs/[slug]","https://x/docs/[slug]"],[200,"/teams","https://x/ui/login/"]] as const) {
    /** 404、缺响应、参数字面量和登录降级均失败。 */
    it("拒绝无效页面 " + JSON.stringify([status,route,url]), () => expect(() => requirePageSuccess(status,route,url)).toThrow(/页面契约/));
  }
  /** 条件 notFound 不代表退役；未知动态路由 fail closed；清理临时目录。 */
  it("发现真实路径", () => {
    const root=fs.mkdtempSync(path.join(os.tmpdir(),"xhub-pages-"));
    try {
      for (const route of ["(dashboard)/teams","docs/[[...slug]]"]) {fs.mkdirSync(path.join(root,route),{recursive:true});fs.writeFileSync(path.join(root,route,"page.tsx"),"if (!article) notFound()");}
      expect(discoverPages(root)).toEqual([{route:"/docs/[[...slug]]",removed:false,paths:["/docs","/docs/product/quickstart","/docs/api/chat-completions"]},{route:"/teams",removed:false,paths:["/teams"]}]);
      fs.mkdirSync(path.join(root,"items/[id]"),{recursive:true});fs.writeFileSync(path.join(root,"items/[id]/page.tsx"),"");
      expect(() => discoverPages(root)).toThrow(/动态路由缺少业务路径/);
    } finally {fs.rmSync(root,{recursive:true,force:true});}
  });
});
/** 固定业务分母不能被跳过、失败、缺失、重试或重复绑定缩小；纯函数无需清理。 */
describe("业务场景覆盖门禁", () => {
  const catalog=[{id:"a",file:"e2e/a.test.ts",title:"A"},{id:"b",file:"e2e/b.test.ts",title:"B"}];
  /** 过滤运行仍保留完整分母。 */
  it("保留未执行分母", () => expect(evaluateScenarios(catalog,[{file:"e2e/a.test.ts",title:"A",status:"passed"}])).toMatchObject({total:2,passed:1,unmet:[{id:"b",state:"missing"}]}));
  for (const status of ["failed","skipped","timedOut","interrupted","retried"]) {
    /** 非一次明确通过的状态均未达标。 */
    it("拒绝 " + status, () => expect(evaluateScenarios(catalog,[{file:"e2e/a.test.ts",title:"A",status}]).passed).toBe(0));
  }
  /** 清单配置错误直接失败，无 I/O。 */
  it("拒绝空清单和重复 ID", () => {
    expect(() => evaluateScenarios([], [])).toThrow(/不能为空/);
    expect(() => evaluateScenarios([catalog[0], catalog[0]], [])).toThrow(/ID 重复/);
  });
  /** 每种情况必须有独立文件；重复文件或非业务测试后缀直接失败，无 I/O 与清理。 */
  it("拒绝多个情况复用文件和非 test.ts 文件", () => {
    expect(() => evaluateScenarios([catalog[0], {...catalog[1], file: catalog[0].file}], [])).toThrow(/文件重复/);
    expect(() => evaluateScenarios([{...catalog[0], file: "e2e/a.spec.ts"}], [])).toThrow(/后缀错误/);
  });
  /** 未登记测试必须单列，不能扩充通过分子。 */
  it("暴露未登记测试", () => expect(evaluateScenarios(catalog,[{file:"e2e/new.test.ts",title:"New",status:"passed"}])).toMatchObject({passed:0,unregistered:[{title:"New"}]}));
  /** 不允许最后一次通过覆盖先前失败。 */
  it("拒绝重复绑定", () => expect(evaluateScenarios(catalog,[{file:"e2e/a.test.ts",title:"A",status:"passed"},{file:"e2e/a.test.ts",title:"A",status:"failed"}]).unmet[0].state).toBe("ambiguous"));
});
