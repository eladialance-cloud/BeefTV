import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(resolve(here, "..", "app.js"), "utf8");

test("commercial ledger exposes user, model, token, credit, and task detail columns", () => {
    assert.match(source, /label: "积分明细", section: "商业化"/u);
    for (const header of ["用户名", "时间", "类型", "模型", "Token 名称", "积分变动", "任务详情"]) {
        assert.match(source, new RegExp(`<th>${header}</th>|${header}`, "u"));
    }
    assert.match(source, /name="model"/u);
    assert.match(source, /name="token"/u);
    assert.match(source, /appendFilter\(params, "model", state\.adminLedgerModel\)/u);
    assert.match(source, /appendFilter\(params, "token", state\.adminLedgerToken\)/u);
});

test("ledger task details use safe task metadata without secrets or prompts", () => {
    const marker = source.indexOf("function openLedgerDetailModal");
    assert.notEqual(marker, -1);
    assert.match(source, /任务详情/u);
    assert.match(source, /上游请求 ID/u);
    const modalSource = source.slice(marker, marker + 5000);
    assert.doesNotMatch(modalSource, /apiKey|secretKey|prompt\s*[:=]/iu);
});
