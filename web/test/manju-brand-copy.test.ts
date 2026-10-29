import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const page = () => readFileSync(resolve(import.meta.dir, "../src/pages/manju/index.tsx"), "utf8");

test("user-facing manju studio does not expose the upstream author brand", () => {
    expect(page()).not.toContain("漫剧老李");
});

test("manju service-task messages use VisiDeep AI branding", () => {
    const source = page();
    expect(source).not.toContain("BeefTV");
    expect(source).toContain("VisiDeep AI 文本任务已提交");
});
