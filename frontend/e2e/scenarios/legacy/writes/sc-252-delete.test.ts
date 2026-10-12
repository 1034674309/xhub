import { registerKeyWrite } from "../../../writes-scenario";

/** 独立验证密钥页面删除立即撤销且无残留；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerKeyWrite("delete", "密钥页面删除立即撤销且无残留");
