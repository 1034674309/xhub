import { registerSessionContinuity } from "../../boundary-scenario";
/** 验证独立成员刷新导航保持会话，密码重置后拒绝并重新登录恢复；前置隔离服务，helper finally 删除成员。 */
registerSessionContinuity("连续导航与刷新保持会话，撤销后跳转并重新登录");
