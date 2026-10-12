import { registerModelWrite } from "../../../writes-scenario";

/** 独立验证模型页面连接测试成功且不修改部署；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerModelWrite("connection", "模型页面连接测试成功且不修改部署");
