import { registerLogoutReplay } from "../../boundary-scenario";

/** 独立验证退出后旧 token 后台拒绝及页面恢复；前置本地真实服务与隔离数据库，断言接口和页面结果，辅助场景负责 finally 清理。 */
registerLogoutReplay("退出后旧 token 后台拒绝及页面恢复");
