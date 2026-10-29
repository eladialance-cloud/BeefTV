import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import {
    manjuP5CanvasPath,
    primaryManjuProjectCanvas,
} from "@/lib/canvas/manju-canvas-entry";
import type { ManjuCanvasImport } from "@/services/api/manju";

const imports: ManjuCanvasImport[] = [{
    id: "import-p4",
    projectId: "manju-1",
    stage: "p4_videos",
    productionRunId: "run-1",
    canvasId: "canvas-1",
    packageHash: "hash",
    taskCount: 4,
    status: "READY",
    createdBy: "owner",
    createdAt: "2026-10-25T00:00:00Z",
    updatedAt: "2026-10-25T00:00:00Z",
}];

test("P5 canvas entry has no target before a project canvas is created", () => {
    expect(primaryManjuProjectCanvas([])).toBeUndefined();
});

test("P5 canvas entry opens the existing project canvas", () => {
    expect(primaryManjuProjectCanvas(imports)?.canvasId).toBe("canvas-1");
    expect(manjuP5CanvasPath("manju 1", "canvas/1"))
        .toBe("/canvas/canvas%2F1?from=manju&manjuProjectId=manju%201&stage=p5");
});

test("P5 renders a canvas entry below the stage and gates it by canvas existence", () => {
    const page = readFileSync(resolve(import.meta.dir, "../src/pages/manju/index.tsx"), "utf8");

    expect(page).toContain('className="manju-production-canvas-entry"');
    expect(page).toContain('stage.key === "qc_handoff"');
    expect(page).toContain("请先在 P2/P3/P4 阶段创建项目画布");
    expect(page).toContain("进入画布");
    expect(page).toContain("disabled={!projectCanvasId");
});
