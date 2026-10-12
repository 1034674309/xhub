import { registerRateScenario } from "../../rate-scenario";
/** 独立验证 tpm-rollback；前置独立租户和真实浏览器；验证接口与持久化结果，finally 清理自建数据。 */
registerRateScenario("tpm-rollback");
