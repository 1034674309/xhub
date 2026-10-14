import { describe, expect, it } from "vitest";
import {
  clientSampleBaseUrl,
  resolveConsoleApiBase,
  resolveApiBase,
  resolveGatewayApiBase,
  resolveRequestUrl,
} from "./resolveApiBase";

describe("resolveRequestUrl", () => {
  it("targets the registered base when one is registered", () => {
    expect(
      resolveRequestUrl("/model_group/info", {
        registeredBase: "https://proxy.example.com",
        pageOrigin: "http://localhost:3000",
      }),
    ).toBe("https://proxy.example.com/model_group/info");
  });

  it("falls back to the page origin when no base is registered", () => {
    expect(resolveRequestUrl("/model_group/info", { registeredBase: "", pageOrigin: "http://localhost:3000" })).toBe(
      "http://localhost:3000/model_group/info",
    );
  });

  it("trims a trailing slash so the path is not doubled up", () => {
    expect(resolveRequestUrl("/model_group/info", { registeredBase: "https://proxy.example.com/" })).toBe(
      "https://proxy.example.com/model_group/info",
    );
  });

  it("keeps the path relative when neither a base nor an origin is available", () => {
    expect(resolveRequestUrl("/model_group/info", {})).toBe("/model_group/info");
    expect(resolveRequestUrl("/model_group/info", { registeredBase: null, pageOrigin: null })).toBe(
      "/model_group/info",
    );
  });

  it("preserves an already-serialized query string", () => {
    expect(
      resolveRequestUrl("/model_group/info?model_group=gpt-4o", { registeredBase: "https://proxy.example.com" }),
    ).toBe("https://proxy.example.com/model_group/info?model_group=gpt-4o");
  });
});

describe("resolveApiBase", () => {
  describe("same-origin (no explicit base)", () => {
    it("returns an empty base so requests stay relative", () => {
      expect(resolveApiBase({})).toBe("");
      expect(resolveApiBase({ explicitBase: null, serverRootPath: "/" })).toBe("");
      expect(resolveApiBase({ explicitBase: "", serverRootPath: undefined })).toBe("");
    });

    it("prefixes a relative base with the server root path", () => {
      expect(resolveApiBase({ serverRootPath: "/litellm" })).toBe("/litellm");
    });
  });

  describe("explicit base (split-origin / dev)", () => {
    it("uses the explicit base verbatim when root path is mounted at /", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000", serverRootPath: "/" })).toBe(
        "http://localhost:4000",
      );
    });

    it("appends the server root path to the explicit base", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000", serverRootPath: "/litellm" })).toBe(
        "http://localhost:4000/litellm",
      );
    });
  });

  describe("dedup — base already includes the root path", () => {
    it("does not double the root path", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000/litellm", serverRootPath: "/litellm" })).toBe(
        "http://localhost:4000/litellm",
      );
    });
  });

  describe("normalization", () => {
    it("trims a trailing slash from the explicit base before appending", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000/", serverRootPath: "/litellm" })).toBe(
        "http://localhost:4000/litellm",
      );
    });

    it("trims a trailing slash from the explicit base when there is no root path", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000/", serverRootPath: "/" })).toBe(
        "http://localhost:4000",
      );
    });

    it("adds a leading slash to a root path that lacks one", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000", serverRootPath: "litellm" })).toBe(
        "http://localhost:4000/litellm",
      );
    });

    it("trims a trailing slash from the root path", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000", serverRootPath: "/litellm/" })).toBe(
        "http://localhost:4000/litellm",
      );
    });

    it("ignores whitespace around inputs", () => {
      expect(resolveApiBase({ explicitBase: "  http://localhost:4000  ", serverRootPath: "  /litellm  " })).toBe(
        "http://localhost:4000/litellm",
      );
    });
  });

  describe("multi-segment root paths", () => {
    it("appends a nested root path", () => {
      expect(resolveApiBase({ explicitBase: "http://localhost:4000", serverRootPath: "/team/litellm" })).toBe(
        "http://localhost:4000/team/litellm",
      );
    });

    it("dedups a nested root path", () => {
      expect(
        resolveApiBase({ explicitBase: "http://localhost:4000/team/litellm", serverRootPath: "/team/litellm" }),
      ).toBe("http://localhost:4000/team/litellm");
    });
  });
});

describe("resolveGatewayApiBase", () => {
  it("uses the local gateway origin when the env and the reported base are empty", () => {
    expect(
      resolveGatewayApiBase({
        envBase: "",
        reportedBase: "",
        pageOrigin: "http://localhost:3000",
      }),
    ).toBe("http://localhost:4000");
  });

  it("does not use the page origin when both overrides are missing", () => {
    const base = resolveGatewayApiBase({ pageOrigin: "http://127.0.0.1:3000" });
    expect(base).toBe("http://localhost:4000");
    expect(base).not.toContain(":3000");
    expect(base).not.toContain("/ui");
  });

  it("prefers a reported proxy base over the env and the page", () => {
    expect(
      resolveGatewayApiBase({
        envBase: "https://env.example.com",
        reportedBase: "https://api.example.com/",
        pageOrigin: "http://localhost:3000",
      }),
    ).toBe("https://api.example.com");
  });

  it("uses the env base when the gateway reports nothing", () => {
    expect(
      resolveGatewayApiBase({
        envBase: "https://env.example.com/",
        reportedBase: "  ",
        pageOrigin: "http://localhost:3000",
      }),
    ).toBe("https://env.example.com");
  });
});

describe("clientSampleBaseUrl", () => {
  it("uses the gateway origin when the page is the console and no sample base is set", () => {
    const base = clientSampleBaseUrl(undefined, "");
    expect(base).toBe("http://localhost:4000");
    expect(base).not.toContain("/ui");
  });

  it("prefers the doc base, then the proxy base", () => {
    expect(
      clientSampleBaseUrl(
        {
          LITELLM_UI_API_DOC_BASE_URL: "https://docs.example.com/",
          PROXY_BASE_URL: "https://proxy.example.com",
        },
        "http://localhost:4000",
      ),
    ).toBe("https://docs.example.com");
    expect(clientSampleBaseUrl({ PROXY_BASE_URL: "https://proxy.example.com/" }, "")).toBe("https://proxy.example.com");
  });
});

/** 验证控制台地址与示例地址分离；前置不同覆盖组合，断言代理回退和覆盖优先级；纯函数无清理。 */
describe("resolveConsoleApiBase", () => {
  /** 前置缺失、null、空白地址；公网 HTTP 与 HTTPS 均可使用相对代理，不产生 localhost 地址。 */
  it.each([{}, { envBase: null, reportedBase: null }, { envBase: "  ", reportedBase: " " }])(
    "defaults to /gw for absent overrides: %j",
    (inputs) => {
      expect(resolveConsoleApiBase(inputs)).toBe("/gw");
    },
  );
  /** 前置独立部署或服务端覆盖；保留现有优先级并规范尾斜杠，纯函数无清理。 */
  it("preserves explicit gateway overrides", () => {
    expect(resolveConsoleApiBase({ envBase: " https://env.example.com/ " })).toBe("https://env.example.com");
    expect(
      resolveConsoleApiBase({ envBase: "https://env.example.com", reportedBase: "https://api.example.com/" }),
    ).toBe("https://api.example.com");
  });
});

/** 前置相对控制台代理；typed client 必须获得绝对 URL，缺少浏览器源时保留相对路径，纯函数无清理。 */
it("resolves relative gateway proxies against the page origin", () => {
  expect(
    resolveRequestUrl("/model/list?all=true", { registeredBase: "/gw/", pageOrigin: "http://xhub-http.test:3000/" }),
  ).toBe("http://xhub-http.test:3000/gw/model/list?all=true");
  expect(resolveRequestUrl("/model/list", { registeredBase: "/gw" })).toBe("/gw/model/list");
});
