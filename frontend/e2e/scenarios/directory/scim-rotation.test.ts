import { registerSCIMBoundary } from "../../boundary-scenario";

/** 独立验证SCIM token 轮换后旧 token 失效；前置本地真实服务与隔离数据库，断言接口和页面结果，辅助场景负责 finally 清理。 */
registerSCIMBoundary("rotation", "SCIM token 轮换后旧 token 失效");
