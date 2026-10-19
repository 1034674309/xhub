import { describe, it, expect } from "vitest";
import moment from "moment";
import "moment/locale/zh-cn";
import sharp from "sharp";
import { parse } from "smol-toml";
import { SourceMapConsumer, SourceMapGenerator } from "source-map-js";

describe("依赖升级兼容性", () => {
  /** 验证中英文日期、严格非法日期和月末边界；前置真实 Moment，使用实例语言不修改全局状态，无资源需清理。 */
  it("preserves English and Chinese date formatting and rejects invalid dates", () => {
    const value = moment.utc("2026-10-19T09:00:00Z", moment.ISO_8601, true);
    expect(value.clone().locale("en").format("MMMM D, YYYY")).toBe("October 19, 2026");
    expect(value.clone().locale("zh-cn").format("LL")).toBe("2026年10月19日");
    expect(moment.utc("2026-02-30", "YYYY-MM-DD", true).isValid()).toBe(false);
    expect(moment.utc("2026-01-31").add(1, "month").format("YYYY-MM-DD")).toBe("2026-02-28");
  });

  /** 验证真实原生图片库的 PNG/JPEG/WebP 编解码、尺寸边界及非法数据拒绝；输入内存红色图片，输出仅在内存无需清理。 */
  it("round trips supported image formats and rejects invalid image buffers", async () => {
    const png = await sharp({ create: { width: 8, height: 6, channels: 3, background: "#ff0000" } }).png().toBuffer();
    for (const format of ["png", "jpeg", "webp"] as const) {
      const encoded = await sharp(png).resize(4, 3).toFormat(format).toBuffer();
      const metadata = await sharp(encoded).metadata();
      expect(metadata.format).toBe(format);
      expect([metadata.width, metadata.height]).toEqual([4, 3]);
      const { data } = await sharp(encoded).removeAlpha().raw().toBuffer({ resolveWithObject: true });
      expect(data[0]).toBeGreaterThan(240);
      expect(data[1]).toBeLessThan(15);
    }
    await expect(sharp(Buffer.from("not an image")).metadata()).rejects.toThrow();
    expect(() => sharp(png).resize(0, 3)).toThrow();
  });

  /** 验证 TOML 的表、数组和数值兼容，以及新版拒绝非法指数符号；前置真实解析器，纯内存断言无需清理。 */
  it("parses valid nested TOML while rejecting malformed exponents and duplicate keys", () => {
    expect(parse('title = "Gateway"\n[provider]\nmodels = ["gpt", "grok"]\nlimit = 1e2\n')).toEqual({
      title: "Gateway", provider: { models: ["gpt", "grok"], limit: 100 },
    });
    expect(parse("")).toEqual({});
    expect(() => parse("value = 1e+-2")).toThrow();
    expect(() => parse("key = 1\nkey = 2")).toThrow();
  });

  /** 验证构建工具源码映射的编码/解码、无映射边界与非法坐标；前置真实库，内存映射无需清理。 */
  it("maps generated code back to its source and validates coordinate boundaries", () => {
    const map = new SourceMapGenerator({ file: "output.js" });
    map.addMapping({ generated: { line: 1, column: 0 }, original: { line: 7, column: 2 }, source: "input.ts", name: "gateway" });
    const consumer = new SourceMapConsumer(map.toString());
    expect(consumer.originalPositionFor({ line: 1, column: 0 })).toMatchObject({ source: "input.ts", line: 7, column: 2, name: "gateway" });
    expect(consumer.originalPositionFor({ line: 2, column: 0 }).source).toBeNull();
    expect(() => consumer.originalPositionFor({ line: 0, column: 0 })).toThrow();
  });
});
