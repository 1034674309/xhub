import { registerMalformedTerminal } from "../../stream-terminal-scenario";

// 注册独立浏览器终态场景；真实接口、日志和清理由共享场景完成。
registerMalformedTerminal("response");
