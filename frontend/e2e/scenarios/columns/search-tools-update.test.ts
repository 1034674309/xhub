/** 目的：独立验证 search-tools 更新；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/search-tools", "更新", true);
