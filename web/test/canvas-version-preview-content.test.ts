import { describe, expect, test } from "bun:test";

import { canvasVersionPreviewContentKind } from "@/lib/canvas/canvas-version-preview-content";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

describe("canvas version preview content", () => {
    test("renders managed Manju tables with the real read-only table component", () => {
        const assetTable = {
            id: "asset-table",
            type: CanvasNodeType.BatchTable,
            title: "数字资产表",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    rows: [],
                    manjuAssetTable: { version: 1 as const, projectId: "project", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const storyboardTable = {
            ...assetTable,
            id: "storyboard-table",
            metadata: {
                batchTable: {
                    ...assetTable.metadata.batchTable,
                    manjuAssetTable: undefined,
                    manjuStoryboardTable: { version: 1 as const, projectId: "project", sourceStage: "p3_images" as const },
                },
            },
        } satisfies CanvasNodeData;

        expect(canvasVersionPreviewContentKind(assetTable)).toBe("manju-table");
        expect(canvasVersionPreviewContentKind(storyboardTable)).toBe("manju-table");
    });

    test("keeps non-Manju batch tables distinct from ordinary content", () => {
        const table = {
            id: "table",
            type: CanvasNodeType.BatchTable,
            title: "批量表",
            position: { x: 0, y: 0 },
            width: 900,
            height: 500,
            metadata: { batchTable: { operation: "creative" as const, concurrency: 1, rows: [] } },
        } satisfies CanvasNodeData;

        expect(canvasVersionPreviewContentKind(table)).toBe("batch-table");
    });
});
