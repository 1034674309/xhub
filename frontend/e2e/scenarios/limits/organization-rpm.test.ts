/** 独立验证organization rpm 页面配置、拒绝和恢复；自建数据由 finally 与专属 schema 清理。 */
import { registerLimitBoundary } from "../../limit-boundary-scenario";
registerLimitBoundary("organization", "rpm", "organization RPM 页面配置、真实拒绝、修正恢复");
