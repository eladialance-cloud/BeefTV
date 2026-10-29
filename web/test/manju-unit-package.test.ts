import { describe, expect, test } from "bun:test";

import { buildManjuUnitPackage } from "@/lib/canvas/manju-unit-package";
import type { ManjuScriptRunDetail } from "@/services/api/manju";

function detail(): ManjuScriptRunDetail {
    return {
        run: {
            id: "run-1", projectId: "project-1", branch: "adaptation", mode: "STEPWISE",
            status: "COMPLETED", currentStage: "", lockRevision: 2, unitTitle: "第一章改编",
            sourceChapterIds: ["chapter-1"], sourceRangeLabel: "第一章 雨夜",
            reuseGlobalStages: false, createdBy: "owner", createdAt: "2026-10-21T00:00:00Z",
            updatedAt: "2026-10-21T00:00:00Z",
        },
        source: {
            id: "source-1", projectId: "project-1", runId: "run-1", branch: "adaptation",
            title: "雨夜旧宅", authorizationScope: "本项目改编", rangeLabel: "第一章 雨夜",
            contentHash: "abcdef1234567890", characterCount: 1200, extractionStatus: "ready",
            createdBy: "owner", createdAt: "2026-10-21T00:00:00Z", updatedAt: "2026-10-21T00:00:00Z",
        },
        stages: [
            { id: "stage-1", key: "script_draft", title: "正式剧本", position: 5, status: "LOCKED", version: 1, artifactId: "artifact-1", updatedAt: "2026-10-21T00:00:00Z" },
            { id: "stage-2", key: "roundtable_review", title: "五席圆桌审核", position: 6, status: "LOCKED", version: 1, artifactId: "artifact-2", updatedAt: "2026-10-21T00:00:00Z" },
        ],
        artifacts: [
            { id: "artifact-1", stageId: "stage-1", kind: "SCRIPT_DRAFT.md", version: 1, content: "# 第一章\n林澈推门。", contentHash: "a", createdAt: "2026-10-21T00:00:00Z" },
            { id: "artifact-2", stageId: "stage-2", kind: "ROUNDTABLE_REVIEW.md", version: 1, content: "# 审核\n通过。", contentHash: "b", createdAt: "2026-10-21T00:00:00Z" },
        ],
    };
}

describe("漫剧章节交付包", () => {
    test("保留改编单元、原著范围和全部已保存阶段工件", () => {
        const result = buildManjuUnitPackage(detail());
        expect(result?.fileName).toContain("第一章改编");
        expect(result?.markdown).toContain("# 第一章改编");
        expect(result?.markdown).toContain("第一章 雨夜");
        expect(result?.markdown).toContain("# 第一章\n林澈推门。");
        expect(result?.markdown).toContain("# 审核\n通过。");
        expect(result?.markdown.indexOf("正式剧本")).toBeLessThan(result?.markdown.indexOf("五席圆桌审核"));
    });
});
