import { registerDocsBoundary } from "../../boundary-scenario";

/** 独立验证有效动态文档路径返回 200 及真实正文；前置本地真实服务与隔离数据库，断言接口和页面结果，辅助场景负责 finally 清理。 */
registerDocsBoundary(true, "有效动态文档路径返回 200 及真实正文");
