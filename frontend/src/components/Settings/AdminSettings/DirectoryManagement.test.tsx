import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/i18n";
import * as api from "@/lib/http/directory";
import DirectoryManagement from "./DirectoryManagement";
vi.mock("@/lib/http/directory");
beforeEach(() => { vi.resetAllMocks(); window.localStorage.clear(); document.cookie = "xhub_locale=; max-age=0; path=/"; });
afterEach(cleanup);

// 前置为中文管理组件和组用户快照；验证映射优先级、人工无团队及恢复自动，清理组件和 mock。
it("管理员可以保存映射、固定无团队并恢复自动", async () => {
 vi.mocked(api.getDirectoryManagement).mockResolvedValue({ connection: {id:"c",organization_id:"o",name:"Directory",enabled:true}, groups:[{id:"g",name:"Engineering"}], mappings:[], users:[{user_id:"u",userName:"member@example.com",displayName:"Member",active:true,mode:"auto",team_id:"",assignment_reason:"none"}] });
 vi.mocked(api.saveDirectoryMapping).mockResolvedValue();
 vi.mocked(api.setDirectoryAssignment).mockResolvedValue();
 render(<I18nProvider initialLocale="zh-CN"><DirectoryManagement id="c" teams={[{team_id:"team",team_alias:"Engineering team",organization_id:"o"}]} /></I18nProvider>);
 fireEvent.click(screen.getByRole("button",{name:"加载目录归属"}));
 await screen.findByLabelText("企业目录组");
 fireEvent.change(screen.getByLabelText("企业目录组"),{target:{value:"g"}});
 fireEvent.change(screen.getByLabelText("目标团队 ID"),{target:{value:"team"}});
 fireEvent.change(screen.getByLabelText("映射优先级"),{target:{value:"2"}});
 fireEvent.click(screen.getByRole("button",{name:"保存组映射"}));
 await waitFor(()=>expect(api.saveDirectoryMapping).toHaveBeenCalledWith("c","g","team",2,true));
 await waitFor(()=>expect(screen.getByRole("button",{name:"保存组映射"})).toBeEnabled());
 fireEvent.click(screen.getByRole("switch",{name:"启用映射"}));
 fireEvent.click(screen.getByRole("button",{name:"保存组映射"}));
 await waitFor(()=>expect(api.saveDirectoryMapping).toHaveBeenCalledWith("c","g","team",2,false));
 await waitFor(()=>expect(screen.getByRole("button",{name:"保存用户归属"})).toBeEnabled());
 fireEvent.change(screen.getByLabelText("目录用户"),{target:{value:"u"}});
 fireEvent.click(screen.getByRole("button",{name:"保存用户归属"}));
 await waitFor(()=>expect(api.setDirectoryAssignment).toHaveBeenCalledWith("u","manual",""));
 await waitFor(()=>expect(screen.getByRole("button",{name:"保存用户归属"})).toBeEnabled());
 fireEvent.change(screen.getByLabelText("归属模式"),{target:{value:"auto"}});
 expect(screen.queryByLabelText("人工团队 ID（留空固定无团队）")).not.toBeInTheDocument();
 fireEvent.click(screen.getByRole("button",{name:"保存用户归属"}));
 await waitFor(()=>expect(api.setDirectoryAssignment).toHaveBeenCalledWith("u","auto",""));
});

// 前置为英文组件且快照读取失败；验证固定错误不泄露后端详情，无写操作，清理组件与 mock。
it("英文读取失败显示本地化错误",async()=>{
 vi.mocked(api.getDirectoryManagement).mockRejectedValue(new Error("secret sql"));
 render(<I18nProvider initialLocale="en"><DirectoryManagement id="c" /></I18nProvider>);
 fireEvent.click(screen.getByRole("button",{name:"Load directory assignments"}));
 await waitFor(()=>expect(screen.getByRole("alert")).toHaveTextContent("Directory operation failed"));
 expect(screen.queryByText("secret sql")).not.toBeInTheDocument();
 expect(api.saveDirectoryMapping).not.toHaveBeenCalled();
});
