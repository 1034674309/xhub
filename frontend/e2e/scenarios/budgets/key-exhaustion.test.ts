/** 独立验证key 预算耗尽、拒绝归因、补配恢复；前置本地上游和专属 schema，finally 回收资源。 */
import { registerBudgetBoundary } from "../../budget-boundary-scenario";
registerBudgetBoundary("key", "key 预算耗尽、拒绝归因、补配恢复");
