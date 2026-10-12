/** 目的：独立验证 budgets 创建；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/budgets", "创建", false);
