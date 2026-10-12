/** 目的：独立验证 tag-management 不支持；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/tag-management", "不支持", false);
