import { describe, expect, test } from "bun:test";

import { reconcileRecoveredCanvasNode } from "@/lib/canvas/canvas-recovery-projection";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";
import type { CanvasProject } from "@/stores/canvas/use-canvas-store";

function node(id: string, patch: Partial<CanvasNodeData> = {}): CanvasNodeData {
    return {
        id,
        type: CanvasNodeType.Video,
        title: id,
        position: { x: 0, y: 0 },
        width: 320,
        height: 180,
        metadata: {},
        ...patch,
    };
}

function project(nodes: CanvasNodeData[]): CanvasProject {
    return {
        id: "canvas-1",
        revision: 8,
        title: "画布",
        createdAt: "2026-01-01",
        updatedAt: "2026-01-01",
        nodes,
        connections: [{ id: "edge-1", fromNodeId: "stale", toNodeId: "target" }],
        chatSessions: [],
        activeChatId: null,
        backgroundMode: "grid",
        showImageInfo: false,
        viewport: { x: 0, y: 0, k: 1 },
        directorScenes: [],
        timeline: {
            version: 2,
            durationMs: 1000,
            tracks: [],
            clips: [{ id: "clip-1", kind: "video", nodeId: "stale", trackId: "video-1", startMs: 0, durationMs: 1000 }],
        },
    };
}

describe("reconcileRecoveredCanvasNode", () => {
    test("replaces the stale error overlay with the recovered node at its visual position", () => {
        const stale = node("stale", {
            title: "原生成框",
            position: { x: 640, y: 220 },
            width: 480,
            height: 270,
            metadata: {
                taskId: "task-1",
                status: "error",
                prompt: "一只白猫",
                composerContent: "一只白猫",
                model: "image-model",
                size: "1:1",
                generationErrorCode: "canvas_conflict",
            },
        });
        const recovered = node("recovered", { title: "恢复视频", position: { x: 120, y: 120 }, metadata: { taskId: "task-1", status: "success" } });
        const target = node("target", { type: CanvasNodeType.Image });
        const reconciled = reconcileRecoveredCanvasNode(project([stale, recovered, target]), "stale", "recovered");

        expect(reconciled.nodes.map((item) => item.id)).toEqual(["recovered", "target"]);
        expect(reconciled.nodes[0]).toMatchObject({
            id: "recovered",
            title: "原生成框",
            position: { x: 640, y: 220 },
            width: 480,
            height: 270,
            metadata: {
                taskId: "task-1",
                status: "success",
                prompt: "一只白猫",
                composerContent: "一只白猫",
                model: "image-model",
                size: "1:1",
            },
        });
        expect(reconciled.nodes[0]?.metadata?.generationErrorCode).toBeUndefined();
        expect(reconciled.connections[0]).toMatchObject({ fromNodeId: "recovered", toNodeId: "target" });
        expect(reconciled.timeline?.clips[0]?.nodeId).toBe("recovered");
    });

    test("leaves the project unchanged when recovery does not change the node identity", () => {
        const original = project([node("node-1")]);
        expect(reconcileRecoveredCanvasNode(original, "node-1", "node-1")).toBe(original);
    });

    test("fills composer metadata from the task when the stale node is already gone", () => {
        const recovered = node("recovered", { metadata: { taskId: "task-1", status: "success" } });
        const reconciled = reconcileRecoveredCanvasNode(project([recovered]), "missing", "recovered", {
            prompt: "任务提示词",
            composerContent: "任务提示词",
            model: "task-model",
        });
        expect(reconciled.nodes[0]?.metadata).toMatchObject({
            prompt: "任务提示词",
            composerContent: "任务提示词",
            model: "task-model",
            status: "success",
        });
    });

    test("remaps asset-table rows and generation batches to the recovered node", () => {
        const stale = node("stale", { metadata: { taskId: "task-1", status: "error" } });
        const recovered = node("recovered", { metadata: { taskId: "task-1", status: "success", content: "/api/resources/generated/file" } });
        const table = node("asset-table", {
            type: CanvasNodeType.BatchTable,
            metadata: {
                batchTable: {
                    operation: "creative",
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "asset-row",
                        enabled: true,
                        inputNodeIds: ["stale"],
                        outputNodeId: "stale",
                        prompt: "资产生成提示词",
                        manjuAsset: { taskId: "task-1", code: "CHR-01", kind: "character", name: "林澈" },
                    }],
                    manjuAssetTable: { version: 1, projectId: "project-1", sourceStage: "p2_assets" },
                },
                generationBatches: [{
                    id: "batch-1",
                    projectId: "project-1",
                    sourceNodeId: "asset-table",
                    mode: "batch_image",
                    status: "failed",
                    items: [{ id: "item-1", rowId: "asset-row", nodeId: "stale", status: "failed", retryCount: 0 }],
                    createdAt: "2026-01-01",
                    updatedAt: "2026-01-01",
                }],
            },
        });

        const reconciled = reconcileRecoveredCanvasNode(project([stale, recovered, table]), "stale", "recovered");
        const repairedTable = reconciled.nodes.find((item) => item.id === "asset-table");
        expect(repairedTable?.metadata?.batchTable?.rows[0]).toMatchObject({
            inputNodeIds: ["recovered"],
            outputNodeId: "recovered",
        });
        expect(repairedTable?.metadata?.generationBatches?.[0]?.items[0]?.nodeId).toBe("recovered");
    });
});
