import assert from "node:assert/strict";
import { describe, test } from "node:test";

// node:test 不提供 expect；这里给最小断言包装，保持用例可读。
function expect(actual) {
    return {
        toBe: (wanted) => assert.equal(actual, wanted),
        toMatch: (pattern) => assert.match(actual, pattern),
        toContain: (wanted) => assert.ok(String(actual).includes(wanted), `缺少 ${wanted}`),
        not: {
            toBe: (wanted) => assert.notEqual(actual, wanted),
            toMatch: (pattern) => assert.doesNotMatch(actual, pattern),
        },
    };
}

import { buildAdjustIdempotencyKey, escapeHtml, formatPoints, formatTime, kindLabel, resolveApiBase, stageLabel } from "../format.js";

describe("escapeHtml", () => {
    test("渲染用户数据前转义 HTML 元字符", () => {
        expect(escapeHtml('<img src=x onerror="alert(1)">')).toBe("&lt;img src=x onerror=&quot;alert(1)&quot;&gt;");
        expect(escapeHtml("Tom & Jerry's")).toBe("Tom &amp; Jerry&#39;s");
    });

    test("null/undefined 渲染成空串而不是字面量", () => {
        expect(escapeHtml(null)).toBe("");
        expect(escapeHtml(undefined)).toBe("");
    });
});

describe("formatTime", () => {
    test("ISO 时间渲染成分钟级本地时间", () => {
        const rendered = formatTime("2026-10-16T08:05:00.000Z");
        expect(rendered).toMatch(/^2026-10-16 \d{2}:\d{2}$/u);
    });

    test("空值和非法值都退回占位符", () => {
        expect(formatTime("")).toBe("-");
        expect(formatTime("not-a-date")).toBe("-");
    });
});

describe("formatPoints", () => {
    test("整数按千位分隔", () => {
        expect(formatPoints(1234567)).toBe("1,234,567");
        expect(formatPoints(0)).toBe("0");
    });

    test("空值按 0 处理", () => {
        expect(formatPoints(null)).toBe("0");
        expect(formatPoints(undefined)).toBe("0");
    });
});

describe("ledger labels", () => {
    test("已知类型翻译成中文", () => {
        expect(kindLabel("freeze")).toBe("冻结");
        expect(kindLabel("adjust")).toBe("人工调整");
        expect(stageLabel("reserve")).toBe("预冻结");
        expect(stageLabel("settle")).toBe("结算");
    });

    test("未知类型原样透出，不吞掉后端新枚举", () => {
        expect(kindLabel("refund")).toBe("refund");
        expect(stageLabel("reconcile")).toBe("reconcile");
    });
});

describe("resolveApiBase", () => {
    test("内联注入优先于本地存储", () => {
        expect(resolveApiBase({ injected: "https://api.example.com/api", stored: "http://localhost:9000/api" })).toBe("https://api.example.com/api");
    });

    test("没有配置时走同源 /api", () => {
        expect(resolveApiBase({})).toBe("/api");
        expect(resolveApiBase()).toBe("/api");
    });

    test("去掉结尾斜杠，避免拼出双斜杠路径", () => {
        expect(resolveApiBase({ stored: "https://api.example.com/api///" })).toBe("https://api.example.com/api");
    });
});

describe("buildAdjustIdempotencyKey", () => {
    test("同一用户同一时间戳加随机数仍是不同键", () => {
        const first = buildAdjustIdempotencyKey("u1", 1000, "aaa");
        const second = buildAdjustIdempotencyKey("u1", 1000, "bbb");
        expect(first).toBe("adjust:u1:1000:aaa");
        expect(first).not.toBe(second);
    });
});
