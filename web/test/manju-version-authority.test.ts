import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

test("active repository instructions name V7.4.26 as the only authority", () => {
    const rootAgents = readFileSync(resolve(import.meta.dir, "../../../AGENTS.md"), "utf8");
    const repoAgents = readFileSync(resolve(import.meta.dir, "../../AGENTS.md"), "utf8");

    for (const text of [rootAgents, repoAgents]) {
        expect(text).toContain("V7.4.26");
        expect(text).not.toContain("V7.4.20 原代码");
    }
});
