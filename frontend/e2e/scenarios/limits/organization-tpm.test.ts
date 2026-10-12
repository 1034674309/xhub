/** 独立验证organization tpm 页面配置、拒绝和恢复；自建数据由 finally 与专属 schema 清理。 */
import { registerLimitBoundary } from "../../limit-boundary-scenario";
registerLimitBoundary("organization", "tpm", "organization TPM 页面配置、真实拒绝、修正恢复");
