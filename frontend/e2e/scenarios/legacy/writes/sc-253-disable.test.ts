import { registerModelWrite } from "../../../writes-scenario";

/** 独立验证模型页面禁用持久化且不调用上游；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerModelWrite("disable", "模型页面禁用持久化且不调用上游");
