/** 目的：独立验证 vector-stores 创建；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/vector-stores", "创建", false);
