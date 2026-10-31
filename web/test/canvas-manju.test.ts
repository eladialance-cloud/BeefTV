import { describe, expect, test } from "bun:test";

import { composeManjuImageExecutionPrompt, composeManjuVideoExecutionPrompt, createManjuCanvasPackage, manjuAssetNameFromCode, manjuCanvasViewportForNodes, manjuGenerationTaskMetadata, mergeManjuCanvasPackage, normalizeManjuCanvasLayout, normalizeManjuVisualReferenceCodes, removeObsoleteManjuReferenceNodes, repairManjuAssetNames, repairManjuAssetTablePrompts, repairRecoveredManagedTableLinks } from "../src/lib/canvas/canvas-manju";
import {
    cleanupManjuInternalConnections,
    appendManjuAssetRow,
    appendManjuStoryboardRow,
    buildManjuAssetRowOutputMediaMap,
    buildManjuStoryboardAssetReferenceMap,
    manjuAssetInputHandleId,
    manjuCanvasAssetRevision,
    manjuAssetOutputHandleId,
    preferredManjuAssetKind,
    resolveManjuAssetConnectionSource,
    resolveManjuInternalReferenceNodes,
    resolveManjuStoryboardAssetReferences,
} from "../src/lib/canvas/canvas-manju-asset-table";
import { patchManjuTableGenerationSettings, resolveBatchGenerationModels, resolveCanvasGenerationSettings } from "../src/lib/canvas/canvas-manju-generation-settings";
import { buildCanvasNodeMentionReferenceMap, buildNodeMentionReferences, getGenerationResourceNodes } from "../src/lib/canvas/canvas-resource-references";
import type { ManjuCanvasPackage } from "../src/services/api/manju";
import { CanvasNodeType, type CanvasConnection, type CanvasNodeData } from "../src/types/canvas";

const P3_PACKAGE: ManjuCanvasPackage = {
    version: 1,
    projectId: "project-1",
    stage: "p3_images",
    productionRunId: "run-p3",
    sourceArtifact: "DIRECTOR_STORYBOARD.md",
    title: "P3 故事板与剧情分镜格",
    hash: "package-hash",
    style: {
        cardId: "sd-style-007",
        cardName: "路阳",
        styleBase: "live-action",
        aspectRatio: "9:16",
        promptParameters: "冷青灰配烛火暖橙 · 低机位贴身跟拍",
    },
    tasks: [
        {
            id: "p3-storyboard-seg-1",
            kind: "image",
            code: "段1",
            title: "故事板图 · 段1",
            prompt: "故事板图提示词",
            category: "material",
            tags: ["P3", "故事板图"],
            aspectRatio: "16:9",
            referenceCodes: ["CHR-林澈"],
            sourceKind: "DIRECTOR_STORYBOARD.md",
            group: "storyboard",
        },
        {
            id: "p3-plot-scene-1",
            kind: "image",
            code: "场1",
            title: "剧情分镜格 · 场1",
            prompt: "剧情分镜格提示词",
            category: "material",
            tags: ["P3", "剧情分镜格"],
            aspectRatio: "16:9",
            referenceCodes: ["CHR-林澈"],
            sourceKind: "DIRECTOR_STORYBOARD.md",
            group: "plot-grid",
        },
    ],
};

describe("canvas manju package", () => {
    test("keeps multiple P2 generation variants that share one logical asset code", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-scene-variants",
            stage: "p2_assets",
            assetRunId: "run-scene-variants",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "scene-variants",
            tasks: [
                {
                    id: "p2-scene-master",
                    kind: "image",
                    code: "图1",
                    assetCode: "SCN-旧宅",
                    title: "旧宅（场景母版）",
                    prompt: "旧宅场景母版完整提示词",
                    category: "environment",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
                {
                    id: "p2-scene-grid",
                    kind: "image",
                    code: "图1-2",
                    assetCode: "SCN-旧宅",
                    title: "旧宅（场景九宫格）",
                    prompt: "旧宅场景九宫格完整提示词",
                    category: "environment",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
            ],
        };

        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const rows = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows || [];

        expect(rows.map((row) => [
            row.manjuAsset?.code,
            row.manjuAsset?.taskCode,
            row.manjuAsset?.taskId,
        ])).toEqual([
            ["SCN-旧宅", "图1", "p2-scene-master"],
            ["SCN-旧宅", "图1-2", "p2-scene-grid"],
        ]);
        expect(rows.map((row) => row.prompt)).toEqual([
            "旧宅场景母版完整提示词",
            "旧宅场景九宫格完整提示词",
        ]);
    });

    test("appends manual asset rows with the next code for each category", () => {
        let table = {
            operation: "creative" as const,
            concurrency: 5,
            referenceColumns: [],
            rows: [
                { id: "chr-row", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "p2-chr", code: "CHR-01", kind: "character" as const, name: "角色一" } },
                { id: "scn-row", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "p2-scn", code: "SCN-01", kind: "scene" as const, name: "场景一" } },
                { id: "prp-row", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "p2-prp", code: "PRP-01", kind: "prop" as const, name: "道具一" } },
            ],
            manjuAssetTable: { version: 1 as const, projectId: "project-manual-assets", sourceStage: "p2_assets" as const },
        };

        table = appendManjuAssetRow(table, "character");
        table = appendManjuAssetRow(table, "scene");
        table = appendManjuAssetRow(table, "prop");

        expect(table.rows.slice(-3).map((row) => row.manjuAsset?.code)).toEqual(["CHR-02", "SCN-02", "PRP-02"]);
        expect(table.rows.slice(-3).every((row) => row.manjuAsset?.taskId.startsWith("manual:asset:"))).toBe(true);
        expect(table.rows.slice(-3).every((row) => row.enabled && row.prompt === "")).toBe(true);
    });

    test("appends a manual storyboard after the highest existing segment number", () => {
        const table = {
            operation: "creative" as const,
            concurrency: 5,
            referenceColumns: [],
            rows: [
                { id: "segment-2", enabled: true, inputNodeIds: [], prompt: "", manjuStoryboard: { taskId: "p3-2", code: "段2", title: "第二段", group: "storyboard" as const, referenceCodes: [] } },
                { id: "segment-10", enabled: true, inputNodeIds: [], prompt: "", manjuStoryboard: { taskId: "p3-10", code: "段10", title: "第十段", group: "storyboard" as const, referenceCodes: [] } },
            ],
            manjuStoryboardTable: { version: 1 as const, projectId: "project-manual-storyboards", sourceStage: "p3_images" as const },
        };

        const next = appendManjuStoryboardRow(table);
        const row = next.rows.at(-1);

        expect(row?.manjuStoryboard?.code).toBe("段11");
        expect(row?.manjuStoryboard?.title).toBe("自定义故事板11");
        expect(row?.manjuStoryboard?.taskId.startsWith("manual:storyboard:")).toBe(true);
        expect(next.rows).toHaveLength(3);
    });

    test("carries managed table identity into backend generation task metadata", () => {
        expect(manjuGenerationTaskMetadata({
            manjuProjectId: "project-1",
            manjuStage: "p2_assets",
            manjuAssetTableId: "asset-table",
            manjuAssetRowId: "asset-row",
            manjuAssetCode: "CHR-01",
            manjuTaskId: "p2-task",
            manjuTaskCode: "图1",
        })).toEqual({
            manjuProjectId: "project-1",
            manjuStage: "p2_assets",
            manjuAssetTableId: "asset-table",
            manjuAssetRowId: "asset-row",
            manjuAssetCode: "CHR-01",
            manjuTaskId: "p2-task",
            manjuTaskCode: "图1",
        });
    });

    test("asset names never fall back to the pending-reference label", () => {
        expect(manjuAssetNameFromCode("CHR-林澈")).toBe("林澈");
        expect(manjuAssetNameFromCode("SCN-老宅")).toBe("老宅");
        expect(manjuAssetNameFromCode("PRP-旧信")).toBe("旧信");
        expect(manjuAssetNameFromCode("CNH-01")).toBe("01");
    });

    test("repairs legacy pending-reference names in existing asset tables", () => {
        const nodes = [{
            id: "asset-table",
            type: CanvasNodeType.BatchTable,
            title: "漫剧数字资产表",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative",
                    concurrency: 1,
                    rows: [{
                        id: "row-chr",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuAsset: {
                            taskId: "task-chr",
                            code: "CHR-林澈",
                            kind: "character",
                            name: "CHR-林澈 · 待补参考",
                        },
                    }],
                    referenceColumns: [],
                    manjuAssetTable: { version: 1 },
                },
            },
        }] as CanvasNodeData[];
        const repaired = repairManjuAssetNames(nodes);
        expect(repaired[0]?.metadata?.batchTable?.rows[0]?.manjuAsset?.name).toBe("林澈");
    });

    test("resolves generation settings from row, category, table, then canvas defaults", () => {
        const row = {
            id: "row-character",
            enabled: true,
            inputNodeIds: [],
            prompt: "",
            manjuGeneration: { size: "1:1" },
            manjuAsset: { taskId: "task", code: "CHR-01", kind: "character" as const, name: "林澈" },
        };
        const settings = resolveCanvasGenerationSettings(
            {
                operation: "creative",
                concurrency: 5,
                rows: [row],
                manjuGenerationDefaults: { model: "table-model", size: "16:9", quality: "1k", count: 1 },
                manjuCategoryGeneration: {
                    character: { model: "character-model", size: "3:4", quality: "2k", count: 2 },
                },
            },
            row,
            {
                imageModel: "canvas-model",
                model: "canvas-model",
                size: "9:16",
                quality: "auto",
                count: "1",
                canvasImageCount: "1",
            } as never,
        );

        expect(settings).toEqual({ model: "character-model", size: "1:1", quality: "2k", count: 2 });
    });

    test("persists model selection in the active table, category, and row scopes", () => {
        const row = {
            id: "row-character",
            enabled: true,
            inputNodeIds: [],
            prompt: "角色提示词",
            manjuAsset: { taskId: "task", code: "CHR-01", kind: "character" as const, name: "林澈" },
        };
        let table = {
            operation: "creative" as const,
            concurrency: 1,
            rows: [row],
            manjuGenerationDefaults: { model: "table-model" },
            manjuCategoryGeneration: { character: { model: "category-model" } },
        };
        table = patchManjuTableGenerationSettings(table, { kind: "character" }, { model: "saved-category-model" });
        expect(table.manjuCategoryGeneration?.character?.model).toBe("saved-category-model");
        expect(resolveBatchGenerationModels(table, [row], {
            imageModel: "fallback-model",
            model: "fallback-model",
            size: "16:9",
            quality: "auto",
            count: "1",
            canvasImageCount: "1",
        } as never)).toEqual(["saved-category-model"]);

        table = patchManjuTableGenerationSettings(table, { rowId: row.id }, { model: "saved-row-model" });
        expect(table.rows[0]?.manjuGeneration?.model).toBe("saved-row-model");
        expect(resolveBatchGenerationModels(table, table.rows, {
            imageModel: "fallback-model",
            model: "fallback-model",
            size: "16:9",
            quality: "auto",
            count: "1",
            canvasImageCount: "1",
        } as never)).toEqual(["saved-row-model"]);
        expect(resolveCanvasGenerationSettings(table, table.rows[0], {
            imageModel: "fallback-model",
            model: "fallback-model",
            size: "16:9",
            quality: "auto",
            count: "1",
            canvasImageCount: "1",
        } as never).model).toBe("saved-row-model");
    });

    test("maps P3 tasks to canvas image boxes with internal P2 references", () => {
        const pipeline = createManjuCanvasPackage(
            P3_PACKAGE,
            [
                {
                    code: "CHR-林澈",
                    title: "P2 参考 · 林澈",
                    content: "/api/resources/res-character/file",
                    storageKey: "resource:res-character",
                    assetId: "generation_character",
                    taskId: "p2-character-task",
                    prompt: "P2 林澈角色四格提示词",
                },
            ],
            { x: 1000, y: 800 },
        );
        const storyboardTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        const assetTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const row = assetTable?.metadata?.batchTable?.rows.find((item) => item.manjuAsset?.code === "CHR-林澈");
        const referenceNode = pipeline.nodes.find((node) => node.id === row?.outputNodeId);
        const storyboardRows = storyboardTable?.metadata?.batchTable?.rows || [];

        expect(storyboardRows).toHaveLength(2);
        expect(assetTable?.title).toBe("漫剧数字资产表");
        expect(storyboardTable?.title).toBe("漫剧故事板表");
        expect(row?.manjuAsset).toMatchObject({ code: "CHR-林澈", kind: "character" });
        expect(row?.prompt).toBe("P2 林澈角色四格提示词");
        expect(storyboardRows.map((item) => item.manjuStoryboard?.taskId)).toEqual(["p3-storyboard-seg-1", "p3-plot-scene-1"]);
        expect(referenceNode?.metadata).toMatchObject({
            content: "/api/resources/res-character/file",
            storageKey: "resource:res-character",
            assetId: "generation_character",
            manjuAssetCode: "CHR-林澈",
            importSource: { provider: "manju", projectId: "project-1", stage: "p2_assets" },
        });
        expect(pipeline.connections).toEqual([]);
        expect(resolveManjuStoryboardAssetReferences(storyboardTable!, storyboardRows[0]!, pipeline.nodes)).toEqual([
            expect.objectContaining({
                code: "CHR-林澈",
                name: "P2 参考 · 林澈",
                kind: "character",
                pending: false,
                node: expect.objectContaining({ id: referenceNode?.id }),
            }),
        ]);
        expect([...buildManjuStoryboardAssetReferenceMap(storyboardTable, storyboardRows, pipeline.nodes).values()].map((references) => references.map((reference) => reference.code))).toEqual([
            ["CHR-林澈"],
            ["CHR-林澈"],
        ]);
        expect(referenceNode?.metadata?.storageKey).toBe("resource:res-character");
    });

    test("P3 import carries the complete P2 asset prompt library", () => {
        const p3Package = {
            ...P3_PACKAGE,
            tasks: P3_PACKAGE.tasks.map((task) => ({ ...task, referenceCodes: ["CHR-01", "PRP-未登记"] })),
        };
        const pipeline = createManjuCanvasPackage(
            p3Package,
            [
                { code: "CHR-01", title: "林澈", taskId: "p2-chr", prompt: "角色四格完整提示词" },
                { code: "SCN-老宅", title: "老宅", taskId: "p2-scn", prompt: "场景母版完整提示词" },
                { code: "PRP-旧信", title: "旧信", taskId: "p2-prp", prompt: "道具设定完整提示词" },
            ],
            { x: 0, y: 0 },
        );
        const rows = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows || [];

        expect(rows.map((row) => [row.manjuAsset?.code, row.prompt])).toEqual([
            ["CHR-01", "角色四格完整提示词"],
            ["SCN-老宅", "场景母版完整提示词"],
            ["PRP-旧信", "道具设定完整提示词"],
        ]);
    });

    test("P3 missing references stay inside the storyboard table without creating image nodes", () => {
        const p3Package = {
            ...P3_PACKAGE,
            tasks: [{
                ...P3_PACKAGE.tasks[0],
                referenceCodes: ["CHR-01", "PRP-缺失甲", "PRP-缺失乙", "SCN-缺失场景"],
            }],
        };
        const pipeline = createManjuCanvasPackage(p3Package, [
            { code: "CHR-01", title: "林澈", prompt: "角色提示词" },
        ], { x: 0, y: 0 });
        const storyboardTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        const visibleReferenceImages = pipeline.nodes.filter((node) =>
            node.type === CanvasNodeType.Image
            && !node.metadata?.manjuAssetTableId
            && Boolean(node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode),
        );

        expect(storyboardTable?.metadata?.batchTable?.rows[0]?.manjuStoryboard?.referenceCodes).toEqual([
            "CHR-01", "PRP-缺失甲", "PRP-缺失乙", "SCN-缺失场景",
        ]);
        expect(visibleReferenceImages).toEqual([]);
    });

    test("removes legacy standalone P2 placeholder image nodes once an asset table exists", () => {
        const packageWithAsset: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-placeholder-clean",
            stage: "p2_assets",
            assetRunId: "run-placeholder-clean",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "placeholder-clean",
            tasks: [
                { id: "p2-chr", kind: "image", code: "CHR-01", assetCode: "CHR-01", title: "角色", prompt: "角色提示词", category: "character", sourceKind: "ASSET_DELIVERY_M1" },
            ],
        };
        const pipeline = createManjuCanvasPackage(packageWithAsset, [], { x: 0, y: 0 });
        const placeholder = {
            id: "legacy-placeholder",
            type: CanvasNodeType.Image,
            title: "待补参考",
            position: { x: 0, y: 0 },
            width: 320,
            height: 180,
            metadata: {
                manjuStage: "p2_assets" as const,
                manjuAssetCode: "PRP-未登记",
                manjuTaskCode: "PRP-未登记",
            },
        } satisfies CanvasNodeData;
        const realImage = {
            ...placeholder,
            id: "real-image",
            metadata: { ...placeholder.metadata, content: "/api/resources/real/file", manjuAssetCode: "PRP-真实" },
        } satisfies CanvasNodeData;

        const cleaned = removeObsoleteManjuReferenceNodes([...pipeline.nodes, placeholder, realImage]);
        expect(cleaned.some((node) => node.id === "legacy-placeholder")).toBe(false);
        expect(cleaned.some((node) => node.id === "real-image")).toBe(true);
    });

    test("legacy CNH P2 asset tasks still render in the digital asset table", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-cnh-assets",
            stage: "p2_assets",
            assetRunId: "run-cnh-assets",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "cnh-assets",
            tasks: [{
                id: "p2-cnh",
                kind: "image",
                code: "图1",
                assetCode: "CNH-林澈",
                title: "林澈角色四格",
                prompt: "林澈角色四格提示词",
                category: "character",
                sourceKind: "ASSET_DELIVERY_M1",
            }],
        };

        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const assetTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);

        expect(assetTable?.metadata?.batchTable?.rows[0]?.manjuAsset).toMatchObject({
            code: "CHR-林澈",
            kind: "character",
            name: "林澈角色四格",
        });
    });

    test("new P2 append viewport centers the asset table instead of restoring the old canvas position", () => {
        const viewport = manjuCanvasViewportForNodes([{
            id: "asset-table-offscreen",
            type: CanvasNodeType.BatchTable,
            title: "漫剧数字资产表",
            position: { x: 8000, y: 1200 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative",
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [],
                    manjuAssetTable: { version: 1 },
                },
            },
        }], { width: 1200, height: 800 });

        expect(viewport).toBeDefined();
        expect(viewport!.x + (8000 + 640) * viewport!.k).toBeCloseTo(600, 5);
        expect(viewport!.y + (1200 + 360) * viewport!.k).toBeCloseTo(400, 5);
    });

    test("asset table opens the first populated category after a scene-only append", () => {
        const table = {
            operation: "creative" as const,
            concurrency: 1,
            referenceColumns: [],
            rows: [{
                id: "scene-row",
                enabled: true,
                inputNodeIds: [],
                prompt: "场景提示词",
                manjuAsset: { taskId: "scene-task", code: "SCN-01", kind: "scene" as const, name: "场景" },
            }],
        };

        expect(preferredManjuAssetKind(table, "character")).toBe("scene");
    });

    test("repairs asset rows that still point to the stale node after recovered media was created", () => {
        const table = {
            id: "asset-table-recovery",
            type: CanvasNodeType.BatchTable,
            title: "资产表",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "asset-row-recovery",
                        enabled: true,
                        inputNodeIds: [],
                        outputNodeId: "stale-image-node",
                        prompt: "角色提示词",
                        manjuAsset: { taskId: "asset-task-recovery", code: "CHR-01", kind: "character" as const, name: "林澈" },
                    }],
                    manjuAssetTable: { version: 1 as const, projectId: "project-recovery", sourceStage: "p2_assets" as const },
                },
                generationBatches: [{
                    id: "batch-recovery",
                    projectId: "project-recovery",
                    sourceNodeId: "asset-table-recovery",
                    mode: "batch_image" as const,
                    status: "partial_failed" as const,
                    items: [{
                        id: "batch-item-recovery",
                        rowId: "asset-row-recovery",
                        nodeId: "stale-image-node",
                        status: "failed" as const,
                        retryCount: 0,
                        taskId: "task-recovery",
                    }],
                    createdAt: "2026-01-01",
                    updatedAt: "2026-01-01",
                }],
            },
        } satisfies CanvasNodeData;
        const recovered = {
            id: "recovered-image-node",
            type: CanvasNodeType.Image,
            title: "图片恢复",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                taskId: "task-recovery",
                taskStatus: "succeeded" as const,
                status: "success" as const,
                content: "/api/resources/recovered/file",
                storageKey: "resource:recovered",
            },
        } satisfies CanvasNodeData;

        const repaired = repairRecoveredManagedTableLinks([table, recovered]);
        const repairedTable = repaired.find((node) => node.id === table.id);
        const repairedImage = repaired.find((node) => node.id === recovered.id);
        expect(repairedTable?.metadata?.batchTable?.rows[0]?.outputNodeId).toBe(recovered.id);
        expect(repairedTable?.metadata?.generationBatches?.[0]?.items[0]).toMatchObject({
            nodeId: recovered.id,
            status: "succeeded",
        });
        expect(repairedImage?.metadata).toMatchObject({
            manjuAssetTableId: table.id,
            manjuAssetRowId: "asset-row-recovery",
            manjuAssetCode: "CHR-01",
        });
    });

    test("repairs an asset row from recovered media that already carries its row identity", () => {
        const table = {
            id: "asset-table-row-identity",
            type: CanvasNodeType.BatchTable,
            title: "资产表",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "asset-row-row-identity",
                        enabled: true,
                        inputNodeIds: [],
                        outputNodeId: "stale-row-identity",
                        prompt: "角色提示词",
                        manjuAsset: { taskId: "asset-task-row-identity", code: "CHR-02", kind: "character" as const, name: "梁崇" },
                    }],
                    manjuAssetTable: { version: 1 as const, projectId: "project-row-identity", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const recovered = {
            id: "recovered-row-identity",
            type: CanvasNodeType.Image,
            title: "CHR-02 · 梁崇",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                taskId: "generation-task-row-identity",
                taskStatus: "succeeded" as const,
                status: "success" as const,
                content: "/api/resources/recovered-row/file",
                storageKey: "resource:recovered-row",
                manjuStage: "p2_assets" as const,
                manjuAssetTableId: table.id,
                manjuAssetRowId: "asset-row-row-identity",
                manjuAssetCode: "CHR-02",
            },
        } satisfies CanvasNodeData;

        const repaired = repairRecoveredManagedTableLinks([table, recovered]);
        const repairedTable = repaired.find((node) => node.id === table.id);

        expect(repairedTable?.metadata?.batchTable?.rows[0]?.outputNodeId).toBe(recovered.id);
        expect(repairedTable?.metadata?.batchTable?.rows[0]?.inputNodeIds).toEqual([]);
    });

    test("resolves P3 and P4 generation inputs from references without canvas wires", () => {
        const pipeline = createManjuCanvasPackage(
            P3_PACKAGE,
            [
                {
                    code: "CHR-林澈",
                    title: "林澈",
                    content: "/api/resources/res-character/file",
                    storageKey: "resource:res-character",
                    taskId: "p2-character-task",
                },
            ],
            { x: 0, y: 0 },
        );
        const assetTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const storyboardTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        const storyboardRow = storyboardTable?.metadata?.batchTable?.rows[0];
        const assetRow = assetTable?.metadata?.batchTable?.rows[0];
        const assetNode = pipeline.nodes.find((node) => node.id === assetRow?.outputNodeId);
        if (!assetTable || !storyboardTable || !storyboardRow || !assetNode) throw new Error("P3 fixture incomplete");

        const storyboardOutput: CanvasNodeData = {
            id: "p3-output",
            type: CanvasNodeType.Image,
            title: "段1 · 故事板图",
            position: { x: 0, y: 0 },
            width: 320,
            height: 180,
            metadata: {
                content: "data:image/png;base64,c3Rvcnlib2FyZA==",
                manjuStage: "p3_images",
                manjuTaskCode: "段1",
                manjuAssetTableId: storyboardTable.id,
                manjuAssetRowId: storyboardRow.id,
            },
        };
        storyboardRow.outputNodeId = storyboardOutput.id;
        const p4Output: CanvasNodeData = {
            id: "p4-output",
            type: CanvasNodeType.Video,
            title: "段1 · 分段视频",
            position: { x: 0, y: 0 },
            width: 320,
            height: 180,
            metadata: {
                manjuStage: "p4_videos",
                manjuTaskCode: "段1",
                manjuReferenceCodes: ["CHR-林澈", "段1"],
            },
        };
        const nodes = [...pipeline.nodes, storyboardOutput, p4Output];

        expect(resolveManjuInternalReferenceNodes(storyboardOutput, nodes).map((node) => node.id)).toEqual([assetNode.id]);
        expect(resolveManjuInternalReferenceNodes(p4Output, nodes).map((node) => node.id)).toEqual([assetNode.id, storyboardOutput.id]);
        expect(getGenerationResourceNodes(storyboardOutput.id, nodes, []).map((node) => node.id)).toEqual([assetNode.id]);
        expect(getGenerationResourceNodes(p4Output.id, nodes, []).map((node) => node.id)).toEqual([assetNode.id, storyboardOutput.id]);
        const references = buildCanvasNodeMentionReferenceMap(nodes, []);
        expect(references.get(storyboardOutput.id)?.map((reference) => reference.nodeId)).toEqual([assetNode.id]);
        expect(references.get(p4Output.id)?.map((reference) => reference.nodeId)).toEqual([assetNode.id, storyboardOutput.id]);
        expect(buildNodeMentionReferences(p4Output, nodes, []).map((reference) => reference.nodeId)).toEqual([assetNode.id, storyboardOutput.id]);
    });

    test("finds a generated storyboard asset even when the asset row link is stale", () => {
        const assetTable = {
            id: "asset-table",
            type: CanvasNodeType.BatchTable,
            title: "数字资产",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    rows: [{
                        id: "asset-row",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuAsset: { taskId: "asset-task", code: "CHR-01", kind: "character" as const, name: "林澈" },
                    }],
                    referenceColumns: [],
                    manjuAssetTable: { version: 1 as const, projectId: "project", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const storyboardTable = {
            id: "storyboard-table",
            type: CanvasNodeType.BatchTable,
            title: "故事板",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    rows: [{
                        id: "storyboard-row",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuStoryboard: {
                            taskId: "storyboard-task",
                            code: "段1",
                            title: "段1",
                            group: "storyboard" as const,
                            referenceCodes: ["CHR-01"],
                        },
                    }],
                    referenceColumns: [],
                    manjuStoryboardTable: { version: 1 as const, projectId: "project", sourceStage: "p3_images" as const },
                },
            },
        } satisfies CanvasNodeData;
        const generatedAsset = {
            id: "generated-asset",
            type: CanvasNodeType.Image,
            title: "CHR-01 · 林澈",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p2_assets" as const,
                manjuAssetCode: "CHR-01",
                manjuAssetRowId: "asset-row",
                content: "/api/resources/generated/file",
                storageKey: "resource:generated",
            },
        } satisfies CanvasNodeData;

        const [reference] = buildManjuStoryboardAssetReferenceMap(
            storyboardTable,
            storyboardTable.metadata.batchTable.rows,
            [assetTable, storyboardTable, generatedAsset],
        ).get("storyboard-row") || [];
        expect(reference).toMatchObject({
            code: "CHR-01",
            pending: false,
            node: expect.objectContaining({ id: "generated-asset" }),
        });
    });

    test("resolves asset row display media by row identity when the output link is stale", () => {
        const row = {
            id: "asset-row-stale-display",
            enabled: true,
            inputNodeIds: [],
            outputNodeId: "deleted-stale-display",
            prompt: "角色提示词",
            manjuAsset: { taskId: "asset-task", code: "CHR-04", kind: "character" as const, name: "梁汶玥" },
        };
        const recovered = {
            id: "recovered-row-display",
            type: CanvasNodeType.Image,
            title: "CHR-04 · 梁汶玥",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuAssetRowId: row.id,
                manjuAssetCode: "CHR-04",
                content: "/api/resources/recovered-display/file",
                storageKey: "resource:recovered-display",
            },
        } satisfies CanvasNodeData;

        const resolved = buildManjuAssetRowOutputMediaMap([row], [recovered]).get(row.id);

        expect(resolved?.id).toBe(recovered.id);
        expect(resolved?.metadata?.storageKey).toBe("resource:recovered-display");
    });

    test("uses the populated CHR-01 asset when an older asset table contains an empty duplicate", () => {
        const staleAssetTable = {
            id: "stale-asset-table",
            type: CanvasNodeType.BatchTable,
            title: "旧数字资产",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    rows: [{
                        id: "stale-chr-01",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuAsset: { taskId: "old-chr-task", code: "CHR-01", kind: "character" as const, name: "林澈" },
                    }],
                    referenceColumns: [],
                    manjuAssetTable: { version: 1 as const, projectId: "project", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const activeAssetTable = {
            ...staleAssetTable,
            id: "active-asset-table",
            title: "当前数字资产",
            metadata: {
                batchTable: {
                    ...staleAssetTable.metadata.batchTable,
                    rows: [{
                        ...staleAssetTable.metadata.batchTable.rows[0],
                        id: "active-chr-01",
                        outputNodeId: "active-chr-image",
                    }],
                },
            },
        } satisfies CanvasNodeData;
        const storyboardTable = {
            id: "storyboard-table",
            type: CanvasNodeType.BatchTable,
            title: "故事板",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    rows: [{
                        id: "storyboard-row",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuStoryboard: {
                            taskId: "storyboard-task",
                            code: "段1",
                            title: "段1",
                            group: "storyboard" as const,
                            referenceCodes: ["CHR-01"],
                        },
                    }],
                    referenceColumns: [],
                    manjuStoryboardTable: { version: 1 as const, projectId: "project", sourceStage: "p3_images" as const },
                },
            },
        } satisfies CanvasNodeData;
        const activeImage = {
            id: "active-chr-image",
            type: CanvasNodeType.Image,
            title: "CHR-01 · 林澈",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuAssetRowId: "active-chr-01",
                content: "/api/resources/active-chr/file",
                storageKey: "resource:active-chr",
            },
        } satisfies CanvasNodeData;

        const [reference] = resolveManjuStoryboardAssetReferences(
            storyboardTable,
            storyboardTable.metadata.batchTable.rows[0],
            [staleAssetTable, activeAssetTable, storyboardTable, activeImage],
        );
        expect(reference).toMatchObject({
            code: "CHR-01",
            rowId: "active-chr-01",
            pending: false,
            node: expect.objectContaining({ id: "active-chr-image" }),
        });
    });

    test("removes every Manju table wire while preserving unrelated links", () => {
        const nodes = [
            {
                id: "assets",
                type: CanvasNodeType.BatchTable,
                title: "资产",
                position: { x: 0, y: 0 },
                width: 1280,
                height: 720,
                metadata: {
                    batchTable: {
                        operation: "creative" as const,
                        concurrency: 5,
                        rows: [
                            { id: "asset-row", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "asset-task", code: "CHR-01", kind: "character" as const, name: "角色" } },
                            { id: "extra-row", enabled: true, inputNodeIds: [], prompt: "", manjuAsset: { taskId: "extra-task", code: "PRP-EXTRA", kind: "prop" as const, name: "手动道具" } },
                        ],
                        manjuAssetTable: { version: 1 as const, projectId: "project", sourceStage: "p2_assets" as const },
                    },
                },
            },
            {
                id: "storyboards",
                type: CanvasNodeType.BatchTable,
                title: "故事板",
                position: { x: 0, y: 0 },
                width: 1280,
                height: 720,
                metadata: {
                    batchTable: {
                        operation: "creative" as const,
                        concurrency: 5,
                        rows: [{ id: "story-row", enabled: true, inputNodeIds: [], prompt: "", manjuStoryboard: { taskId: "story-task", code: "段1", title: "段1", group: "storyboard" as const, referenceCodes: ["CHR-01"] } }],
                        manjuStoryboardTable: { version: 1 as const, projectId: "project", sourceStage: "p3_images" as const },
                    },
                },
            },
            {
                id: "style",
                type: CanvasNodeType.Text,
                title: "项目画风",
                position: { x: 0, y: 0 },
                width: 320,
                height: 180,
                metadata: { content: "style", manjuSharedStyleNode: true },
            },
            {
                id: "p4",
                type: CanvasNodeType.Video,
                title: "P4",
                position: { x: 0, y: 0 },
                width: 320,
                height: 180,
                metadata: { manjuStage: "p4_videos" as const, manjuReferenceCodes: ["CHR-01", "段1"], manjuGlobalStylePrompt: "style" },
            },
            {
                id: "manual-target",
                type: CanvasNodeType.Image,
                title: "手动",
                position: { x: 0, y: 0 },
                width: 320,
                height: 180,
                metadata: {},
            },
            {
                id: "other-target",
                type: CanvasNodeType.Image,
                title: "普通节点",
                position: { x: 0, y: 0 },
                width: 320,
                height: 180,
                metadata: {},
            },
        ] satisfies CanvasNodeData[];
        const connections: CanvasConnection[] = [
            { id: "manju-auto:duplicate", fromNodeId: "assets", fromHandleId: manjuAssetOutputHandleId("asset-row"), toNodeId: "storyboards", toHandleId: manjuAssetInputHandleId("story-row") },
            { id: "legacy-internal", fromNodeId: "assets", fromHandleId: manjuAssetOutputHandleId("asset-row"), toNodeId: "storyboards", toHandleId: manjuAssetInputHandleId("story-row") },
            { id: "internal-p4", fromNodeId: "assets", fromHandleId: manjuAssetOutputHandleId("asset-row"), toNodeId: "p4" },
            { id: "internal-style", fromNodeId: "style", toNodeId: "p4" },
            { id: "manual-asset-extra", fromNodeId: "assets", fromHandleId: manjuAssetOutputHandleId("extra-row"), toNodeId: "storyboards", toHandleId: manjuAssetInputHandleId("story-row") },
            { id: "manual-extra", fromNodeId: "assets", fromHandleId: manjuAssetOutputHandleId("asset-row"), toNodeId: "manual-target" },
            { id: "unrelated", fromNodeId: "manual-target", toNodeId: "other-target" },
        ];

        expect(cleanupManjuInternalConnections(nodes, connections).map((connection) => connection.id)).toEqual(["unrelated"]);
    });

    test("lays out imported Manju nodes without overlapping visible boxes", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-layout",
            stage: "p4_videos",
            productionRunId: "run-p4-layout",
            sourceArtifact: "FEED_PACKAGE.md",
            title: "P4 分段视频",
            hash: "p4-layout-hash",
            style: {
                aspectRatio: "9:16",
                imageStylePrompt: "统一画风",
                referenceResourceId: "style-reference",
                referenceStorageKey: "resource:style-reference",
            },
            tasks: Array.from({ length: 6 }, (_, index) => ({
                id: `p4-layout-${index + 1}`,
                kind: "video" as const,
                code: `段${index + 1}`,
                title: `分段视频 · 段${index + 1}`,
                prompt: `分段提示词 ${index + 1}`,
                category: "material",
                seconds: 15,
                aspectRatio: "9:16",
                sourceKind: "FEED_PACKAGE.md",
                group: "segment",
                referenceCodes: ["CHR-01", `段${index + 1}`],
            })),
        };
        const pipeline = createManjuCanvasPackage(pkg, [{ code: "CHR-01", title: "角色", taskId: "p2-1", content: "/api/resources/chr/file", storageKey: "resource:chr" }], { x: 0, y: 0 });
        const visible = pipeline.nodes.filter((node) => !node.metadata?.manjuAssetTableId);

        expect(overlappingCanvasNodes(visible)).toEqual([]);
    });

    test("repairs legacy overlapping Manju canvases once without moving normalized nodes again", () => {
        const pipeline = createManjuCanvasPackage(P3_PACKAGE, [{ code: "CHR-林澈", title: "林澈", taskId: "p2-1", content: "/api/resources/chr/file", storageKey: "resource:chr" }], { x: 0, y: 0 });
        const legacy = pipeline.nodes.map((node) => ({
            ...node,
            position: { x: 0, y: 0 },
            metadata: { ...node.metadata, manjuLayoutVersion: undefined },
        }));

        expect(overlappingCanvasNodes(legacy.filter((node) => !node.metadata?.manjuAssetTableId)).length).toBeGreaterThan(0);
        const repaired = normalizeManjuCanvasLayout(legacy);
        expect(overlappingCanvasNodes(repaired.filter((node) => !node.metadata?.manjuAssetTableId))).toEqual([]);
        expect(normalizeManjuCanvasLayout(repaired)).toBe(repaired);
    });

    test("repairs an asset table row whenever a missing prompt becomes available", () => {
        const p3Pipeline = createManjuCanvasPackage(P3_PACKAGE, [{ code: "CHR-林澈", title: "林澈", taskId: "p2-1", content: "/api/resources/chr/file", storageKey: "resource:chr" }], { x: 0, y: 0 });
        const assetTable = p3Pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const row = assetTable?.metadata?.batchTable?.rows[0];
        if (!assetTable || !row) throw new Error("asset fixture incomplete");
        const legacyNodes: CanvasNodeData[] = [
            {
                ...assetTable,
                metadata: {
                    ...assetTable.metadata,
                    batchTable: {
                        ...assetTable.metadata!.batchTable!,
                        rows: [{ ...row, outputNodeId: "p2-backing", prompt: "" }],
                    },
                },
            },
            {
                id: "p2-backing",
                type: CanvasNodeType.Image,
                title: "林澈",
                position: { x: 0, y: 0 },
                width: 320,
                height: 320,
                metadata: {
                    manjuStage: "p2_assets",
                    manjuTaskId: "p2-1",
                    manjuAssetCode: "CHR-林澈",
                    manjuAssetRowId: row.id,
                    prompt: "P2 角色四格提示词",
                    composerContent: "P2 角色四格提示词",
                    content: "/api/resources/chr/file",
                },
            },
        ];

        const repaired = repairManjuAssetTablePrompts(legacyNodes);
        const repairedTable = repaired.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        expect(repairedTable?.metadata?.batchTable?.rows[0]?.prompt).toBe("P2 角色四格提示词");
        expect(repairedTable?.metadata?.batchTable?.manjuAssetTable?.promptRepairVersion).toBe(2);

        const updatedSourceAndClearedRow = repaired.map((node) =>
            node.id === repairedTable?.id
                ? { ...node, metadata: { ...node.metadata, batchTable: { ...node.metadata!.batchTable!, rows: node.metadata!.batchTable!.rows.map((item) => ({ ...item, prompt: "" })) } } }
                : node.id === "p2-backing"
                  ? { ...node, metadata: { ...node.metadata, prompt: "更新后的 P2 角色四格提示词", composerContent: "更新后的 P2 角色四格提示词" } }
                  : node,
        );
        const repairedAgain = repairManjuAssetTablePrompts(updatedSourceAndClearedRow);
        expect(repairedAgain.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows[0]?.prompt).toBe("更新后的 P2 角色四格提示词");
    });

    test("changes the managed-table render revision when a generated asset image becomes available", () => {
        const pending: CanvasNodeData = {
            id: "asset-output",
            type: CanvasNodeType.Image,
            title: "角色资产",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p2_assets",
                manjuAssetCode: "CHR-01",
                manjuAssetRowId: "asset-row",
                status: "idle",
            },
        };
        const completed = {
            ...pending,
            metadata: {
                ...pending.metadata,
                status: "success" as const,
                content: "/api/resources/asset/file",
                storageKey: "resource:asset",
            },
        };

        expect(manjuCanvasAssetRevision([pending])).not.toBe(manjuCanvasAssetRevision([completed]));
    });

    test("matches legacy CNH references against standard CHR asset rows", () => {
        const assetTable = {
            id: "assets",
            type: CanvasNodeType.BatchTable,
            title: "资产",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 5,
                    rows: [{
                        id: "asset-row",
                        enabled: true,
                        inputNodeIds: ["asset-image"],
                        outputNodeId: "asset-image",
                        prompt: "",
                        manjuAsset: { taskId: "asset-task", code: "CHR-01", kind: "character" as const, name: "角色" },
                    }],
                    manjuAssetTable: { version: 1 as const, projectId: "project", sourceStage: "p2_assets" as const },
                },
            },
        };
        const storyboardTable = {
            id: "storyboards",
            type: CanvasNodeType.BatchTable,
            title: "故事板",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 5,
                    rows: [{
                        id: "story-row",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "",
                        manjuStoryboard: { taskId: "story", code: "段1", title: "段1", group: "storyboard" as const, referenceCodes: ["CNH-01"] },
                    }],
                    manjuStoryboardTable: { version: 1 as const, projectId: "project", sourceStage: "p3_images" as const },
                },
            },
        };
        const assetImage: CanvasNodeData = {
            id: "asset-image",
            type: CanvasNodeType.Image,
            title: "角色",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: { content: "/api/resources/asset/file", storageKey: "resource:asset" },
        };

        expect(resolveManjuStoryboardAssetReferences(storyboardTable, storyboardTable.metadata.batchTable.rows[0], [assetTable, storyboardTable, assetImage])).toEqual([
            expect.objectContaining({ code: "CNH-01", name: "角色", pending: false, node: expect.objectContaining({ id: "asset-image" }) }),
        ]);
    });

    test("uses identifier-first titles for imported image tasks", () => {
        const p3Pipeline = createManjuCanvasPackage(P3_PACKAGE, [], { x: 0, y: 0 });
        const p3Titles = p3Pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable)?.metadata?.batchTable?.rows.map((row) => row.manjuStoryboard?.title);
        const p2Package: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-1",
            stage: "p2_assets",
            assetRunId: "run-p2",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "p2-title",
            tasks: [
                {
                    id: "p2-character-1",
                    kind: "image",
                    code: "图1",
                    assetCode: "CHR-林澈",
                    title: "林澈角色四格",
                    prompt: "角色四格提示词",
                    category: "character",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
            ],
        };
        const p2Pipeline = createManjuCanvasPackage(p2Package, [], { x: 0, y: 0 });
        const p2Table = p2Pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const p2Row = p2Table?.metadata?.batchTable?.rows.find((row) => row.manjuAsset?.taskId === "p2-character-1");

        expect(p3Titles).toEqual(["段1 · 故事板图", "场1 · 剧情分镜格"]);
        expect(p2Row?.manjuAsset).toEqual({
            taskId: "p2-character-1",
            code: "CHR-林澈",
            taskCode: "图1",
            kind: "character",
            name: "林澈角色四格",
        });
    });

    test("appends later Manju stages to one canvas without duplicating P2 references", () => {
        const p2Node = {
            id: "existing-p2-character",
            type: CanvasNodeType.Image,
            title: "P2 林澈",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p2_assets" as const,
                manjuTaskId: "p2-character-task",
                manjuAssetCode: "CHR-林澈",
                prompt: "P2 角色四格提示词",
                composerContent: "P2 角色四格提示词",
                content: "/api/resources/res-character/file",
                storageKey: "resource:res-character",
            },
        };
        const pipeline = createManjuCanvasPackage(P3_PACKAGE, [{ code: "CHR-林澈", title: "P2 参考 · 林澈", taskId: "p2-character-task", content: "/api/resources/res-character/file", storageKey: "resource:res-character" }], { x: 1200, y: 800 });
        const merged = mergeManjuCanvasPackage([p2Node], [], pipeline, "p3_images");

        expect(merged.addedTaskCount).toBe(2);
        const assetTable = merged.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const assetRow = assetTable?.metadata?.batchTable?.rows.find((row) => row.manjuAsset?.code === "CHR-林澈");
        expect(assetRow?.outputNodeId).toBe(p2Node.id);
        expect(assetRow?.prompt).toBe("P2 角色四格提示词");
        const storyboardTable = merged.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        expect(storyboardTable?.metadata?.batchTable?.rows).toHaveLength(2);
        expect(merged.connections).toEqual([]);
    });

    test("prefers absorbed P2 media over a stale managed-table output link", () => {
        const existingTable = {
            id: "asset-table-stale-output",
            type: CanvasNodeType.BatchTable,
            title: "漫剧数字资产表",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "asset-row-stale-output",
                        enabled: true,
                        inputNodeIds: [],
                        outputNodeId: "deleted-stale-output",
                        prompt: "林澈角色四格提示词",
                        manjuAsset: { taskId: "p2-character-task", code: "CHR-林澈", kind: "character" as const, name: "林澈" },
                    }],
                    manjuAssetTable: { version: 1 as const, projectId: "project-stale-output", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const recoveredMedia = {
            id: "recovered-character-media",
            type: CanvasNodeType.Image,
            title: "CHR-林澈 · 林澈",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p2_assets" as const,
                manjuTaskId: "p2-character-task",
                manjuAssetCode: "CHR-林澈",
                content: "/api/resources/recovered-character/file",
                storageKey: "resource:recovered-character",
            },
        } satisfies CanvasNodeData;
        const p2Package: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-stale-output",
            stage: "p2_assets",
            assetRunId: "run-p2-stale-output",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "p2-stale-output",
            tasks: [{
                id: "p2-character-task",
                kind: "image",
                code: "图1",
                assetCode: "CHR-林澈",
                title: "林澈角色四格",
                prompt: "林澈角色四格提示词",
                category: "character",
                sourceKind: "ASSET_DELIVERY_M1",
            }],
        };
        const incoming = createManjuCanvasPackage(p2Package, [], { x: 0, y: 0 });

        const merged = mergeManjuCanvasPackage([existingTable, recoveredMedia], [], incoming, "p2_assets");
        const assetRow = merged.nodes
            .find((node) => node.metadata?.batchTable?.manjuAssetTable)
            ?.metadata?.batchTable?.rows.find((row) => row.manjuAsset?.code === "CHR-林澈");

        expect(assetRow?.outputNodeId).toBe(recoveredMedia.id);
    });

    test("appending P3 keeps every existing P2 asset row and its prompt", () => {
        const p2Package: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-merge-preserve",
            stage: "p2_assets",
            assetRunId: "run-p2",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "p2-preserve",
            tasks: [
                {
                    id: "p2-chr",
                    kind: "image",
                    code: "图1",
                    assetCode: "CHR-01",
                    title: "林澈角色四格",
                    prompt: "林澈角色四格完整提示词",
                    category: "character",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
                {
                    id: "p2-scn",
                    kind: "image",
                    code: "图2",
                    assetCode: "SCN-老宅",
                    title: "老宅场景母版",
                    prompt: "老宅场景母版完整提示词",
                    category: "environment",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
            ],
        };
        const p2Pipeline = createManjuCanvasPackage(p2Package, [], { x: 0, y: 0 });
        const p3Package: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            projectId: "project-merge-preserve",
            tasks: [{
                ...P3_PACKAGE.tasks[0],
                id: "p3-only-character",
                referenceCodes: ["CHR-01"],
            }],
        };
        const p3Pipeline = createManjuCanvasPackage(p3Package, [], { x: 1600, y: 0 });
        const merged = mergeManjuCanvasPackage(p2Pipeline.nodes, p2Pipeline.connections, p3Pipeline, "p3_images");
        const rows = merged.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows || [];

        expect(rows).toHaveLength(2);
        expect(rows.find((row) => row.manjuAsset?.code === "CHR-01")?.prompt).toBe("林澈角色四格完整提示词");
        expect(rows.find((row) => row.manjuAsset?.code === "SCN-老宅")?.prompt).toBe("老宅场景母版完整提示词");
    });

    test("stage reimports preserve manually added asset and storyboard rows", () => {
        const existingAssetTable = {
            id: "asset-table-manual-preserve",
            type: CanvasNodeType.BatchTable,
            title: "数字资产",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "manual-asset-preserve",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "自定义资产提示词",
                        manjuAsset: { taskId: "manual:asset:manual-asset-preserve", code: "CHR-12", kind: "character" as const, name: "自定义人物12" },
                    }],
                    manjuAssetTable: { version: 1 as const, projectId: "project-manual-preserve", sourceStage: "p2_assets" as const },
                },
            },
        } satisfies CanvasNodeData;
        const existingStoryboardTable = {
            id: "storyboard-table-manual-preserve",
            type: CanvasNodeType.BatchTable,
            title: "故事板",
            position: { x: 0, y: 0 },
            width: 1280,
            height: 720,
            metadata: {
                batchTable: {
                    operation: "creative" as const,
                    concurrency: 1,
                    referenceColumns: [],
                    rows: [{
                        id: "manual-storyboard-preserve",
                        enabled: true,
                        inputNodeIds: [],
                        prompt: "自定义故事板提示词",
                        manjuStoryboard: { taskId: "manual:storyboard:manual-storyboard-preserve", code: "段17", title: "自定义故事板17", group: "storyboard" as const, referenceCodes: [] },
                    }],
                    manjuStoryboardTable: { version: 1 as const, projectId: "project-manual-preserve", sourceStage: "p3_images" as const },
                },
            },
        } satisfies CanvasNodeData;
        const p2Package: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-manual-preserve",
            stage: "p2_assets",
            assetRunId: "run-p2",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2",
            hash: "p2-manual-preserve",
            tasks: [{ id: "p2-chr-01", kind: "image", code: "图1", assetCode: "CHR-01", title: "林澈", prompt: "角色提示词", category: "character", sourceKind: "ASSET_DELIVERY_M1" }],
        };
        const p2Import = createManjuCanvasPackage(p2Package, [], { x: 0, y: 0 });
        const afterP2 = mergeManjuCanvasPackage([existingAssetTable, existingStoryboardTable], [], p2Import, "p2_assets");

        const p3Package: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            projectId: "project-manual-preserve",
        };
        const p3Import = createManjuCanvasPackage(p3Package, [], { x: 0, y: 0 });
        const afterP3 = mergeManjuCanvasPackage(afterP2.nodes, afterP2.connections, p3Import, "p3_images");

        expect(afterP3.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows.some((row) => row.id === "manual-asset-preserve")).toBe(true);
        expect(afterP3.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable)?.metadata?.batchTable?.rows.some((row) => row.id === "manual-storyboard-preserve")).toBe(true);
    });

    test("reimporting P3 removes legacy non-P2 asset rows and duplicate codes", () => {
        const p2Package: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-clean-assets",
            stage: "p2_assets",
            assetRunId: "run-p2-clean",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "p2-clean",
            tasks: [
                { id: "p2-chr", kind: "image", code: "CHR-01", assetCode: "CHR-01", title: "林澈", prompt: "角色提示词", category: "character", sourceKind: "ASSET_DELIVERY_M1" },
                { id: "p2-scn", kind: "image", code: "SCN-老宅", assetCode: "SCN-老宅", title: "老宅", prompt: "场景提示词", category: "environment", sourceKind: "ASSET_DELIVERY_M1" },
            ],
        };
        const p2Pipeline = createManjuCanvasPackage(p2Package, [], { x: 0, y: 0 });
        const p2Table = p2Pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        if (!p2Table?.metadata?.batchTable) throw new Error("P2 asset fixture incomplete");
        const chrRow = p2Table.metadata.batchTable.rows.find((row) => row.manjuAsset?.code === "CHR-01");
        if (!chrRow) throw new Error("CHR fixture incomplete");
        const dirtyNodes = p2Pipeline.nodes.map((node) =>
            node.id === p2Table.id
                ? {
                      ...node,
                      metadata: {
                          ...node.metadata,
                          batchTable: {
                              ...node.metadata!.batchTable!,
                              rows: [
                                  ...node.metadata!.batchTable!.rows,
                                  { ...chrRow, id: "duplicate-chr", prompt: "重复角色提示词" },
                                  {
                                      id: "p3-only-row",
                                      enabled: true,
                                      inputNodeIds: [],
                                      prompt: "",
                                      manjuAsset: { taskId: "asset:PRP-未登记", code: "PRP-未登记", kind: "prop" as const, name: "未登记道具" },
                                  },
                              ],
                          },
                      },
                  }
                : node,
        );
        const p3Package: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            tasks: [{ ...P3_PACKAGE.tasks[0], referenceCodes: ["CHR-01", "PRP-未登记"] }],
        };
        const p3Pipeline = createManjuCanvasPackage(p3Package, [
            { code: "CHR-01", title: "林澈", prompt: "角色提示词" },
            { code: "SCN-老宅", title: "老宅", prompt: "场景提示词" },
        ], { x: 1600, y: 0 });
        const merged = mergeManjuCanvasPackage(dirtyNodes, p2Pipeline.connections, p3Pipeline, "p3_images");
        const rows = merged.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable)?.metadata?.batchTable?.rows || [];

        expect(rows.map((row) => row.manjuAsset?.code)).toEqual(["CHR-01", "SCN-老宅"]);
        expect(rows.find((row) => row.manjuAsset?.code === "CHR-01")?.prompt).toBe("角色提示词");
    });

    test("reimporting P2 removes stale false tasks and refreshes empty task prompts", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-1",
            stage: "p2_assets",
            assetRunId: "run-p2",
            sourceArtifact: "APPEARANCE_ARCHIVE_M0.md",
            title: "P2 数字资产",
            hash: "p2-hash",
            tasks: [
                {
                    id: "p2-chr-01",
                    kind: "image",
                    code: "CHR-01",
                    assetCode: "CHR-01",
                    title: "CHR-01 正确提示词",
                    prompt: "正确角色四格提示词",
                    category: "character",
                    sourceKind: "ASSET_APPEARANCE_M0",
                    group: "character",
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const existingTask = (id: string, taskId: string, prompt: string, position: number) => ({
            id,
            type: CanvasNodeType.Image,
            title: id,
            position: { x: position, y: 0 },
            width: 320,
            height: 320,
            metadata: { manjuStage: "p2_assets" as const, manjuTaskId: taskId, manjuAssetCode: taskId, prompt, composerContent: prompt, workflowTitle: id },
        });
        const merged = mergeManjuCanvasPackage(
            [existingTask("stale-range", "p2-range", "错误分组", 0), existingTask("real-task", "p2-chr-01", "旧提示词", 420), existingTask("duplicate-summary", "p2-chr-01", "错误接线摘要", 840)],
            [],
            pipeline,
            "p2_assets",
        );

        const table = merged.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
        const rows = table?.metadata?.batchTable?.rows || [];
        expect(rows).toHaveLength(1);
        expect(rows[0]?.manjuAsset?.taskId).toBe("p2-chr-01");
        expect(rows[0]?.prompt).toBe("正确角色四格提示词");
    });

    test("reimporting P3 refreshes the storyboard prompt and 16:9 canvas size", () => {
        const existingTask = {
            id: "existing-p3-storyboard",
            type: CanvasNodeType.Image,
            title: "旧故事板图",
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p3_images" as const,
                manjuTaskId: "p3-storyboard-seg-1",
                prompt: "旧的 9:16 连续故事板提示词",
                composerContent: "旧的 9:16 连续故事板提示词",
                size: "9:16",
            },
        };
        const pipeline = createManjuCanvasPackage(P3_PACKAGE, [], { x: 0, y: 0 });
        const merged = mergeManjuCanvasPackage([existingTask], [], pipeline, "p3_images");
        const storyboardTable = merged.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        const updated = storyboardTable?.metadata?.batchTable?.rows.find((row) => row.manjuStoryboard?.taskId === "p3-storyboard-seg-1");

        expect(updated?.prompt).toBe("故事板图提示词");
        expect(updated?.outputNodeId).toBe(existingTask.id);
    });

    test("image references split composite cells and exclude AUD audio codes", () => {
        expect(normalizeManjuVisualReferenceCodes(["CHR-A-01 / PRP-A-02 / AUD-B-01", "SCN-A-01、AUD-C-02", "AUD-A-01"])).toEqual(["CHR-A-01", "PRP-A-02", "SCN-A-01"]);

        const pkg: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            tasks: [
                {
                    ...P3_PACKAGE.tasks[0],
                    referenceCodes: ["CHR-林澈 / PRP-旧信 / AUD-B-01"],
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(
            pkg,
            [
                { code: "CHR-林澈", title: "林澈", content: "/api/resources/chr/file" },
                { code: "PRP-旧信", title: "旧信", content: "/api/resources/letter/file" },
            ],
            { x: 0, y: 0 },
        );
        const referenceCodes = pipeline.nodes
            .find((node) => node.metadata?.batchTable?.manjuAssetTable)
            ?.metadata?.batchTable?.rows.map((row) => row.manjuAsset?.code)
            .filter(Boolean);
        expect(referenceCodes).toEqual(["CHR-林澈", "PRP-旧信"]);
        expect(referenceCodes).not.toContain("AUD-B-01");
    });

    test("reimporting P3 removes stale AUD and composite reference placeholders", () => {
        const staleNode = (id: string, code: string) => ({
            id,
            type: CanvasNodeType.Image,
            title: `${code} · 待补参考`,
            position: { x: 0, y: 0 },
            width: 320,
            height: 320,
            metadata: {
                manjuStage: "p2_assets" as const,
                manjuAssetCode: code,
            },
        });
        const pkg: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            tasks: [
                {
                    ...P3_PACKAGE.tasks[0],
                    referenceCodes: ["CHR-林澈"],
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(pkg, [{ code: "CHR-林澈", title: "林澈", content: "/api/resources/chr/file" }], { x: 0, y: 0 });
        const merged = mergeManjuCanvasPackage([staleNode("stale-audio", "AUD-B-01"), staleNode("stale-composite", "CHR-林澈 / PRP-旧信 / AUD-A-01")], [], pipeline, "p3_images");
        const referenceCodes = merged.nodes
            .find((node) => node.metadata?.batchTable?.manjuAssetTable)
            ?.metadata?.batchTable?.rows.map((row) => row.manjuAsset?.code)
            .filter(Boolean);

        expect(referenceCodes).toEqual(["CHR-林澈"]);
    });

    test("P4 keeps segment text exact while adding global style only for execution", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-1",
            stage: "p4_videos",
            productionRunId: "run-p4",
            sourceArtifact: "FEED_PACKAGE.md",
            title: "P4 分段视频",
            hash: "p4-hash",
            tasks: [
                {
                    id: "p4-segment-1",
                    kind: "video",
                    code: "段1",
                    title: "分段视频 · 段1",
                    prompt: "【画幅风格】9:16 横屏\n【时间轴分镜】00:00-00:15",
                    globalStylePrompt: "【STYLE｜视觉风格 · 全片锁死】\n光比 [1:5, 1:9]",
                    referenceCodes: ["CHR-林澈", "段1"],
                    category: "material",
                    seconds: 15,
                    aspectRatio: "9:16",
                    sourceKind: "FEED_PACKAGE.md",
                    group: "segment",
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const node = pipeline.nodes.find((item) => item.metadata?.manjuTaskId === "p4-segment-1");

        expect(node?.metadata?.prompt).toBe("【画幅风格】9:16 横屏\n【时间轴分镜】00:00-00:15");
        expect(node?.metadata?.manjuGlobalStylePrompt).toBe("【STYLE｜视觉风格 · 全片锁死】\n光比 [1:5, 1:9]");
        expect(node?.metadata?.manjuReferenceCodes).toEqual(["CHR-林澈", "段1"]);
        expect(composeManjuVideoExecutionPrompt(node?.metadata?.prompt || "", node?.metadata?.manjuGlobalStylePrompt)).toBe("【STYLE｜视觉风格 · 全片锁死】\n光比 [1:5, 1:9]\n\n【画幅风格】9:16 横屏\n【时间轴分镜】00:00-00:15");
    });

    test("three stages share one project style node and P4 enriches it", () => {
        const p3Pipeline = createManjuCanvasPackage(P3_PACKAGE, [], { x: 0, y: 0 });
        const p4Package: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            stage: "p4_videos",
            productionRunId: "run-p4",
            title: "P4 分段视频",
            hash: "p4-hash",
            sourceArtifact: "FEED_PACKAGE.md",
            style: {
                ...P3_PACKAGE.style,
                globalStylePrompt: "【STYLE｜视觉风格 · 全片锁死】\n光比 [1:5, 1:9]\n\n【PERFORMANCE｜表演与动作 · 全片锁死】\n帧率区间：[24fps, 60fps]",
            },
            tasks: [
                {
                    id: "p4-segment-1",
                    kind: "video",
                    code: "段1",
                    title: "分段视频 · 段1",
                    prompt: "【画幅风格】9:16 横屏",
                    category: "material",
                    seconds: 15,
                    aspectRatio: "9:16",
                    sourceKind: "FEED_PACKAGE.md",
                    group: "segment",
                },
            ],
        };
        const p4Pipeline = createManjuCanvasPackage(p4Package, [], { x: 1600, y: 0 });
        const legacyContext = {
            id: "legacy-context",
            type: CanvasNodeType.Text,
            title: "旧生产包说明",
            position: { x: 0, y: 0 },
            width: 320,
            height: 220,
            metadata: {
                content: "旧说明",
                manjuProjectId: "project-1",
                manjuStage: "p3_images" as const,
                manjuSourceArtifact: "DIRECTOR_STORYBOARD.md",
            },
        };
        const merged = mergeManjuCanvasPackage([...p3Pipeline.nodes, legacyContext], p3Pipeline.connections, p4Pipeline, "p4_videos");
        const styleNodes = merged.nodes.filter((node) => node.metadata?.manjuSharedStyleNode);

        expect(styleNodes).toHaveLength(1);
        expect(styleNodes[0]?.title).toBe("VisiDeep AI · 项目画风设定");
        expect(styleNodes[0]?.metadata?.content).toContain("【STYLE｜视觉风格 · 全片锁死】");
        expect(styleNodes[0]?.metadata?.content).toContain("【PERFORMANCE｜表演与动作 · 全片锁死】");
        expect(merged.nodes.some((node) => node.id === "legacy-context")).toBe(false);
    });

    test("uses the compiled strong image style block as the shared global style", () => {
        const pkg: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            style: {
                ...P3_PACKAGE.style,
                imageStylePrompt: "【STYLE｜视觉风格 · 全片锁死】\n成像底层：实拍胶片质感\n光比 [1:3, 1:6]",
                referenceAnalysis: "低调偏暗 · 高对比 · 偏暖 · 主色 #C88A52",
            },
        };
        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const styleNode = pipeline.nodes.find((node) => node.metadata?.manjuSharedStyleNode);

        expect(styleNode?.metadata?.content).toContain("【STYLE｜视觉风格 · 全片锁死】");
        expect(styleNode?.metadata?.content).toContain("光比 [1:3, 1:6]");
        expect(styleNode?.metadata?.content).toContain("风格参考图分析");
        const storyboardTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        expect(storyboardTable?.metadata?.batchTable?.rows.find((row) => row.manjuStoryboard?.taskId === "p3-storyboard-seg-1")?.prompt).toBe("故事板图提示词");
        expect(storyboardTable?.metadata?.manjuGlobalStylePrompt).toContain("【STYLE｜视觉风格 · 全片锁死】");
    });

    test("removes global style already embedded in an imported task prompt", () => {
        const globalStyle = "【STYLE｜视觉风格 · 全片锁死】\n光比 [1:3, 1:6]";
        const pkg: ManjuCanvasPackage = {
            ...P3_PACKAGE,
            style: {
                ...P3_PACKAGE.style,
                imageStylePrompt: globalStyle,
            },
            tasks: [
                {
                    ...P3_PACKAGE.tasks[0],
                    prompt: `${globalStyle}\n\n【本任务｜在全局风格锁内执行】\n故事板图提示词`,
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const styleNode = pipeline.nodes.find((node) => node.metadata?.manjuSharedStyleNode);
        const storyboardTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
        const storyboardRow = storyboardTable?.metadata?.batchTable?.rows.find((row) => row.manjuStoryboard?.taskId === "p3-storyboard-seg-1");

        expect(styleNode?.metadata?.content).toContain(globalStyle);
        expect(storyboardRow?.prompt).toBe("故事板图提示词");
        expect(storyboardTable?.metadata?.manjuGlobalStylePrompt).toBe(globalStyle);
    });

    test("imports the style reference image and connects it to P2 asset tasks", () => {
        const pkg: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-1",
            stage: "p2_assets",
            assetRunId: "run-p2",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "p2-style-reference",
            style: {
                cardId: "sd-style-012",
                cardName: "袁和平",
                styleBase: "live-action",
                aspectRatio: "16:9",
                promptParameters: "真实发力 · 青褐大地色",
                imageStylePrompt: "【STYLE｜视觉风格 · 全片锁死】\n影调区间：光比 [1:3, 1:6]",
                referenceResourceId: "style-image-1",
                referenceAssetId: "style-asset-1",
                referenceStorageKey: "resource:style-image-1",
                referenceAnalysis: "低调偏暗 · 高反差 · 偏暖",
            },
            tasks: [
                {
                    id: "p2-character-1",
                    kind: "image",
                    code: "CHR-01",
                    assetCode: "CHR-01",
                    title: "角色四格",
                    prompt: "角色四格提示词",
                    category: "character",
                    sourceKind: "ASSET_DELIVERY_M1",
                },
            ],
        };
        const pipeline = createManjuCanvasPackage(pkg, [], { x: 0, y: 0 });
        const styleReference = pipeline.nodes.find((node) => node.metadata?.manjuSharedStyleReference);
        const assetTable = pipeline.nodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);

        expect(styleReference?.metadata).toMatchObject({
            storageKey: "resource:style-image-1",
            assetId: "style-asset-1",
            manjuStyleReferenceAnalysis: "低调偏暗 · 高反差 · 偏暖",
        });
        expect(styleReference?.metadata?.prompt).toContain("不复刻构图");
        expect(assetTable?.metadata?.manjuGlobalStylePrompt).toContain("【STYLE｜视觉风格 · 全片锁死】");
        expect(assetTable?.metadata?.batchTable?.rows[0]?.manjuAsset?.code).toBe("CHR-01");
    });

    test("image execution prepends global style once without leaking it into the visible prompt", () => {
        const style = "【STYLE｜视觉风格 · 全片锁死】\n光比 [1:3, 1:6]";
        expect(composeManjuImageExecutionPrompt("角色四格提示词", style)).toBe(`${style}\n\n【本任务｜在全局风格锁内执行】\n角色四格提示词`);
        expect(composeManjuImageExecutionPrompt(`${style}\n\n角色四格提示词`, style)).toContain("角色四格提示词");
        expect(composeManjuImageExecutionPrompt(`${style}\n\n角色四格提示词`, style).match(/【STYLE｜视觉风格 · 全片锁死】/g) || []).toHaveLength(1);
    });

    test("does not append the visible style text node when the task carries execution style metadata", () => {
        const styleNode = {
            id: "style-text",
            type: CanvasNodeType.Text,
            title: "全局强风格",
            position: { x: 0, y: 0 },
            width: 320,
            height: 200,
            metadata: { content: "【STYLE｜视觉风格 · 全片锁死】", manjuSharedStyleNode: true },
        };
        const styledTask = {
            id: "styled-task",
            type: CanvasNodeType.Image,
            title: "角色资产",
            position: { x: 400, y: 0 },
            width: 320,
            height: 320,
            metadata: { prompt: "角色四格提示词", manjuGlobalStylePrompt: "【STYLE｜视觉风格 · 全片锁死】" },
        };
        const connection = { id: "style-edge", fromNodeId: styleNode.id, toNodeId: styledTask.id };

        expect(getGenerationResourceNodes(styledTask.id, [styleNode, styledTask], [connection])).toEqual([]);
        expect(getGenerationResourceNodes(styledTask.id, [styleNode, { ...styledTask, metadata: { prompt: "旧角色四格提示词" } }], [connection]).map((node) => node.id)).toEqual(["style-text"]);
    });

    test("creates the global style reference from either resource id or storage key", () => {
        const basePackage: ManjuCanvasPackage = {
            version: 1,
            projectId: "project-style-reference",
            stage: "p2_assets",
            assetRunId: "run-style-reference",
            sourceArtifact: "ASSET_DELIVERY_M1.md",
            title: "P2 数字资产",
            hash: "style-reference",
            style: { referenceResourceId: "style-resource" },
            tasks: [
                { id: "p2-chr", kind: "image", code: "CHR-01", assetCode: "CHR-01", title: "角色", prompt: "角色提示词", category: "character", sourceKind: "ASSET_DELIVERY_M1" },
            ],
        };
        const byResourceId = createManjuCanvasPackage(basePackage, [], { x: 0, y: 0 }).nodes.find((node) => node.metadata?.manjuSharedStyleReference);
        expect(byResourceId?.metadata).toMatchObject({
            content: "/api/resources/style-resource/file",
            storageKey: "resource:style-resource",
        });
        expect(byResourceId?.metadata?.assetId).toBeUndefined();

        const byStorageKey = createManjuCanvasPackage({
            ...basePackage,
            style: { referenceStorageKey: "resource:style-from-key" },
        }, [], { x: 0, y: 0 }).nodes.find((node) => node.metadata?.manjuSharedStyleReference);
        expect(byStorageKey?.metadata).toMatchObject({
            content: "/api/resources/style-from-key/file",
            storageKey: "resource:style-from-key",
        });
        expect(byStorageKey?.metadata?.assetId).toBeUndefined();
    });
});

function overlappingCanvasNodes(nodes: CanvasNodeData[]) {
    const overlaps: Array<[string, string]> = [];
    for (let leftIndex = 0; leftIndex < nodes.length; leftIndex += 1) {
        for (let rightIndex = leftIndex + 1; rightIndex < nodes.length; rightIndex += 1) {
            const left = nodes[leftIndex];
            const right = nodes[rightIndex];
            const horizontal = Math.min(left.position.x + left.width, right.position.x + right.width) - Math.max(left.position.x, right.position.x);
            const vertical = Math.min(left.position.y + left.height, right.position.y + right.height) - Math.max(left.position.y, right.position.y);
            if (horizontal > 0 && vertical > 0) overlaps.push([left.title, right.title]);
        }
    }
    return overlaps;
}
