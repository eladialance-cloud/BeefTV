import { expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

test("active repository instructions name V7.4.26 as the only authority", () => {
    const rootAgentsPath = resolve(import.meta.dir, "../../../AGENTS.md");
    const repoAgents = readFileSync(resolve(import.meta.dir, "../../AGENTS.md"), "utf8");
    const rootAgents = existsSync(rootAgentsPath) ? readFileSync(rootAgentsPath, "utf8") : "";

    for (const text of [rootAgents, repoAgents].filter(Boolean)) {
        expect(text).toContain("V7.4.26");
        expect(text).not.toContain("V7.4.20 原代码");
    }
});
