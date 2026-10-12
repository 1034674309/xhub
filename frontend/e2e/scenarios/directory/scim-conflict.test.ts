import { registerSCIMBoundary } from "../../boundary-scenario";

/** 独立验证SCIM 重复/冲突/失败不留半成品并恢复；前置本地真实服务与隔离数据库，断言接口和页面结果，辅助场景负责 finally 清理。 */
registerSCIMBoundary("conflict", "SCIM 重复/冲突/失败不留半成品并恢复");
