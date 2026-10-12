import { registerKeyRotation } from "../../boundary-scenario";

/** 独立验证轮换旧密钥拒绝、新密钥成功且日志身份不变；前置本地真实服务与隔离数据库，断言接口和页面结果，辅助场景负责 finally 清理。 */
registerKeyRotation("轮换旧密钥拒绝、新密钥成功且日志身份不变");
