import {registerArchiveScenario} from "../../enterprise-scenario";
/** 独立归档上传读回与失败恢复；官方 SDK、本地 TLS 存储，finally 停用并清理密钥。 */
registerArchiveScenario(false);
