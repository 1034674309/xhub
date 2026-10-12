import fs from "node:fs";
import path from "node:path";
import { gunzipSync } from "node:zlib";
import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import { GATEWAY, loginAdmin, sessionBearer, stableGoto } from "./helpers";
import { requireAuthorizationError } from "./assertions";
/** 读取本轮本地夹具地址；无参数，返回配置；未启动即失败，不以跳过掩盖闭环缺失。 */
function fixture(): {issuer:string; console:string; control:string} {
 return JSON.parse(fs.readFileSync(path.join(process.env.E2E_RUN_DIR!,"enterprise-fixture.json"),"utf8"));
}
/** 注册独立 SSO 情况；参数为正常/非法 nonce/未知身份；页面配置、IdP 表单、TLS 回调、会话及权限。
 * 每条创建独立身份，finally 删除 SSO/SCIM 并回收独立身份；身份锚点及 schema 最终由脚本回收。 */
export function registerSSOScenario(mode:"valid"|"nonce"|"unknown"|"expired"|"rejected"|"replay") {
 test("SSO 浏览器闭环 " + mode, async ({page,browser}) => {
  test.setTimeout(120_000);const f=fixture();await loginAdmin(page);
  const headers={Authorization:"Bearer "+await sessionBearer(page)};
  const provision=await page.request.post(f.control+"/identity");expect(provision.status()).toBe(200);const identity=await provision.json();
  const context=await browser.newContext({ignoreHTTPSErrors:true});const visitor=await context.newPage();
  try {
   await stableGoto(page,"/admin-panel");await page.getByRole("tab",{name:"单点登录设置"}).click();await page.getByRole("button",{name:"配置单点登录",exact:true}).click();
   const form=page.getByRole("dialog",{name:"配置单点登录"});await form.getByLabel("组织 ID").selectOption(identity.organization_id);await form.getByLabel("单点登录名称").fill("Browser SSO "+mode);
   await form.getByLabel("XHub 公开访问地址").fill("https://console.example.com");await form.getByRole("button",{name:"生成回调和返回地址"}).click();
   // 本地 TLS 夹具使用表单支持的自定义回调/返回地址；公开地址生成器保持公网校验。
   await form.getByLabel("回调地址").fill(f.console+"/auth/enterprise/callback");await form.getByLabel("控制台返回地址",{exact:true}).fill(f.console+"/ui/login");await form.getByLabel("OIDC 签发者").fill(f.issuer);await form.getByLabel("客户端 ID",{exact:true}).fill("xhub-browser");await form.getByLabel("客户端密钥").fill("local-browser-secret");const saved=page.waitForResponse(r=>r.url().includes("/admin/enterprise/sso")&&r.request().method()==="POST");await form.getByRole("button",{name:"保存集成配置"}).click();const result=await saved;expect(result.status(),await result.text()).toBe(201);await expect(form).not.toBeVisible();
   const toggle=page.getByRole("switch",{name:"启用 Browser SSO "+mode+" 的单点登录"});await toggle.click();await expect(toggle).toBeChecked();
   expect((await page.request.post(f.control+"/mode?value="+(mode==="replay"?"valid":mode))).status()).toBe(200);await visitor.goto(f.console+"/ui/login/");
   await visitor.getByRole("link").filter({hasText:"Browser SSO "+mode}).click();await expect(visitor.getByLabel("Employee")).toBeVisible();
   const callback=visitor.waitForResponse(r=>new URL(r.url()).pathname==="/auth/enterprise/callback");await visitor.getByRole("button",{name:"Sign in to identity provider"}).click();const response=await callback;
   if(mode==="valid" || mode==="replay") {
    expect(response.status()).toBe(302);await expect(visitor.getByText("虚拟密钥",{exact:true}).first()).toBeVisible();
    const cookies=await context.cookies();const token=cookies.find(c=>c.name==="token");expect(token?.secure).toBe(true);
    const payload=JSON.parse(Buffer.from(token!.value.split(".")[1],"base64url").toString());const session={Authorization:"Bearer "+payload.key};
    const info=await visitor.request.get(GATEWAY+"/user/info",{headers:session});expect(info.status(),await info.text()).toBe(200);expect((await info.json()).user_info.user_id).toBe(identity.user.id ?? identity.user.user_id);
    const denied=await visitor.request.get(GATEWAY+"/audit/logs",{headers:session});requireAuthorizationError(denied.status(),await denied.json(),403);
    if(mode==="replay") {
     // 已消费的 state/授权码即使复制到新浏览器也不能再次建立会话。
     const replayContext=await browser.newContext({ignoreHTTPSErrors:true});
     try {
      const replay=await replayContext.newPage();const refused=await replay.goto(response.url());
      expect(refused!.status()).toBe(401);expect((await replayContext.cookies()).some(c=>c.name==="token")).toBe(false);
     } finally {await replayContext.close();}
    }
   } else {
    expect(response.status()).toBe(401);expect((await context.cookies()).some(c=>c.name==="token")).toBe(false);await expect(visitor.locator("body")).toContainText(/error|invalid|identity|token|failed/i);
    // 失败不会污染下一次登录；更正 IdP 后从登录入口重新完成真实授权码流程。
    expect((await page.request.post(f.control+"/mode?value=valid")).status()).toBe(200);
    await visitor.goto(f.console+"/ui/login/");await visitor.getByRole("link").filter({hasText:"Browser SSO "+mode}).click();
    const recovered=visitor.waitForResponse(r=>new URL(r.url()).pathname==="/auth/enterprise/callback");await visitor.getByRole("button",{name:"Sign in to identity provider"}).click();expect((await recovered).status()).toBe(302);
    await expect(visitor.getByText("虚拟密钥",{exact:true}).first()).toBeVisible();expect((await context.cookies()).some(c=>c.name==="token"&&c.secure)).toBe(true);
   }
  } finally {
   test.setTimeout(150_000);await context.close();await page.request.post(f.control+"/mode?value=valid");
   const removed=await page.request.delete(GATEWAY+"/admin/enterprise/sso/"+identity.connection_id,{headers});expect([204,404]).toContain(removed.status());
   expect((await page.request.delete(GATEWAY+"/admin/directory/connections/"+identity.connection_id,{headers})).status()).toBe(204);
   const cleanup=await page.request.post(f.control+"/identity/cleanup?organization_id="+identity.organization_id);expect(cleanup.status(),await cleanup.text()).toBe(200);
  }
 });
}
/** 注册归档上传或失败恢复闭环；参数为是否注入故障；真实页面、推理账务、官方 SDK 上传和对象读回。
 * 验证 gzip/manifest 哈希与状态，重试不重复对象；finally 停用并清理密钥，脚本回收 schema/对象服务。 */
export function registerArchiveScenario(retry:boolean) {
 test("归档浏览器闭环 " + (retry?"失败重试":"上传读回"), async ({page})=>{
  test.setTimeout(120_000);const f=fixture();await loginAdmin(page);const headers={Authorization:"Bearer "+await sessionBearer(page)};
  const old=await (await page.request.get(GATEWAY+"/admin/archive/kodo",{headers})).json();
  // 单 worker、独占测试 schema：回收前一场景的 outbox 和本地对象，当前请求必须独立经历完整状态链。
  const reset=await page.request.post(f.control+"/archive/reset");expect(reset.status(),await reset.text()).toBe(200);
  const minted=await page.request.post(GATEWAY+"/key/generate",{headers,data:{key_alias:"archive-roundtrip"}});expect(minted.status()).toBe(200);const key=await minted.json();
  try {
   await stableGoto(page,"/admin-panel");await page.getByRole("tab",{name:"日志设置"}).click();await page.getByRole("button",{name:"配置 Kodo 归档"}).click();const form=page.getByRole("dialog",{name:"配置 Kodo 归档"});
   await form.getByLabel("Kodo 存储空间").fill("xhub-test");await form.getByLabel("Kodo 访问密钥").fill("fake-ak");await form.getByLabel("Kodo 秘密密钥").fill("fake-sk");await form.getByLabel("归档处理间隔").selectOption("86400");await form.getByLabel("每轮最多处理事件数").fill("200");await form.getByRole("switch",{name:"启用集成"}).check();await form.getByRole("button",{name:"保存集成配置"}).click();await expect(form).not.toBeVisible();
   const call=await page.request.post(GATEWAY+"/v1/chat/completions",{headers:{Authorization:"Bearer "+key.key},data:{model:"gpt-4o-mini",messages:[{role:"user",content:"Verify archive roundtrip"}]}});expect(call.status(),await call.text()).toBe(200);const requestID=call.headers()["x-litellm-call-id"];
   /** 读取真实队列当前请求事件；返回事件或 undefined，供轮询等待提交，不写数据。 */
   const event=async()=>{const res=await page.request.get(GATEWAY+"/admin/archive/kodo",{headers});expect(res.status()).toBe(200);return (await res.json()).events.find((e:{request_id:string})=>e.request_id===requestID);};
   await expect.poll(async()=> (await event())?.state).toBe("pending");
   if(retry){await page.request.post(f.control+"/mode?value=archive-fail");const process=await page.request.post(f.control+"/archive/process");expect((await process.json()).error).not.toBe("");await expect.poll(async()=> (await event())?.state).toBe("failed");}
   await page.getByRole("button",{name:"配置 Kodo 归档"}).click();await form.getByRole("button",{name:"加载配置与状态"}).click();
   if(retry){await expect(form.getByLabel("归档队列")).toContainText(/失败 [1-9]/);await page.request.post(f.control+"/mode?value=valid");await form.getByRole("button",{name:"重试失败归档"}).click();await expect.poll(async()=> (await event())?.state).toBe("pending");}
   const process=await page.request.post(f.control+"/archive/process");expect((await process.json()).error).toBe("");await expect.poll(async()=> (await event())?.state).toBe("done");
   await form.getByRole("button",{name:"加载配置与状态"}).click();await expect(form.getByLabel("归档队列")).toContainText(/已归档 [1-9]/);
   const uploaded=await event();const objects:Record<string,string>=await (await page.request.get(f.control+"/objects")).json();const objectKey=uploaded.object_key;expect(objects[objectKey]).toBeTruthy();const data=Buffer.from(objects[objectKey],"base64");const rows=gunzipSync(data).toString().trim().split("\n").map(line=>JSON.parse(line));const row=rows.find(item=>item.usage.request_id===requestID);expect(row, "归档对象必须包含当前请求").toBeTruthy();expect(row.usage.request_id).toBe(requestID);expect(JSON.stringify(row)).not.toContain("Verify archive roundtrip");
   const manifest=JSON.parse(Buffer.from(objects[objectKey+".manifest.json"],"base64").toString());expect(manifest.sha256).toBe(createHash("sha256").update(data).digest("hex"));
   const count=Object.keys(objects).length;expect((await (await page.request.post(f.control+"/archive/process")).json()).processed).toBe(0);expect(Object.keys(await (await page.request.get(f.control+"/objects")).json())).toHaveLength(count);
  } finally {
   await page.request.post(f.control+"/mode?value=valid");const stopped=await page.request.put(GATEWAY+"/admin/archive/kodo",{headers,data:{...old.configuration,bucket:old.configuration?.bucket||"xhub-test",region:old.configuration?.region||"z0",prefix:old.configuration?.prefix||"billing",interval_seconds:old.configuration?.interval_seconds||86400,batch_size:old.configuration?.batch_size||200,enabled:false}});expect(stopped.status(),await stopped.text()).toBe(200);expect((await page.request.post(GATEWAY+"/key/delete",{headers,data:{keys:[key.token_id||key.key]}})).status()).toBe(200);
  }
 });
}
