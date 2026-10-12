import { registerModelWrite } from "../../../writes-scenario";

/** 独立验证模型页面删除部署且不可调用；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerModelWrite("delete", "模型页面删除部署且不可调用");
