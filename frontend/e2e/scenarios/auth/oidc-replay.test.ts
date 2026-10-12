/** 前置本地 TLS IdP 与独立目录身份；验证回调重放不创建会话及可恢复登录；helper finally 回收身份及浏览器。 */
import { registerSSOScenario } from "../../enterprise-scenario";
registerSSOScenario("replay");
