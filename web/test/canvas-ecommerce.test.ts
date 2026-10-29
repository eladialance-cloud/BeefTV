import { describe, expect, test } from "bun:test";

import { batchReferenceHandleId } from "../src/lib/canvas/canvas-batch-table";
import { createEcommercePipeline } from "../src/lib/canvas/canvas-ecommerce";
import { CanvasNodeType } from "../src/types/canvas";

describe("canvas ecommerce pipeline", () => {
    test("creates a connected product-to-output workflow", () => {
        const pipeline = createEcommercePipeline({ x: 1200, y: 800 });
        const nodeById = new Map(pipeline.nodes.map((node) => [node.id, node]));

        expect(pipeline.nodes).toHaveLength(10);
        expect(new Set(pipeline.nodes.map((node) => node.id)).size).toBe(pipeline.nodes.length);
        expect(nodeById.get(pipeline.productBriefNodeId)?.type).toBe(CanvasNodeType.Text);
        expect(nodeById.get(pipeline.productImageNodeId)?.metadata?.workflowKind).toBe("ecommerce_product");
        expect(nodeById.get(pipeline.mainImageNodeId)?.metadata?.workflowKind).toBe("ecommerce_main");
        expect(nodeById.get(pipeline.detailImageNodeId)?.metadata?.workflowKind).toBe("ecommerce_detail");
        expect(nodeById.get(pipeline.sceneImageNodeId)?.metadata?.workflowKind).toBe("ecommerce_scene");
        expect(nodeById.get(pipeline.socialImageNodeId)?.metadata?.workflowKind).toBe("ecommerce_social");
        expect(nodeById.get(pipeline.videoNodeId)?.metadata).toMatchObject({
            workflowKind: "ecommerce_video",
            videoStartFrameNodeId: pipeline.mainImageNodeId,
            seconds: "5",
        });

        expect(pipeline.connections.every((connection) => nodeById.has(connection.fromNodeId) && nodeById.has(connection.toNodeId))).toBe(true);
        expect(pipeline.connections.some((connection) => connection.fromNodeId === pipeline.mainImageNodeId && connection.toNodeId === pipeline.videoNodeId)).toBe(true);
        expect(pipeline.connections.some((connection) => connection.fromNodeId === pipeline.productImageNodeId && connection.toNodeId === pipeline.batchTableNodeId && connection.toHandleId === batchReferenceHandleId("product"))).toBe(true);
        expect(pipeline.connections.some((connection) => connection.fromNodeId === pipeline.productImageNodeId && connection.toNodeId === pipeline.detailImageNodeId)).toBe(true);
    });

    test("seeds the batch table with product and style references", () => {
        const pipeline = createEcommercePipeline({ x: 0, y: 0 });
        const table = pipeline.nodes.find((node) => node.id === pipeline.batchTableNodeId)?.metadata?.batchTable;

        expect(table?.operation).toBe("creative");
        expect(table?.concurrency).toBe(5);
        expect(table?.referenceColumns).toEqual([
            { id: "product", label: "商品参考" },
            { id: "style", label: "风格参考" },
        ]);
        expect(table?.rows).toHaveLength(4);
        expect(table?.rows.every((row) => row.enabled && row.prompt.trim().length > 0)).toBe(true);
        expect(table?.rows.every((row) => row.inputNodeIds[0] === pipeline.productImageNodeId)).toBe(true);
        expect(table?.rows.every((row) => row.inputNodeIds[1] === pipeline.styleReferenceNodeId)).toBe(true);
    });
});
