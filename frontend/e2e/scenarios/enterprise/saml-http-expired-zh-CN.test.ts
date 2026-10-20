import { registerSAMLScenario } from "../../saml-scenario";
/** 过期 IdP 证书可配置及导出但不得启用；验证双语警告与真实后台拒绝，场景负责清理。 */
registerSAMLScenario("zh-CN", false, "expired");
