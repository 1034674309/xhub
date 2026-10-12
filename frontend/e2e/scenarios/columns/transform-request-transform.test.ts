/** 目的：独立验证 transform-request 转换；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/transform-request", "转换", false);
