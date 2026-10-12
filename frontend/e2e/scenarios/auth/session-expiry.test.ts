import { registerSessionExpiry } from "../../boundary-scenario";
/** 独立验证真实会话自然到期、页面跳转和重新登录恢复；测试控制端只缩短 TTL，schema 回收测试数据。 */
registerSessionExpiry("自然会话到期后页面明确跳转并恢复登录");
