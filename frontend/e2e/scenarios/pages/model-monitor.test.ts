/** 目的：独立验证模型监控真实页面；前置隔离网关和管理员，验证页面与接口；上下文和 schema 由运行器回收。 */
import { discoverPages } from "../../routes";
import { registerPageScenario } from "../../page-scenario";
registerPageScenario(discoverPages().find(page => page.route === "/model-monitor")!, "/model-monitor");
