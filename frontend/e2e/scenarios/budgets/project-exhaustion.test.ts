/** 独立验证项目预算耗尽与页面补配恢复；前置本地上游和专属 schema，finally 回收资源。 */
import { registerBudgetBoundary } from "../../budget-boundary-scenario";
registerBudgetBoundary("project", "project 预算耗尽、拒绝归因、补配恢复");
