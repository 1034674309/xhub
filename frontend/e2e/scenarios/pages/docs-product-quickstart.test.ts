/** 目的：独立验证 /docs/product/quickstart 的页面与读接口；前置隔离网关与管理员，异常独立报告；上下文和 schema 由运行器清理。 */
import { discoverPages } from "../../routes";
import { registerPageScenario } from "../../page-scenario";
registerPageScenario(discoverPages().find(page => page.route === '/docs/[[...slug]]')!, '/docs/product/quickstart');
