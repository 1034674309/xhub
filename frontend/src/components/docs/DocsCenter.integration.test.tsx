import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/i18n/I18nProvider";
import { LOCALE_COOKIE } from "@/i18n/translate";
import { DocsCenter } from "./DocsCenter";
import { docPages } from "./catalog";
import { readApiMarkdown } from "./apiMarkdown";
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }), usePathname: () => "/docs" }));
vi.mock("@/components/networking", () => ({ getProxyBaseUrl: () => "https://gateway.example.com/prefix/v1/" }));

/** 目的：供应商帮助区分页面配置与未实现音乐协议，禁止教授内联部署 YAML；前置真实双语文章，语言切换保留启动边界与代码清单，无业务写入，afterEach 清理语言。 */
it("explains NewAPI configuration and Suno development in both languages", () => {
  render(<I18nProvider initialLocale="zh-CN"><DocsCenter page={docPages.find((page) => page.id === "help-models")!} /></I18nProvider>);
  expect(screen.getByRole("heading", { name: "NewAPI：连接、部署、密钥、调用" })).toBeInTheDocument();
  expect(screen.getByRole("main")).toHaveTextContent("internal/provider/all/all.go");
  expect(screen.getByRole("main")).toHaveTextContent("当前没有 Suno 执行器");
  expect(screen.getByRole("main")).not.toHaveTextContent("model_list: []");
  expect(screen.getByRole("main")).not.toHaveTextContent("store_model_in_db: true");
  expect(screen.getByRole("main")).toHaveTextContent("不编辑 model_list");
  expect(screen.getByRole("main")).toHaveTextContent("部署级 api_base 优先；供应商地址只补空值。");
  expect(screen.getByRole("main")).not.toHaveTextContent("os.environ/NEWAPI_KEY");
  expect(screen.getByRole("img", { name: "中文模型供应商连接表单" })).toHaveAttribute("src", "/assets/provider-guide/provider-form-zh.jpg");
  expect(screen.getByRole("img", { name: "中文模型部署表单" })).toHaveAttribute("src", "/assets/provider-guide/deployment-form-zh.jpg");
  fireEvent.click(screen.getByRole("button", { name: "English" }));
  expect(screen.getByRole("heading", { name: "NewAPI: connection, deployment, key and call" })).toBeInTheDocument();
  expect(screen.getByRole("main")).toHaveTextContent("There is no Suno executor");
  expect(screen.getByRole("main")).toHaveTextContent("bypass_openai_chat");
  expect(screen.getByRole("main")).not.toHaveTextContent("model_list: []");
  expect(screen.getByRole("main")).not.toHaveTextContent("store_model_in_db: true");
  expect(screen.getByRole("main")).toHaveTextContent("without editing model_list");
  expect(screen.getByRole("main")).toHaveTextContent("Deployment-level api_base values take precedence; provider resolution fills empty addresses.");
  expect(screen.getByRole("img", { name: "English provider connection form" })).toHaveAttribute("src", "/assets/provider-guide/provider-form-en.jpg");
  expect(screen.getByRole("img", { name: "English model deployment form" })).toHaveAttribute("src", "/assets/provider-guide/deployment-form-en.jpg");
  expect(screen.queryByRole("img", { name: "中文模型供应商连接表单" })).not.toBeInTheDocument();
  expect(screen.getByRole("main")).not.toHaveTextContent("os.environ/NEWAPI_KEY");
});

/** 目的：验证内容任务正文和 Bypass 目录没有重复标题；前置真实双语 Markdown，切换语言并搜索仍可访问总览及接口；框架清理 DOM，afterEach 清理语言存储，无业务数据。 */
it("deduplicates protocol titles while preserving bilingual navigation", () => {
  render(
    <I18nProvider initialLocale="zh-CN">
      <DocsCenter page={docPages.find((page) => page.id === "ark")!} apiMarkdown={readApiMarkdown("ark")} />
    </I18nProvider>,
  );
  const main = screen.getByRole("main");
  expect(within(main).getAllByText("内容生成任务协议", { exact: true })).toHaveLength(1);
  const nav = screen.getByRole("navigation", { name: "文档目录" });
  expect(within(nav).getAllByText("原厂协议 Bypass", { exact: true })).toHaveLength(1);
  expect(within(nav).getByRole("link", { name: "协议总览" })).toHaveAttribute("href", "/docs/api/native-bypass");
  fireEvent.click(screen.getByRole("button", { name: "English" }));
  expect(within(main).getAllByText("Content generation task protocol", { exact: true })).toHaveLength(1);
  expect(within(nav).getAllByText("Native protocol Bypass", { exact: true })).toHaveLength(1);
  expect(within(nav).getByRole("link", { name: "Overview", exact: true })).toHaveAttribute(
    "href",
    "/docs/api/native-bypass",
  );
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "Bypass Anthropic" } });
  expect(within(nav).getByRole("link", { name: "Bypass Anthropic Messages", exact: true })).toBeInTheDocument();
  expect(within(nav).getAllByText("Native protocol Bypass", { exact: true })).toHaveLength(1);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "no-such-protocol-xyz" } });
  expect(screen.getByRole("status")).toHaveTextContent("0 documents found");
  expect(within(nav).queryByRole("link")).not.toBeInTheDocument();
});

/** 目的：验证真实双语 Markdown、动态网关及不重复渲染；前置独立 API 文件，语言切换保持面板并更新目录；不调用上游，无业务数据清理。 */
it("renders the shared Markdown API in both languages", () => {
  render(
    <I18nProvider initialLocale="zh-CN">
      <DocsCenter page={docPages.find((page) => page.id === "chat")!} apiMarkdown={readApiMarkdown("chat")} />
    </I18nProvider>,
  );
  expect(screen.getAllByRole("heading", { name: "请求参数" })).toHaveLength(1);
  expect(screen.getByRole("region", { name: "shell 代码面板" })).toHaveTextContent(
    "https://gateway.example.com/prefix/v1/chat/completions",
  );
  expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "English" }));
  expect(screen.getByRole("heading", { name: "Request parameters" })).toBeInTheDocument();
  expect(
    within(screen.getByRole("navigation", { name: "Documentation directory" })).getAllByText("POST").length,
  ).toBeGreaterThan(0);
  expect(screen.getByRole("region", { name: "json code panel" })).toHaveTextContent("chatcmpl-example");
  expect(screen.getByRole("link", { name: "Success response" })).toHaveAttribute("href", "#doc-success-response");
});

/** 每例清理语言 cookie，防止语言切换影响下一例的前置语言，无业务数据。 */
afterEach(() => {
  document.cookie = LOCALE_COOKIE + "=; Max-Age=0; path=/";
  localStorage.clear();
});
describe("documentation reader", () => {
  /** 目的：验证双语搜索空结果、动态计数和原文保留；前置真实组件树，切语言保留输入，空查询恢复目录；测试环境清理 DOM 与存储。 */
  it("keeps the search query across language changes", () => {
    render(
      <I18nProvider initialLocale="zh-CN">
          <DocsCenter page={docPages[0]} />
      </I18nProvider>,
    );
    expect(screen.getByRole("searchbox", { name: "搜索文档" })).toHaveValue("");
    expect(screen.getByRole("searchbox", { name: "搜索文档" })).toHaveAttribute("autocomplete", "off");
    expect(screen.getByRole("searchbox")).toHaveAttribute("id", "xhub-documentation-filter-text");
    expect(screen.getByRole("search")).toHaveAttribute("autocomplete", "off");
    expect(screen.getByRole("heading", { name: "欢迎使用 XHub" })).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox", { name: "搜索文档" }), { target: { value: "用户原文" } });
    expect(screen.getByRole("status")).toHaveTextContent("找到 0 篇文档");
    fireEvent.click(screen.getByRole("button", { name: "English" }));
    expect(screen.getByRole("searchbox", { name: "Search documentation" })).toHaveValue("用户原文");
    expect(screen.getByText("No matching documents. Try another keyword.")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "" } });
    expect(screen.getByRole("navigation", { name: "Documentation directory" })).toBeInTheDocument();
    expect(
      within(screen.getByRole("navigation", { name: "Documentation directory" })).queryByText("POST"),
    ).not.toBeInTheDocument();
  });
  /** 目的：验证复制成功和拒绝后的可见降级；前置英文请求文章与剪贴板边界，复制文本保持英文并显示结果；mock 自动恢复，不写业务数据。 */
  it("reports clipboard success and failure", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    render(
      <I18nProvider initialLocale="en">
        <DocsCenter page={docPages[1]} />
      </I18nProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Copy example" }));
    expect(await screen.findByText("Copied", { selector: "p" })).toBeInTheDocument();
    expect(writeText).toHaveBeenCalledWith(
      "export XHUB_BASE_URL='https://gateway.example.com/prefix'\n" + docPages[1].code,
    );
    writeText.mockRejectedValueOnce(new Error("denied"));
    fireEvent.click(screen.getByRole("button", { name: "Copy example" }));
    expect(await screen.findByText("Copy failed. Select and copy the example manually.")).toBeInTheDocument();
  });
});

/** 目的：验证模型接口字段、锚点、失败响应和双语状态；前置真实翻译树，切换响应与语言保留选中状态；测试后框架清理 DOM，不产生业务数据。 */
it("shows API schemas and translated response states", () => {
  localStorage.clear();
  document.cookie = `${LOCALE_COOKIE}=; path=/; max-age=0`;
  render(
    <I18nProvider initialLocale="zh-CN">
      <DocsCenter page={{ ...docPages.find((page) => page.id === "models")! }} />
    </I18nProvider>,
  );
  expect(screen.getByRole("link", { name: "请求参数 / 响应" })).toHaveAttribute("href", "#listModels");
  expect(screen.getByText("data[].id")).toBeInTheDocument();
  expect(
    within(screen.getByRole("navigation", { name: "文档目录" })).queryByRole("link", { name: "快速开始" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: /下一篇.*对话补全/ })).toHaveAttribute("href", "/docs/api/chat-completions");
  fireEvent.click(screen.getByRole("tab", { name: "401" }));
  expect(screen.getByRole("tabpanel")).toHaveTextContent("XHub 凭据缺失或无效");
  fireEvent.click(screen.getByRole("button", { name: "English" }));
  expect(screen.getByRole("tabpanel")).toHaveTextContent("Missing or invalid XHub credential.");
  expect(screen.getByRole("tab", { name: "401" })).toHaveAttribute("aria-selected", "true");
  expect(
    within(screen.getByRole("navigation", { name: "Documentation directory" })).queryByRole("link", {
      name: "Quickstart",
    }),
  ).not.toBeInTheDocument();
  localStorage.clear();
  document.cookie = `${LOCALE_COOKIE}=; path=/; max-age=0`;
});

/** 目的：验证工具配置复用调试台网关；前置带反向代理前缀的地址，示例去除末尾版本并保留前缀；不调用外部服务，无数据清理。 */
it("uses the playground gateway root in tool configuration", () => {
  render(
    <I18nProvider initialLocale="en">
      <DocsCenter page={docPages.find((page) => page.id === "opencode")!} />
    </I18nProvider>,
  );
  expect(screen.getByLabelText("Request example", { exact: true })).toHaveTextContent(
    '"baseURL": "https://gateway.example.com/prefix"',
  );
  expect(screen.getByLabelText("Request example", { exact: true })).not.toHaveTextContent("YOUR_GATEWAY_HOST");
});
