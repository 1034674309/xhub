/** 目的：独立验证 organization models；前置隔离租户及真实登录；验证契约和无副作用，finally 回收自建数据。 */
import { registerVisibilityScenario } from "../../visibility-scenario";
registerVisibilityScenario("organization", "models");
