import { createApiClient } from "./client";
import { getAuthHeaderName, getAuthToken, getRequestBaseUrl } from "./runtime";

export interface DirectoryConnection {
  id: string; organization_id: string; name: string; enabled: boolean; default_team_id?: string | null;
}
const client = createApiClient({ getBaseUrl: getRequestBaseUrl, getAuthHeaderName });

/** 管理员可选组织/团队；标识用于请求，名称仅用于展示，不推断跨组织权限。 */
export interface DirectoryOrganization { organization_id: string; organization_alias: string }
export interface DirectoryTeam { team_id: string; team_alias: string; organization_id: string }
/** 加载管理员组织下拉选项；返回真实接口列表，失败抛错，不写入数据。 */
export function listDirectoryOrganizations(): Promise<DirectoryOrganization[]> {
 return client.get("/organization/list", { accessToken: getAuthToken() });
}
/** 加载管理员团队下拉选项；返回真实团队列表，组件按连接组织过滤，失败抛错。 */
export function listDirectoryTeams(): Promise<DirectoryTeam[]> {
 return client.get("/team/list", { accessToken: getAuthToken() });
}
/** 校验管理员输入的公开 HTTPS 源；返回标准源或空串，禁止路径、凭据及本地开发地址。 */
export function enterprisePublicOrigin(raw: string): string {
 try { const url = new URL(raw); return url.protocol === "https:" && !url.username && !url.password && !url.search && !url.hash && url.pathname === "/" && !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) ? url.origin : ""; } catch { return ""; }
}

export interface DirectoryManagement {
 connection: DirectoryConnection;
 groups: { id: string; name: string }[];
 mappings: { group_id: string; team_id: string; priority: number; enabled: boolean }[];
 users: { user_id: string; userName: string; displayName: string; active: boolean; mode: string; team_id: string; assignment_reason: string }[];
}

/** 读取一致管理快照；id 为连接标识，返回用户归属和组映射，失败抛错，无写入。 */
export function getDirectoryManagement(id: string): Promise<DirectoryManagement> {
 return client.get("/admin/directory/connections/" + encodeURIComponent(id), { accessToken: getAuthToken() });
}

/** 保存显式组映射；参数指定连接、组、团队和优先级，返回完成 Promise；失败事务回滚。 */
export function saveDirectoryMapping(connection_id: string, group_id: string, team_id: string, priority: number, enabled: boolean): Promise<void> {
 return client.put("/admin/directory/mappings", { accessToken: getAuthToken(), body: { connection_id, group_id, team_id, priority, enabled } });
}

/** 设置人工归属或恢复自动；manual 空 team_id 固定无团队，失败抛错且保留当前归属。 */
export function setDirectoryAssignment(user_id: string, mode: string, team_id: string): Promise<void> {
 return client.put("/admin/directory/users/" + encodeURIComponent(user_id) + "/assignment", { accessToken: getAuthToken(), body: { mode, team_id } });
}

/** 读取连接列表；无参数，返回无秘密的连接。管理员页调用，失败抛错，无写副作用。 */
export function listDirectoryConnections(): Promise<DirectoryConnection[]> {
  return client.get("/admin/directory/connections", { accessToken: getAuthToken() });
}

/** 删除指定目录连接；id 为连接标识，管理员确认弹窗调用，返回完成 Promise。
 * 撤销 SCIM 凭据及组映射，保留 SSO、本地用户和计费数据；失败抛错，调用方不得提前移除卡片。
 */
export function deleteDirectoryConnection(id: string): Promise<void> {
  return client.delete("/admin/directory/connections/" + encodeURIComponent(id), { accessToken: getAuthToken() });
}

/** 创建组织目录连接；参数为组织 ID/名称，返回连接及一次性 token。管理员页调用，失败不创建。 */
export function createDirectoryConnection(organization_id: string, name: string): Promise<{ connection: DirectoryConnection; token: string }> {
  return client.post("/admin/directory/connections", { accessToken: getAuthToken(), body: { organization_id, name } });
}

/** 设置连接启用状态；参数为连接及目标状态，返回完成 Promise。不改变组织和默认团队。 */
export function configureDirectoryConnection(connection: DirectoryConnection, enabled: boolean): Promise<void> {
  return client.put("/admin/directory/connections/" + encodeURIComponent(connection.id), {
    accessToken: getAuthToken(), body: { enabled, default_team_id: connection.default_team_id || "" },
  });
}
