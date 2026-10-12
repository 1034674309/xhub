import { registerKeyWrite } from "../../../writes-scenario";

/** 独立验证密钥页面封禁立即拒绝且不调用上游；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerKeyWrite("block", "密钥页面封禁立即拒绝且不调用上游");
