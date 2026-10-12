/** 目的：独立验证 ui-theme 保存；真实页面后台和隔离数据；finally 或脚本回收测试数据。 */
import { registerColumnScenario } from "../../column-scenario";
registerColumnScenario("/ui-theme", "保存", false);
