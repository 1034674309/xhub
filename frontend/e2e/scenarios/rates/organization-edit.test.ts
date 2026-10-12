import { registerRateScenario } from "../../rate-scenario";
/** 独立验证organization-edit额度编辑；前置隔离租户与真实浏览器；验证保存及后台持久化，finally 回收自建数据。 */
registerRateScenario("organization-edit");
