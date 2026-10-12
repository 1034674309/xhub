import { registerMemberWrite } from "../../../view-only-scenario";

/** 独立验证成员创建用户明确拒绝且无数据或上游副作用；前置、结果断言及 finally 清理由共享注册函数负责。 */
registerMemberWrite("user", "成员创建用户明确拒绝且无数据或上游副作用");
