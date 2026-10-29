import { describe, expect, test } from "bun:test";

import { minimapVisibleNodes } from "@/components/canvas/canvas-mini-map";
import { removeCanvasNodes } from "@/lib/canvas/canvas-project-domain";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

function node(id: string, patch: Partial<CanvasNodeData> = {}): CanvasNodeData {
    return {
        id,
        type: CanvasNodeType.Image,
        title: id,
        position: { x: 0, y: 0 },
        width: 320,
        height: 180,
        metadata: {},
        ...patch,
    };
}

describe("canvas minimap visibility", () => {
    test("excludes hidden batch children and managed-table backing nodes", () => {
        const nodes = [
            node("batch-root", { metadata: { isBatchRoot: true, imageBatchExpanded: false, batchChildIds: ["batch-child"] } }),
            node("batch-child", { metadata: { batchRootId: "batch-root" } }),
            node("asset-table", {
                type: CanvasNodeType.BatchTable,
                metadata: { batchTable: { operation: "creative", concurrency: 1, rows: [], manjuAssetTable: { version: 1, projectId: "p", sourceStage: "p2_assets" } } },
            }),
            node("asset-backing", { metadata: { manjuAssetTableId: "asset-table", manjuAssetRowId: "row-1" } }),
            node("visible"),
        ];

        expect(minimapVisibleNodes(nodes).map((item) => item.id)).toEqual(["batch-root", "asset-table", "visible"]);
    });

    test("deleting a managed table removes its hidden row nodes", () => {
        const table = node("asset-table", {
            type: CanvasNodeType.BatchTable,
            metadata: {
                batchTable: {
                    operation: "creative",
                    concurrency: 1,
                    rows: [{ id: "row-1", enabled: true, inputNodeIds: ["asset-backing"], outputNodeId: "asset-result", prompt: "prompt" }],
                    manjuAssetTable: { version: 1, projectId: "p", sourceStage: "p2_assets" },
                },
            },
        });
        const nodes = [
            table,
            node("asset-backing", { metadata: { manjuAssetTableId: table.id, manjuAssetRowId: "row-1" } }),
            node("asset-result", { metadata: { manjuAssetTableId: table.id, manjuAssetRowId: "row-1" } }),
            node("visible"),
        ];

        const result = removeCanvasNodes(nodes, new Set([table.id]));
        expect([...result.removedIds].sort()).toEqual(["asset-backing", "asset-result", "asset-table"]);
        expect(result.nodes.map((item) => item.id)).toEqual(["visible"]);
    });
});
