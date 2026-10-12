import { describe, expect, it } from "vitest";
import {
  monitorOptions,
  parseMonitorData,
  chartRows,
  localDate,
  seriesKey,
  validWindow,
  type MonitorData,
  type MonitorRow,
} from "./data";
/** 模型监控纯函数测试；输入本地日期和隔离数据，验证边界、身份冲突及无样本，不写外部数据。 */
describe("model monitor data", () => {
  /** 前置未知接口 JSON，验证正常空目录与缺字段、错误类型、无效身份被拒绝；无持久化。 */
  it("rejects malformed responses before rendering", () => {
    const data = { options: [], models: [], trend: [], summary: {}, start_date: "2026-01-01", end_date: "2026-01-02" };
    expect(parseMonitorData(data)).toBe(data);
    for (const value of [
      null,
      {},
      { ...data, options: undefined },
      { ...data, options: {} },
      { ...data, options: [null] },
      { ...data, options: [{ supplier: 3, model: "x" }] },
    ]) {
      expect(() => parseMonitorData(value)).toThrow("Invalid model monitor response");
    }
  });
  /** 前置配置与历史身份，验证无调用选项、删除历史和去重；输入只读，无清理需求。 */
  it("merges configured identities with authorized history", () => {
    const options = [
      { supplier: "b", model: "unused" },
      { supplier: "a", model: "same" },
    ];
    const models = [
      { supplier: "a", model: "same" },
      { supplier: "deleted", model: "historic" },
    ] as MonitorRow[];
    expect(monitorOptions({ options, models } as MonitorData)).toEqual([options[1], options[0], models[1]]);
    expect(monitorOptions({ options: [], models: [] } as unknown as MonitorData)).toEqual([]);
  });
  it("validates date bounds and local date", () => {
    expect(validWindow("2026-01-01", "2026-03-31")).toBe(true);
    expect(validWindow("2026-01-01", "2026-04-01")).toBe(false);
    expect(validWindow("", "invalid")).toBe(false);
    expect(validWindow("2026-02-01", "2026-01-01")).toBe(false);
    expect(localDate(new Date(2026, 0, 2))).toBe("2026-01-02");
  });
  it("separates suppliers and keeps missing samples as gaps", () => {
    const a = { supplier: "a/b", model: "c", day: "2026-01-02", avg_tps: 15, total_tokens: 30 } as MonitorRow;
    const b = { supplier: "a", model: "b/c", day: "2026-01-02", avg_tps: null, total_tokens: 20 } as MonitorRow;
    expect(seriesKey(a)).not.toBe(seriesKey(b));
    const data = { start_date: "2026-01-01", end_date: "2026-01-03", trend: [a, b] } as MonitorData;
    const rows = chartRows(data, "avg_tps", [seriesKey(a), seriesKey(b)]);
    expect(rows).toHaveLength(3);
    expect(rows[0][seriesKey(a)]).toBeNull();
    expect(rows[1][seriesKey(a)]).toBe(15);
    expect(rows[1][seriesKey(b)]).toBeNull();
    expect(chartRows(data, "total_tokens", [seriesKey(a)])[0][seriesKey(a)]).toBe(0);
  });
});
