import { registerDocsBoundary } from "../../boundary-scenario";

/** 匿名访问不存在文档，验证 HTTP404 和错误页面；无需业务写入，由运行器回收上下文。 */
registerDocsBoundary(false, "无效动态文档路径返回明确 404 且无业务正文");
