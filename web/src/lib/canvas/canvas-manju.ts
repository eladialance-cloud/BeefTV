import { nanoid } from "nanoid";

import { createCanvasNode } from "@/lib/canvas/canvas-project-domain";
import { cleanupManjuInternalConnections, isManjuAssetTable, isManjuStoryboardTable, manjuAssetKindForCategory, manjuAssetOutputHandleId, manjuReferenceCodeKey } from "@/lib/canvas/canvas-manju-asset-table";
import { generationBatchStatus } from "@/lib/canvas/canvas-generation-batch";
import { getCanvasNodesBounds, viewportForBounds, type CanvasViewportSize } from "@/lib/canvas/canvas-viewport";
import type { ManjuCanvasPackage, ManjuCanvasTask } from "@/services/api/manju";
import { CanvasNodeType, type CanvasBatchRow, type CanvasConnection, type CanvasNodeData, type CanvasNodeMetadata, type Position, type ViewportTransform } from "@/types/canvas";

export type ManjuCanvasReference = {
    code: string;
    title: string;
    content?: string;
    storageKey?: string;
    taskId?: string;
    assetCode?: string;
    assetId?: string;
    category?: string;
    prompt?: string;
};

export type ManjuCanvasPackageImport = {
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    contextNodeId: string;
    styleReferenceNodeId?: string;
    referenceNodeIds: string[];
    referenceNodeIdByCode: Map<string, string>;
    taskNodeIds: Map<string, string>;
};

export type ManjuCanvasPackageMerge = {
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    addedTaskCount: number;
};

export function manjuGenerationTaskMetadata(metadata: CanvasNodeMetadata | undefined): Partial<CanvasNodeMetadata> {
    if (!metadata) return {};
    return {
        ...(metadata.manjuProjectId ? { manjuProjectId: metadata.manjuProjectId } : {}),
        ...(metadata.manjuStage ? { manjuStage: metadata.manjuStage } : {}),
        ...(metadata.manjuAssetTableId ? { manjuAssetTableId: metadata.manjuAssetTableId } : {}),
        ...(metadata.manjuAssetRowId ? { manjuAssetRowId: metadata.manjuAssetRowId } : {}),
        ...(metadata.manjuAssetCode ? { manjuAssetCode: metadata.manjuAssetCode } : {}),
        ...(metadata.manjuTaskId ? { manjuTaskId: metadata.manjuTaskId } : {}),
        ...(metadata.manjuTaskCode ? { manjuTaskCode: metadata.manjuTaskCode } : {}),
    };
}

const MANJU_VISUAL_REFERENCE_PATTERN = /\b(?:CHR|CNH|SCN|PRP)-[^\s|,，、:：)\]}]+/g;
const MANJU_STYLE_EXECUTION_SEPARATOR = "【本任务｜在全局风格锁内执行】";

export function normalizeManjuVisualReferenceCodes(codes: string[]): string[] {
    return unique(codes.flatMap((code) =>
        (code.match(MANJU_VISUAL_REFERENCE_PATTERN) || []).map(manjuReferenceCodeKey),
    ));
}

export function manjuAssetNameFromCode(code: string) {
    const normalized = code.trim();
    const name = normalized.replace(/^(?:CHR|SCN|PRP|CNH)-/i, "").trim();
    return name || normalized;
}

export function composeManjuVideoExecutionPrompt(segmentPrompt: string, globalStylePrompt?: string): string {
    const segment = segmentPrompt.trim();
    const globalStyle = (globalStylePrompt || "").trim();
    if (!globalStyle) return segment;
    if (!segment) return globalStyle;
    return `${globalStyle}\n\n${segment}`;
}

export function composeManjuImageExecutionPrompt(taskPrompt: string, globalStylePrompt?: string): string {
    const task = taskPrompt.trim();
    const globalStyle = (globalStylePrompt || "").trim();
    if (!globalStyle || task.includes("【STYLE｜视觉风格 · 全片锁死】")) return task;
    if (!task) return globalStyle;
    return `${globalStyle}\n\n【本任务｜在全局风格锁内执行】\n${task}`;
}

export function manjuTaskReferenceCodes(task: ManjuCanvasTask): string[] {
    return task.kind === "video" ? unique(task.referenceCodes || []) : normalizeManjuVisualReferenceCodes(task.referenceCodes || []);
}

export function mergeManjuCanvasPackage(existingNodes: CanvasNodeData[], existingConnections: CanvasConnection[], imported: ManjuCanvasPackageImport, stage: ManjuCanvasPackage["stage"]): ManjuCanvasPackageMerge {
    const incomingAssetTable = imported.nodes.find(isManjuAssetTable);
    const incomingStoryboardTable = imported.nodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
    const existingAssetTable = existingNodes.find((node) => node.metadata?.batchTable?.manjuAssetTable);
    const existingStoryboardTable = existingNodes.find((node) => node.metadata?.batchTable?.manjuStoryboardTable);
    const incomingTasks = new Map(
        imported.nodes.flatMap((node) => {
            const taskId = node.metadata?.manjuTaskId;
            return taskId && node.metadata?.manjuStage === stage ? [[taskId, node] as const] : [];
        }),
    );
    const incomingReferenceCodes = new Set(imported.nodes.flatMap((node) => (node.metadata?.manjuStage === "p2_assets" ? normalizeManjuVisualReferenceCodes([node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || ""]) : [])));
    incomingAssetTable?.metadata?.batchTable?.rows.forEach((row) => {
        if (row.manjuAsset?.code) incomingReferenceCodes.add(row.manjuAsset.code);
    });
    incomingStoryboardTable?.metadata?.batchTable?.rows.forEach((row) => {
        if (row.manjuStoryboard?.code) incomingReferenceCodes.add(row.manjuStoryboard.code);
    });
    const incomingStyleNode = imported.nodes.find((node) => node.metadata?.manjuSharedStyleNode);
    const incomingStyleReferenceNode = imported.nodes.find((node) => node.metadata?.manjuSharedStyleReference);
    const selectedByTask = new Map<string, number>();
    const normalizedNodes: CanvasNodeData[] = [];
    let sharedStyleNode: CanvasNodeData | undefined;
    let sharedStyleReferenceNode: CanvasNodeData | undefined;
    const absorbedP2Nodes = new Map<string, CanvasNodeData>();
    const absorbedP3Nodes = new Map<string, CanvasNodeData>();
    for (const node of existingNodes) {
        if (incomingAssetTable && node.metadata?.batchTable?.manjuAssetTable) continue;
        if (incomingStoryboardTable && node.metadata?.batchTable?.manjuStoryboardTable) continue;
        if (isObsoleteManjuReferencePlaceholder(node, stage, incomingReferenceCodes)) continue;
        if (isLegacyManjuPackageContext(node)) continue;
        const p2Code = node.metadata?.manjuStage === "p2_assets" ? normalizeManjuVisualReferenceCodes([node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || ""])[0] : undefined;
        if (incomingAssetTable && p2Code && incomingReferenceCodes.has(p2Code)) {
            absorbedP2Nodes.set(node.id, node);
            continue;
        }
        const incomingStoryboardRow = node.metadata?.manjuStage === "p3_images" ? incomingStoryboardTable?.metadata?.batchTable?.rows.find((row) => row.manjuStoryboard?.taskId === node.metadata?.manjuTaskId) : undefined;
        const p3Code = node.metadata?.manjuStage === "p3_images" ? node.metadata?.manjuTaskCode?.trim() || incomingStoryboardRow?.manjuStoryboard?.code || "" : "";
        if (incomingStoryboardTable && p3Code && incomingReferenceCodes.has(p3Code)) {
            absorbedP3Nodes.set(node.id, node);
            continue;
        }
        if (node.metadata?.manjuSharedStyleNode) {
            sharedStyleNode = incomingStyleNode ? withIncomingSharedStyleNode(node, incomingStyleNode) : node;
            normalizedNodes.push(sharedStyleNode);
            continue;
        }
        if (node.metadata?.manjuSharedStyleReference) {
            sharedStyleReferenceNode = incomingStyleReferenceNode ? withIncomingSharedStyleNode(node, incomingStyleReferenceNode) : node;
            normalizedNodes.push(sharedStyleReferenceNode);
            continue;
        }
        const taskId = node.metadata?.manjuTaskId;
        if (!taskId || node.metadata?.manjuStage !== stage) {
            normalizedNodes.push(node);
            continue;
        }
        const incoming = incomingTasks.get(taskId);
        if (!incoming) {
            if (nodeHasMedia(node)) normalizedNodes.push(node);
            continue;
        }
        const candidate = withIncomingManjuPrompt(node, incoming);
        const selectedIndex = selectedByTask.get(taskId);
        if (selectedIndex === undefined) {
            selectedByTask.set(taskId, normalizedNodes.length);
            normalizedNodes.push(candidate);
            continue;
        }
        const selected = normalizedNodes[selectedIndex];
        if (!nodeHasMedia(selected) && nodeHasMedia(candidate)) normalizedNodes[selectedIndex] = candidate;
    }
    const mergedAssetTable = incomingAssetTable ? mergeManjuAssetTable(existingAssetTable, incomingAssetTable, absorbedP2Nodes) : existingAssetTable;
    if (mergedAssetTable) normalizedNodes.push(mergedAssetTable);
    const mergedStoryboardTable = incomingStoryboardTable ? mergeManjuStoryboardTable(existingStoryboardTable, incomingStoryboardTable, absorbedP3Nodes) : existingStoryboardTable;
    if (mergedStoryboardTable) normalizedNodes.push(mergedStoryboardTable);
    const absorbedBackingNodes = [...absorbedP2Nodes.entries()].flatMap(([nodeId, node]) => {
        const code = normalizeManjuVisualReferenceCodes([node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || ""])[0];
        const row = mergedAssetTable?.metadata?.batchTable?.rows.find((item) => item.manjuAsset?.code === code);
        if (!row) return [];
        absorbedP2Nodes.set(nodeId, {
            ...node,
            metadata: {
                ...node.metadata,
                manjuAssetTableId: mergedAssetTable?.id,
                manjuAssetRowId: row.id,
            },
        });
        return [absorbedP2Nodes.get(nodeId)!];
    });
    const absorbedStoryboardBackingNodes = [...absorbedP3Nodes.entries()].flatMap(([nodeId, node]) => {
        const code = node.metadata?.manjuTaskCode?.trim() || "";
        const row = mergedStoryboardTable?.metadata?.batchTable?.rows.find((item) => item.manjuStoryboard?.code === code);
        if (!row) return [];
        const backing = {
            ...node,
            metadata: {
                ...node.metadata,
                manjuAssetTableId: mergedStoryboardTable?.id,
                manjuAssetRowId: row.id,
            },
        };
        absorbedP3Nodes.set(nodeId, backing);
        return [backing];
    });

    const byTask = new Map<string, CanvasNodeData>();
    const p2AssetNodes = new Map<string, CanvasNodeData>();
    for (const node of normalizedNodes) {
        if (node.metadata?.manjuTaskId) byTask.set(node.metadata.manjuTaskId, node);
        if (node.metadata?.manjuStage === "p2_assets" && node.metadata.manjuAssetCode) p2AssetNodes.set(node.metadata.manjuAssetCode, node);
    }

    const nodeIdMap = new Map<string, string>();
    const addedNodes: CanvasNodeData[] = [];
    let addedTaskCount = 0;
    for (const node of imported.nodes) {
        if (isManjuAssetTable(node)) {
            if (mergedAssetTable) nodeIdMap.set(node.id, mergedAssetTable.id);
            continue;
        }
        if (isManjuStoryboardTable(node)) {
            if (mergedStoryboardTable) nodeIdMap.set(node.id, mergedStoryboardTable.id);
            continue;
        }
        if (node.metadata?.manjuStage === "p3_images" && node.metadata?.manjuTaskCode && mergedStoryboardTable) {
            const row = mergedStoryboardTable.metadata?.batchTable?.rows.find((item) => item.manjuStoryboard?.code === node.metadata?.manjuTaskCode);
            if (row?.outputNodeId) {
                nodeIdMap.set(node.id, row.outputNodeId);
                continue;
            }
        }
        if (node.metadata?.manjuAssetTableId && (mergedAssetTable || mergedStoryboardTable)) {
            const storyboardBacking = Boolean(node.metadata?.manjuTaskCode && mergedStoryboardTable);
            const targetTable = storyboardBacking ? mergedStoryboardTable : mergedAssetTable;
            const row = targetTable?.metadata?.batchTable?.rows.find(
                (item) =>
                    item.id === node.metadata?.manjuAssetRowId || (node.metadata?.manjuAssetCode && item.manjuAsset?.code === node.metadata.manjuAssetCode) || (node.metadata?.manjuTaskCode && item.manjuStoryboard?.code === node.metadata.manjuTaskCode),
            );
            if (row?.outputNodeId) {
                nodeIdMap.set(node.id, row.outputNodeId);
                continue;
            }
            node.metadata = {
                ...node.metadata,
                manjuAssetTableId: targetTable?.id,
                manjuAssetRowId: row?.id || node.metadata.manjuAssetRowId,
            };
        }
        let duplicate = node.metadata?.manjuTaskId ? byTask.get(node.metadata.manjuTaskId) : undefined;
        if (!duplicate && node.metadata?.manjuStage === "p2_assets" && node.metadata.manjuAssetCode) {
            duplicate = p2AssetNodes.get(node.metadata.manjuAssetCode);
        }
        if (!duplicate && node.metadata?.manjuSharedStyleNode) duplicate = sharedStyleNode;
        if (!duplicate && node.metadata?.manjuSharedStyleReference) duplicate = sharedStyleReferenceNode;
        if (duplicate) {
            nodeIdMap.set(node.id, duplicate.id);
            continue;
        }
        nodeIdMap.set(node.id, node.id);
        addedNodes.push(node);
        if (node.metadata?.manjuStage === stage && node.metadata.manjuTaskId) addedTaskCount += 1;
    }

    const nodes = [...normalizedNodes, ...absorbedBackingNodes, ...absorbedStoryboardBackingNodes, ...addedNodes];
    const nodeIds = new Set(nodes.map((node) => node.id));
    const absorbedRowByNodeId = new Map(
        [...absorbedP2Nodes.entries()].flatMap(([nodeId, node]) => {
            const code = normalizeManjuVisualReferenceCodes([node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || ""])[0];
            const row = mergedAssetTable?.metadata?.batchTable?.rows.find((item) => item.manjuAsset?.code === code);
            return row ? [[nodeId, row] as const] : [];
        }),
    );
    const absorbedStoryboardRowByNodeId = new Map(
        [...absorbedP3Nodes.entries()].flatMap(([nodeId, node]) => {
            const code = node.metadata?.manjuTaskCode?.trim() || "";
            const row = mergedStoryboardTable?.metadata?.batchTable?.rows.find((item) => item.manjuStoryboard?.code === code);
            return row ? [[nodeId, row] as const] : [];
        }),
    );
    const retainedExistingConnections = existingConnections
        .filter((item) => nodeIds.has(item.fromNodeId) || absorbedRowByNodeId.has(item.fromNodeId) || absorbedStoryboardRowByNodeId.has(item.fromNodeId))
        .filter((item) => nodeIds.has(item.toNodeId))
        .map((item) => {
            const row = absorbedRowByNodeId.get(item.fromNodeId);
            const storyboardRow = absorbedStoryboardRowByNodeId.get(item.fromNodeId);
            if (storyboardRow && mergedStoryboardTable) {
                return {
                    ...item,
                    fromNodeId: mergedStoryboardTable.id,
                    fromHandleId: manjuAssetOutputHandleId(storyboardRow.id),
                };
            }
            if (!row || !mergedAssetTable) return item;
            return {
                ...item,
                fromNodeId: mergedAssetTable.id,
                fromHandleId: manjuAssetOutputHandleId(row.id),
            };
        });
    const connectionKeys = new Set(retainedExistingConnections.map(connectionKey));
    const connections = [...retainedExistingConnections];
    for (const connection of imported.connections) {
        const mapped = {
            ...connection,
            fromNodeId: nodeIdMap.get(connection.fromNodeId) || connection.fromNodeId,
            toNodeId: nodeIdMap.get(connection.toNodeId) || connection.toNodeId,
        };
        if (!nodeIds.has(mapped.fromNodeId) || !nodeIds.has(mapped.toNodeId)) continue;
        const key = connectionKey(mapped);
        if (connectionKeys.has(key)) continue;
        connectionKeys.add(key);
        connections.push(mapped);
    }
    if (stage === "p2_assets" && incomingAssetTable) {
        addedTaskCount = incomingAssetTable.metadata?.batchTable?.rows.length || 0;
    }
    if (stage === "p3_images" && incomingStoryboardTable) {
        addedTaskCount = incomingStoryboardTable.metadata?.batchTable?.rows.length || 0;
    }
    return { nodes, connections: cleanupManjuInternalConnections(nodes, connections), addedTaskCount };
}

function mergeManjuAssetTable(existing: CanvasNodeData | undefined, incoming: CanvasNodeData, absorbedNodes: Map<string, CanvasNodeData>) {
    const absorbedByCode = new Map<string, CanvasNodeData>();
    absorbedNodes.forEach((node) => {
        const code = normalizeManjuVisualReferenceCodes([node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || ""])[0];
        if (code) absorbedByCode.set(manjuReferenceCodeKey(code), node);
    });
    const incomingRows = incoming.metadata?.batchTable?.rows || [];
    const incomingIdentities = new Set(incomingRows.map(manjuAssetRowIdentity).filter(Boolean));
    const existingRows = dedupeManjuAssetRows((existing?.metadata?.batchTable?.rows || []).filter((row) => {
        return Boolean(row.manjuAsset?.taskId.startsWith("manual:asset:") || incomingIdentities.has(manjuAssetRowIdentity(row)));
    }));
    const findPrevious = (row: CanvasBatchRow) => {
        return existingRows.find((item) => {
            return Boolean(
                (row.manjuAsset?.taskId && item.manjuAsset?.taskId === row.manjuAsset.taskId)
                || manjuAssetRowIdentity(item) === manjuAssetRowIdentity(row),
            );
        });
    };
    const mergeRow = (row: CanvasBatchRow, previous?: CanvasBatchRow) => {
        const code = row.manjuAsset?.code || previous?.manjuAsset?.code || "";
        const absorbed = absorbedByCode.get(manjuReferenceCodeKey(code));
        const absorbedHasMedia = Boolean(absorbed?.metadata?.content || absorbed?.metadata?.storageKey || absorbed?.metadata?.assetId);
        const incomingTaskId = row.manjuAsset?.taskId || "";
        const taskId = incomingTaskId.startsWith("asset:") && previous?.manjuAsset?.taskId
            ? previous.manjuAsset.taskId
            : incomingTaskId || previous?.manjuAsset?.taskId || "";
        const incomingName = row.manjuAsset?.name?.trim() || "";
        const previousName = previous?.manjuAsset?.name?.trim() || "";
        const name = incomingName && incomingName !== code && !incomingName.includes("待补参考")
            ? incomingName
            : previousName || absorbed?.metadata?.workflowTitle || manjuAssetNameFromCode(code);
        return {
            ...row,
            id: previous?.id || row.id,
            inputNodeIds: previous?.inputNodeIds?.length ? previous.inputNodeIds : absorbedHasMedia && absorbed ? [absorbed.id] : row.inputNodeIds,
            outputNodeId: absorbedHasMedia && absorbed ? absorbed.id : previous?.outputNodeId || absorbed?.id || row.outputNodeId,
            prompt: row.prompt || previous?.prompt || absorbed?.metadata?.prompt || absorbed?.metadata?.composerContent || "",
            enabled: previous?.enabled ?? row.enabled,
            manjuGeneration: previous?.manjuGeneration || row.manjuGeneration,
            manjuAsset: {
                ...previous?.manjuAsset,
                ...row.manjuAsset,
                taskId,
                code,
                kind: row.manjuAsset?.kind || previous?.manjuAsset?.kind || manjuAssetKindForCategory(),
                name,
            },
        };
    };
    const incomingByPrevious = new Map<CanvasBatchRow, CanvasBatchRow>();
    const incomingWithoutPrevious: CanvasBatchRow[] = [];
    incomingRows.forEach((row) => {
        const previous = findPrevious(row);
        if (previous) incomingByPrevious.set(previous, row);
        else incomingWithoutPrevious.push(row);
    });
    const rows = [
        ...existingRows.map((previous) => {
            const incomingRow = incomingByPrevious.get(previous);
            return incomingRow ? mergeRow(incomingRow, previous) : previous;
        }),
        ...incomingWithoutPrevious.map((row) => mergeRow(row)),
    ];
    const base = existing || incoming;
    return {
        ...base,
        title: incoming.title || base.title,
        position: existing?.position || incoming.position,
        width: existing?.width || incoming.width,
        height: existing?.height || incoming.height,
        metadata: {
            ...base.metadata,
            ...incoming.metadata,
            batchTable: {
                ...(base.metadata?.batchTable || incoming.metadata?.batchTable),
                ...(incoming.metadata?.batchTable || {}),
                manjuGenerationDefaults: existing?.metadata?.batchTable?.manjuGenerationDefaults || incoming.metadata?.batchTable?.manjuGenerationDefaults,
                manjuCategoryGeneration: {
                    ...(incoming.metadata?.batchTable?.manjuCategoryGeneration || {}),
                    ...(existing?.metadata?.batchTable?.manjuCategoryGeneration || {}),
                },
                rows,
            },
        },
    } as CanvasNodeData;
}

function dedupeManjuAssetRows(rows: CanvasBatchRow[]) {
    const byCode = new Map<string, CanvasBatchRow>();
    const manualRowsWithoutCode: CanvasBatchRow[] = [];
    const score = (row: CanvasBatchRow) =>
        (row.prompt.trim() ? 8 : 0)
        + (row.outputNodeId ? 4 : 0)
        + (row.inputNodeIds.length ? 2 : 0)
        + (row.manjuAsset?.name && row.manjuAsset.name !== row.manjuAsset.code ? 1 : 0);
    rows.forEach((row) => {
        const identity = manjuAssetRowIdentity(row);
        if (!identity) {
            if (row.manjuAsset?.taskId.startsWith("manual:asset:")) manualRowsWithoutCode.push(row);
            return;
        }
        const previous = byCode.get(identity);
        if (!previous) {
            byCode.set(identity, row);
            return;
        }
        const preferred = score(row) > score(previous) ? row : previous;
        const fallback = preferred === row ? previous : row;
        const code = preferred.manjuAsset?.code || fallback.manjuAsset?.code || "";
        byCode.set(identity, {
            ...fallback,
            ...preferred,
            inputNodeIds: unique([...(preferred.inputNodeIds || []), ...(fallback.inputNodeIds || [])]),
            outputNodeId: preferred.outputNodeId || fallback.outputNodeId,
            prompt: preferred.prompt || fallback.prompt,
            manjuGeneration: preferred.manjuGeneration || fallback.manjuGeneration,
            manjuAsset: {
                ...fallback.manjuAsset,
                ...preferred.manjuAsset,
                taskId: preferred.manjuAsset?.taskId || fallback.manjuAsset?.taskId || "",
                code,
                kind: preferred.manjuAsset?.kind || fallback.manjuAsset?.kind || manjuAssetKindForCategory(),
                name: preferred.manjuAsset?.name || fallback.manjuAsset?.name || manjuAssetNameFromCode(code),
            },
        });
    });
    return [...byCode.values(), ...manualRowsWithoutCode];
}

function manjuAssetRowIdentity(row: CanvasBatchRow) {
    const code = manjuReferenceCodeKey(row.manjuAsset?.code || "");
    if (!code) return "";
    const taskCode = row.manjuAsset?.taskCode?.trim() || "";
    return taskCode ? `${code}\u0000${taskCode}` : code;
}

function mergeManjuStoryboardTable(existing: CanvasNodeData | undefined, incoming: CanvasNodeData, absorbedNodes: Map<string, CanvasNodeData>) {
    const absorbedByCode = new Map<string, CanvasNodeData>();
    const absorbedByTaskId = new Map<string, CanvasNodeData>();
    absorbedNodes.forEach((node) => {
        const code = node.metadata?.manjuTaskCode?.trim() || "";
        if (code) absorbedByCode.set(code, node);
        if (node.metadata?.manjuTaskId) absorbedByTaskId.set(node.metadata.manjuTaskId, node);
    });
    const existingRows = existing?.metadata?.batchTable?.rows || [];
    const rows = (incoming.metadata?.batchTable?.rows || []).map((row) => {
        const code = row.manjuStoryboard?.code || "";
        const previous = existingRows.find((item) => item.manjuStoryboard?.code === code || (row.manjuStoryboard?.taskId && item.manjuStoryboard?.taskId === row.manjuStoryboard.taskId));
        const absorbed = absorbedByCode.get(code) || absorbedByTaskId.get(row.manjuStoryboard?.taskId || "");
        const absorbedHasMedia = Boolean(absorbed?.metadata?.content || absorbed?.metadata?.storageKey || absorbed?.metadata?.assetId);
        return {
            ...row,
            inputNodeIds: previous?.inputNodeIds?.length ? previous.inputNodeIds : absorbedHasMedia && absorbed ? [absorbed.id] : row.inputNodeIds,
            outputNodeId: absorbedHasMedia && absorbed ? absorbed.id : previous?.outputNodeId || absorbed?.id || row.outputNodeId,
            prompt: row.prompt || previous?.prompt || "",
            enabled: previous?.enabled ?? row.enabled,
            manjuGeneration: previous?.manjuGeneration || row.manjuGeneration,
        };
    });
    const preservedManualRows = (existingRows || []).filter((row) =>
        row.manjuStoryboard?.taskId.startsWith("manual:storyboard:")
        && !rows.some((candidate) =>
            candidate.id === row.id
            || candidate.manjuStoryboard?.taskId === row.manjuStoryboard?.taskId
            || (candidate.manjuStoryboard?.code && candidate.manjuStoryboard.code === row.manjuStoryboard?.code),
        ),
    );
    const base = existing || incoming;
    return {
        ...base,
        title: incoming.title || base.title,
        position: existing?.position || incoming.position,
        width: existing?.width || incoming.width,
        height: existing?.height || incoming.height,
        metadata: {
            ...base.metadata,
            ...incoming.metadata,
            batchTable: {
                ...(base.metadata?.batchTable || incoming.metadata?.batchTable),
                ...(incoming.metadata?.batchTable || {}),
                manjuGenerationDefaults: existing?.metadata?.batchTable?.manjuGenerationDefaults || incoming.metadata?.batchTable?.manjuGenerationDefaults,
                rows: [...rows, ...preservedManualRows],
            },
        },
    } as CanvasNodeData;
}

function isLegacyManjuPackageContext(node: CanvasNodeData) {
    return node.type === CanvasNodeType.Text && Boolean(node.metadata?.manjuProjectId) && Boolean(node.metadata?.manjuStage) && Boolean(node.metadata?.manjuSourceArtifact) && !node.metadata?.manjuTaskId && !node.metadata?.manjuSharedStyleNode;
}

function isObsoleteManjuReferencePlaceholder(node: CanvasNodeData, stage: ManjuCanvasPackage["stage"], incomingCodes: Set<string>) {
    if (stage === "p2_assets" || node.metadata?.manjuStage !== "p2_assets" || node.metadata?.manjuTaskId || nodeHasMedia(node)) {
        return false;
    }
    return Boolean(node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || incomingCodes.size);
}

function withIncomingSharedStyleNode(node: CanvasNodeData, incoming: CanvasNodeData): CanvasNodeData {
    return {
        ...node,
        title: incoming.title || node.title,
        metadata: {
            ...node.metadata,
            ...incoming.metadata,
            ...(incoming.metadata?.manjuSharedStyleNode ? { locked: false } : {}),
        },
    };
}

function nodeHasMedia(node: CanvasNodeData) {
    return Boolean(node.metadata?.content || node.metadata?.storageKey || node.metadata?.assetId);
}

function withIncomingManjuPrompt(node: CanvasNodeData, incoming: CanvasNodeData): CanvasNodeData {
    const prompt = incoming.metadata?.prompt || incoming.metadata?.composerContent || "";
    if (!prompt) return node;
    const metadata: CanvasNodeMetadata = {
        ...node.metadata,
        prompt,
        composerContent: incoming.metadata?.composerContent || prompt,
        workflowTitle: incoming.metadata?.workflowTitle || node.metadata?.workflowTitle,
        workflowDescription: incoming.metadata?.workflowDescription || node.metadata?.workflowDescription,
        generationMode: incoming.metadata?.generationMode || node.metadata?.generationMode,
        generationType: incoming.metadata?.generationType || node.metadata?.generationType,
        size: incoming.metadata?.size || node.metadata?.size,
        videoEditOperation: incoming.metadata?.videoEditOperation || node.metadata?.videoEditOperation,
        seconds: incoming.metadata?.seconds || node.metadata?.seconds,
        assetTags: incoming.metadata?.assetTags || node.metadata?.assetTags,
        manjuPackageHash: incoming.metadata?.manjuPackageHash || node.metadata?.manjuPackageHash,
        manjuReferenceCodes: incoming.metadata?.manjuReferenceCodes || node.metadata?.manjuReferenceCodes,
        manjuGlobalStylePrompt: incoming.metadata?.manjuGlobalStylePrompt || node.metadata?.manjuGlobalStylePrompt,
        manjuSourceArtifact: incoming.metadata?.manjuSourceArtifact || node.metadata?.manjuSourceArtifact,
    };
    return {
        ...node,
        title: incoming.title || node.title,
        metadata,
    };
}

export function nextManjuCanvasCenter(nodes: CanvasNodeData[]): Position {
    if (!nodes.length) return { x: 0, y: 0 };
    const minY = Math.min(...nodes.map((node) => node.position.y));
    const maxX = Math.max(...nodes.map((node) => node.position.x + node.width));
    return { x: maxX + MANJU_LAYOUT_GAP, y: minY };
}

export function manjuCanvasViewportForNodes(nodes: CanvasNodeData[], viewportSize: CanvasViewportSize): ViewportTransform | undefined {
    const bounds = getCanvasNodesBounds(nodes);
    if (!bounds) return undefined;
    return viewportForBounds(bounds, viewportSize, { padding: 96, minScale: 0.2, maxScale: 0.9 });
}

export const MANJU_CANVAS_LAYOUT_VERSION = 2;
export const MANJU_ASSET_PROMPT_REPAIR_VERSION = 2;
const MANJU_LAYOUT_GAP = 64;
const MANJU_LAYOUT_COLUMNS = 3;

export function layoutManjuCanvasPackageNodes(nodes: CanvasNodeData[], origin: Position) {
    const groups = groupManjuCanvasLayoutNodes(nodes);
    if (!groups.size) return nodes;
    const replacements = new Map<string, CanvasNodeData>();
    let groupX = origin.x;
    groups.forEach((group) => {
        const laidOut = layoutManjuCanvasGroup(group, { x: groupX, y: origin.y });
        laidOut.forEach((node) => replacements.set(node.id, node));
        groupX = Math.max(...laidOut.map((node) => node.position.x + node.width)) + MANJU_LAYOUT_GAP;
    });
    return nodes.map((node) => replacements.get(node.id) || node);
}

export function normalizeManjuCanvasLayout(nodes: CanvasNodeData[]) {
    const groups = groupManjuCanvasLayoutNodes(nodes);
    if (!groups.size) return nodes;
    const replacements = new Map<string, CanvasNodeData>();
    groups.forEach((group) => {
        if (group.every((node) => (node.metadata?.manjuLayoutVersion || 0) >= MANJU_CANVAS_LAYOUT_VERSION)) return;
        const visible = group.filter(isVisibleManjuLayoutNode);
        if (!visible.length) return;
        const origin = {
            x: Math.min(...visible.map((node) => node.position.x)),
            y: Math.min(...visible.map((node) => node.position.y)),
        };
        layoutManjuCanvasGroup(group, origin).forEach((node) => replacements.set(node.id, node));
    });
    return replacements.size ? nodes.map((node) => replacements.get(node.id) || node) : nodes;
}

export function repairManjuAssetTablePrompts(nodes: CanvasNodeData[]): CanvasNodeData[] {
    const promptByRowId = new Map<string, string>();
    const promptByTaskId = new Map<string, string>();
    const promptByCode = new Map<string, string>();
    const assetTables = nodes.filter(isManjuAssetTable);
    if (!assetTables.length) return nodes;
    assetTables.forEach((table) => {
        (table.metadata?.batchTable?.rows || []).forEach((row) => {
            const prompt = row.prompt.trim();
            if (!prompt) return;
            promptByRowId.set(row.id, prompt);
            if (row.manjuAsset?.taskId) promptByTaskId.set(row.manjuAsset.taskId, prompt);
            const code = canonicalManjuAssetPromptCode(row.manjuAsset?.code);
            if (code) promptByCode.set(code, prompt);
        });
    });
    nodes.forEach((node) => {
        if (!isManjuAssetPromptSource(node)) return;
        const prompt = node.metadata?.prompt?.trim() || node.metadata?.composerContent?.trim() || "";
        if (!prompt) return;
        if (node.metadata?.manjuAssetRowId) promptByRowId.set(node.metadata.manjuAssetRowId, prompt);
        if (node.metadata?.manjuTaskId) promptByTaskId.set(node.metadata.manjuTaskId, prompt);
        const code = canonicalManjuAssetPromptCode(node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode);
        if (code) promptByCode.set(code, prompt);
    });
    let changed = false;
    const repaired = nodes.map((node): CanvasNodeData => {
        if (!isManjuAssetTable(node) || !node.metadata?.batchTable?.manjuAssetTable) return node;
        const tableData = node.metadata.batchTable;
        const meta = tableData.manjuAssetTable;
        if (!meta) return node;
        let tableChanged = (meta.promptRepairVersion || 0) < MANJU_ASSET_PROMPT_REPAIR_VERSION;
        const rows = tableData.rows.map((row) => {
            if (row.prompt.trim()) return row;
            const taskId = row.manjuAsset?.taskId || "";
            const code = canonicalManjuAssetPromptCode(row.manjuAsset?.code);
            const prompt = promptByRowId.get(row.id)
                || (taskId ? promptByTaskId.get(taskId) : "")
                || (code ? promptByCode.get(code) : "")
                || "";
            if (!prompt) return row;
            tableChanged = true;
            return { ...row, prompt };
        });
        if (!tableChanged) return node;
        changed = true;
        return {
            ...node,
            metadata: {
                ...node.metadata,
                batchTable: {
                    ...tableData,
                    rows,
                    manjuAssetTable: {
                        ...meta,
                        promptRepairVersion: MANJU_ASSET_PROMPT_REPAIR_VERSION,
                    },
                },
            },
        };
    });
    return changed ? repaired : nodes;
}

export function removeObsoleteManjuReferenceNodes(nodes: CanvasNodeData[]) {
    if (!nodes.some(isManjuAssetTable)) return nodes;
    const filtered = nodes.filter((node) => !(
        node.type === CanvasNodeType.Image
        && node.metadata?.manjuStage === "p2_assets"
        && !node.metadata?.manjuTaskId
        && !node.metadata?.manjuAssetTableId
        && !node.metadata?.content
        && !node.metadata?.storageKey
        && !node.metadata?.assetId
        && Boolean(node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode)
    ));
    return filtered.length === nodes.length ? nodes : filtered;
}

export function repairRecoveredManagedTableLinks(nodes: CanvasNodeData[]) {
    const recoveredByTaskId = new Map<string, CanvasNodeData>();
    const recoveredByRowId = new Map<string, CanvasNodeData>();
    const recoveredByAssetCode = new Map<string, CanvasNodeData>();
    nodes.forEach((node) => {
        const taskId = node.metadata?.taskId;
        if (taskId && node.metadata?.status === "success" && nodeHasMedia(node)) {
            recoveredByTaskId.set(taskId, node);
        }
        if (!nodeHasMedia(node)) return;
        if (node.metadata?.manjuAssetRowId) {
            recoveredByRowId.set(node.metadata.manjuAssetRowId, node);
        }
        const assetCode = node.metadata?.manjuAssetCode?.trim();
        if (assetCode) {
            recoveredByAssetCode.set(manjuReferenceCodeKey(assetCode), node);
        }
    });
    if (!recoveredByTaskId.size && !recoveredByRowId.size && !recoveredByAssetCode.size) return nodes;

    const recoveredMetadataByNodeId = new Map<string, CanvasNodeMetadata>();
    let changed = false;
    const repaired = nodes.map((node): CanvasNodeData => {
        const tableData = node.metadata?.batchTable;
        if (!tableData || (!tableData.manjuAssetTable && !tableData.manjuStoryboardTable)) return node;
        const rows = tableData.rows.map((row) => {
            const currentOutput = row.outputNodeId ? nodes.find((item) => item.id === row.outputNodeId) : undefined;
            if (currentOutput && nodeHasMedia(currentOutput)) return row;
            const batchItem = (node.metadata?.generationBatches || [])
                .flatMap((batch) => batch.items)
                .find((item) => item.rowId === row.id || (row.outputNodeId && item.nodeId === row.outputNodeId));
            const taskId = currentOutput?.metadata?.taskId || batchItem?.taskId || "";
            const assetCode = row.manjuAsset?.code ? manjuReferenceCodeKey(row.manjuAsset.code) : "";
            const recovered = recoveredByRowId.get(row.id)
                || (assetCode ? recoveredByAssetCode.get(assetCode) : undefined)
                || (taskId ? recoveredByTaskId.get(taskId) : undefined);
            if (!recovered || recovered.id === row.outputNodeId) return row;
            const code = row.manjuAsset?.code || row.manjuStoryboard?.code || "";
            recoveredMetadataByNodeId.set(recovered.id, {
                ...(recoveredMetadataByNodeId.get(recovered.id) || recovered.metadata),
                manjuAssetTableId: node.id,
                manjuAssetRowId: row.id,
                ...(row.manjuAsset
                    ? {
                          manjuStage: "p2_assets" as const,
                          manjuTaskId: row.manjuAsset.taskId,
                          manjuAssetCode: code,
                          manjuTaskCode: undefined,
                      }
                    : {
                          manjuStage: "p3_images" as const,
                          manjuTaskId: row.manjuStoryboard?.taskId,
                          manjuAssetCode: undefined,
                          manjuTaskCode: code,
                      }),
            });
            changed = true;
            return {
                ...row,
                outputNodeId: recovered.id,
                inputNodeIds: row.inputNodeIds.filter((id) => id !== row.outputNodeId && id !== recovered.id),
            };
        });
        const generationBatches = node.metadata?.generationBatches?.map((batch) => {
            let batchChanged = false;
            const items = batch.items.map((item) => {
                if (!item.taskId) return item;
                const recovered = recoveredByTaskId.get(item.taskId);
                if (!recovered || item.nodeId === recovered.id) return item;
                batchChanged = true;
                changed = true;
                return {
                    ...item,
                    nodeId: recovered.id,
                    status: "succeeded" as const,
                    errorDetails: undefined,
                    submissionUncertain: false,
                };
            });
            if (!batchChanged) return batch;
            const next = { ...batch, items, updatedAt: new Date().toISOString() };
            return { ...next, status: generationBatchStatus(next) };
        });
        return {
            ...node,
            metadata: {
                ...node.metadata,
                batchTable: { ...tableData, rows },
                ...(generationBatches ? { generationBatches } : {}),
            },
        };
    });
    if (!changed) return nodes;
    return repaired.map((node) => {
        const metadata = recoveredMetadataByNodeId.get(node.id);
        return metadata ? { ...node, metadata: { ...node.metadata, ...metadata } } : node;
    });
}

function isManjuAssetPromptSource(node: CanvasNodeData) {
    return Boolean(
        node.metadata?.manjuStage === "p2_assets"
        || node.metadata?.manjuAssetCode
        || node.metadata?.manjuAssetRowId,
    );
}

function canonicalManjuAssetPromptCode(code?: string) {
    const value = code?.trim() || "";
    return /^(?:CHR|SCN|PRP|CNH)-/.test(value) ? manjuReferenceCodeKey(value) : "";
}

export function repairManjuAssetNames(nodes: CanvasNodeData[]) {
    let changed = false;
    const repaired = nodes.map((node) => {
        if (!isManjuAssetTable(node)) return node;
        const tableData = node.metadata?.batchTable;
        if (!tableData?.rows?.length) return node;
        let nodeChanged = false;
        const rows = tableData.rows.map((row) => {
            const asset = row.manjuAsset;
            if (!asset?.code || !needsManjuAssetNameRepair(asset.name, asset.code)) return row;
            const name = manjuAssetNameFromCode(asset.code);
            if (!name || name === asset.name) return row;
            changed = true;
            nodeChanged = true;
            return { ...row, manjuAsset: { ...asset, name } };
        });
        if (!nodeChanged) return node;
        return {
            ...node,
            metadata: {
                ...node.metadata,
                batchTable: {
                    ...tableData,
                    rows,
                },
            },
        };
    });
    return changed ? repaired : nodes;
}

function needsManjuAssetNameRepair(name: string | undefined, code: string) {
    const normalized = String(name || "").trim();
    return !normalized || normalized === code || normalized.includes("待补参考") || normalized.includes(" · 待补");
}

function groupManjuCanvasLayoutNodes(nodes: CanvasNodeData[]) {
    const groups = new Map<string, CanvasNodeData[]>();
    nodes.forEach((node) => {
        const projectId = manjuCanvasProjectId(node);
        if (!projectId) return;
        const group = groups.get(projectId) || [];
        group.push(node);
        groups.set(projectId, group);
    });
    return groups;
}

function layoutManjuCanvasGroup(nodes: CanvasNodeData[], origin: Position) {
    const positions = new Map<string, Position>();
    let y = origin.y;
    const styleAndReferences = nodes
        .filter((node) => isVisibleManjuLayoutNode(node) && !isManjuAssetTable(node) && !isManjuStoryboardTable(node) && node.metadata?.manjuStage !== "p4_videos")
        .sort((left, right) => layoutStyleOrder(left) - layoutStyleOrder(right));
    if (styleAndReferences.length) {
        const row = layoutManjuNodeGrid(styleAndReferences, origin.x, y, MANJU_LAYOUT_COLUMNS);
        row.positions.forEach((position, nodeId) => positions.set(nodeId, position));
        y = row.bottom + MANJU_LAYOUT_GAP;
    }

    const tables = nodes.filter((node) => isManjuAssetTable(node) || isManjuStoryboardTable(node)).sort((left, right) => Number(isManjuStoryboardTable(left)) - Number(isManjuStoryboardTable(right)));
    tables.forEach((node) => {
        positions.set(node.id, { x: origin.x, y });
        y += node.height + MANJU_LAYOUT_GAP;
    });

    const tasks = nodes.filter((node) => isVisibleManjuLayoutNode(node) && node.metadata?.manjuStage === "p4_videos").sort((left, right) => taskOrderFromNode(left) - taskOrderFromNode(right));
    if (tasks.length) {
        const grid = layoutManjuNodeGrid(tasks, origin.x, y, MANJU_LAYOUT_COLUMNS);
        grid.positions.forEach((position, nodeId) => positions.set(nodeId, position));
    }

    const tablePositionById = new Map(tables.map((node) => [node.id, positions.get(node.id)!]));
    return nodes.map((node) => {
        const position = positions.get(node.id) || (node.metadata?.manjuAssetTableId ? tablePositionById.get(node.metadata.manjuAssetTableId) : undefined) || node.position;
        if (node.position.x === position.x && node.position.y === position.y && node.metadata?.manjuLayoutVersion === MANJU_CANVAS_LAYOUT_VERSION) return node;
        return {
            ...node,
            position,
            metadata: {
                ...node.metadata,
                manjuLayoutVersion: MANJU_CANVAS_LAYOUT_VERSION,
            },
        };
    });
}

function layoutManjuNodeGrid(nodes: CanvasNodeData[], x: number, y: number, columns: number) {
    const positions = new Map<string, Position>();
    const columnWidth = Math.max(...nodes.map((node) => node.width)) + MANJU_LAYOUT_GAP;
    let rowTop = y;
    for (let offset = 0; offset < nodes.length; offset += columns) {
        const row = nodes.slice(offset, offset + columns);
        row.forEach((node, index) => positions.set(node.id, { x: x + index * columnWidth, y: rowTop }));
        rowTop += Math.max(...row.map((node) => node.height)) + MANJU_LAYOUT_GAP;
    }
    return { positions, bottom: Math.max(y, rowTop - MANJU_LAYOUT_GAP) };
}

function layoutStyleOrder(node: CanvasNodeData) {
    if (node.metadata?.manjuSharedStyleNode) return 0;
    if (node.metadata?.manjuSharedStyleReference) return 1;
    return 2;
}

function taskOrderFromNode(node: CanvasNodeData) {
    const code = node.metadata?.manjuTaskCode || "";
    const match = code.match(/\d+/);
    return match ? Number(match[0]) : Number.MAX_SAFE_INTEGER;
}

function isVisibleManjuLayoutNode(node: CanvasNodeData) {
    return !node.metadata?.manjuAssetTableId;
}

function manjuCanvasProjectId(node: CanvasNodeData) {
    return node.metadata?.manjuProjectId
        || node.metadata?.batchTable?.manjuAssetTable?.projectId
        || node.metadata?.batchTable?.manjuStoryboardTable?.projectId;
}

export function createManjuCanvasPackage(pkg: ManjuCanvasPackage, references: ManjuCanvasReference[], center: Position): ManjuCanvasPackageImport {
    const referenceByCode = new Map(references.map((item) => [item.code, item]));
    const taskReferenceCodes = unique(pkg.tasks.flatMap(manjuTaskReferenceCodes));
    const assetTable = createManjuAssetTable(pkg, references, taskReferenceCodes, center);
    const storyboardTable = createManjuStoryboardTable(pkg, references, taskReferenceCodes, center);
    const tableCodes = new Set([...(assetTable?.rowByCode.keys() || []), ...(storyboardTable?.rowByCode.keys() || [])]);
    const referenceNodes = (pkg.stage === "p2_assets" ? taskReferenceCodes : [])
        .filter((code) => !tableCodes.has(code))
        .map((code, index) =>
            createReferenceNode(
                referenceByCode.get(code) || { code, title: manjuAssetNameFromCode(code) },
                {
                    x: center.x - 900,
                    y: center.y + index * 330,
                },
                pkg,
            ),
        );
    const referenceNodeIdByCode = new Map(referenceNodes.map((node) => [node.metadata?.manjuAssetCode || node.metadata?.manjuTaskCode || node.title, node.id]));

    const context = createCanvasNode(
        CanvasNodeType.Text,
        { x: center.x - 900, y: center.y - 520 },
        {
            content: styleContext(pkg),
            status: "success",
            fontSize: 16,
            workflowKind: "styleboard",
            workflowTitle: "项目画风设定",
            workflowDescription: "项目全片画风与表演锁定；阶段工件审计保留在节点元数据",
            manjuProjectId: pkg.projectId,
            manjuPackageHash: pkg.hash,
            manjuSourceArtifact: pkg.sourceArtifact,
            manjuSharedStyleNode: true,
            manjuGlobalStylePrompt: pkg.style?.globalStylePrompt,
            locked: false,
        },
    );
    context.title = "VisiDeep AI · 项目画风设定";

    const styleReference = pkg.style?.referenceResourceId || pkg.style?.referenceStorageKey ? createStyleReferenceNode(pkg, { x: center.x - 1480, y: center.y - 520 }) : undefined;

    const taskNodes = pkg.stage === "p2_assets" || pkg.stage === "p3_images" ? [] : [...pkg.tasks].sort((left, right) => taskOrder(left) - taskOrder(right)).map((task, index) => createTaskNode(task, pkg, index, center));

    const taskNodeIds = new Map<string, string>();
    taskNodes.forEach((node) => {
        const taskId = node.metadata?.manjuTaskId;
        if (taskId) taskNodeIds.set(taskId, node.id);
    });

    const connections: CanvasConnection[] = [];

    const nodes = [
            context,
            ...(styleReference ? [styleReference] : []),
            ...(assetTable ? [assetTable.node, ...assetTable.backingNodes] : []),
            ...(storyboardTable ? [storyboardTable.node, ...storyboardTable.backingNodes] : []),
            ...referenceNodes,
            ...taskNodes,
        ];
    return {
        nodes: layoutManjuCanvasPackageNodes(nodes, center),
        connections,
        contextNodeId: context.id,
        styleReferenceNodeId: styleReference?.id,
        referenceNodeIds: [...(assetTable?.backingNodes.map((node) => node.id) || []), ...(storyboardTable?.backingNodes.map((node) => node.id) || []), ...referenceNodes.map((node) => node.id)],
        referenceNodeIdByCode: new Map([...(assetTable?.rowByCode || []), ...(storyboardTable?.rowByCode || []), ...referenceNodeIdByCode]),
        taskNodeIds,
    };
}

function createManjuAssetTable(pkg: ManjuCanvasPackage, references: ManjuCanvasReference[], taskReferenceCodes: string[], center: Position) {
    const tableNodeId = `manju-asset-table-${nanoid()}`;
    const table = createCanvasNode(
        CanvasNodeType.BatchTable,
        { x: center.x - 560, y: center.y - 260 },
        {
            batchTable: {
                operation: "creative",
                concurrency: 5,
                rows: [],
                referenceColumns: [{ id: "manju-asset-input", label: "引入图" }],
                manjuAssetTable: {
                    version: 1,
                    projectId: pkg.projectId,
                    assetRunId: pkg.stage === "p2_assets" ? pkg.assetRunId : undefined,
                    sourceStage: "p2_assets",
                    promptRepairVersion: MANJU_ASSET_PROMPT_REPAIR_VERSION,
                },
                manjuGenerationDefaults: {
                    size: pkg.style?.aspectRatio,
                },
            },
            manjuGlobalStylePrompt: pkg.style?.globalStylePrompt || pkg.style?.imageStylePrompt,
        },
    );
    table.id = tableNodeId;
    table.title = "漫剧数字资产表";
    table.width = 1280;
    table.height = 720;

    const candidates = new Map<
        string,
        {
            code: string;
            taskCode?: string;
            title: string;
            kind: "character" | "scene" | "prop";
            taskId: string;
            prompt: string;
            reference?: ManjuCanvasReference;
        }
    >();
    if (pkg.stage === "p2_assets") {
        pkg.tasks.forEach((task) => {
            const code = manjuReferenceCodeKey((task.assetCode || task.code).trim());
            if (!/^(CHR|SCN|PRP)-/.test(code)) return;
            const taskCode = task.code.trim();
            candidates.set(`task:${task.id || taskCode}`, {
                code,
                taskCode,
                title: task.title,
                kind: manjuAssetKindForCategory(task.category || task.group),
                taskId: task.id,
                prompt: task.prompt,
            });
        });
    } else {
        const referenceByCode = new Map(references.map((item) => [manjuReferenceCodeKey(item.code), item]));
        unique(references.map((item) => manjuReferenceCodeKey(item.code)))
            .filter((code) => /^(?:CHR|SCN|PRP)-/.test(code))
            .forEach((code) => {
                const reference = referenceByCode.get(code);
                candidates.set(`asset:${code}`, {
                    code,
                    title: reference?.title || code,
                    kind: code.startsWith("CHR-") ? "character" : code.startsWith("SCN-") ? "scene" : "prop",
                    taskId: reference?.taskId || `asset:${code}`,
                    prompt: reference?.prompt || "",
                    reference,
                });
            });
    }
    if (!candidates.size) return null;

    const backingNodes: CanvasNodeData[] = [];
    const rowByCode = new Map<string, string>();
    const rows: CanvasBatchRow[] = [];
    [...candidates.values()].forEach((candidate) => {
        const rowId = `manju-asset-row-${nanoid()}`;
        if (!rowByCode.has(candidate.code)) rowByCode.set(candidate.code, rowId);
        if (candidate.taskCode && !rowByCode.has(candidate.taskCode)) rowByCode.set(candidate.taskCode, rowId);
        let backing: CanvasNodeData | undefined;
        if (candidate.reference && (candidate.reference.content || candidate.reference.storageKey)) {
            backing = createReferenceNode(candidate.reference, { x: table.position.x, y: table.position.y }, pkg);
            backing.id = `manju-asset-backing-${nanoid()}`;
            backing.position = { x: table.position.x, y: table.position.y };
            backing.metadata = {
                ...backing.metadata,
                manjuAssetTableId: table.id,
                manjuAssetRowId: rowId,
                manjuAssetCode: candidate.code,
            };
            backingNodes.push(backing);
        }
        rows.push({
            id: rowId,
            enabled: true,
            inputNodeIds: backing ? [backing.id] : [],
            outputNodeId: backing?.id,
            prompt: candidate.prompt,
            manjuAsset: {
                taskId: candidate.taskId,
                code: candidate.code,
                taskCode: candidate.taskCode,
                kind: candidate.kind,
                name: candidate.title,
            },
        });
    });
    table.metadata = {
        ...table.metadata,
        batchTable: {
            ...table.metadata?.batchTable,
            operation: "creative",
            concurrency: 5,
            rows,
            referenceColumns: [{ id: "manju-asset-input", label: "引入图" }],
            manjuAssetTable: {
                version: 1,
                projectId: pkg.projectId,
                assetRunId: pkg.stage === "p2_assets" ? pkg.assetRunId : undefined,
                sourceStage: "p2_assets",
                promptRepairVersion: MANJU_ASSET_PROMPT_REPAIR_VERSION,
            },
            manjuGenerationDefaults: {
                size: pkg.style?.aspectRatio,
            },
        },
        manjuGlobalStylePrompt: pkg.style?.globalStylePrompt || pkg.style?.imageStylePrompt,
    };
    return { node: table, backingNodes, rowByCode };
}

function createManjuStoryboardTable(pkg: ManjuCanvasPackage, references: ManjuCanvasReference[], taskReferenceCodes: string[], center: Position) {
    const tasks = pkg.stage === "p3_images" ? pkg.tasks.filter((task) => task.group === "storyboard" || task.group === "plot-grid") : [];
    const codes = pkg.stage === "p3_images" ? unique(tasks.map((task) => task.code)) : unique(taskReferenceCodes.filter((code) => !/^(CHR|SCN|PRP)-/.test(code)));
    if (!codes.length) return null;

    const table = createCanvasNode(
        CanvasNodeType.BatchTable,
        { x: center.x - 560, y: center.y + 520 },
        {
            batchTable: {
                operation: "creative",
                concurrency: 5,
                rows: [],
                referenceColumns: [{ id: "manju-storyboard-reference", label: "资产参考" }],
                manjuStoryboardTable: {
                    version: 1,
                    projectId: pkg.projectId,
                    productionRunId: pkg.stage === "p3_images" ? pkg.productionRunId : undefined,
                    sourceStage: "p3_images",
                },
                manjuGenerationDefaults: {
                    size: "16:9",
                },
            },
            manjuGlobalStylePrompt: pkg.style?.globalStylePrompt || pkg.style?.imageStylePrompt,
        },
    );
    table.id = `manju-storyboard-table-${nanoid()}`;
    table.title = "漫剧故事板表";
    table.width = 1280;
    table.height = 720;

    const backingNodes: CanvasNodeData[] = [];
    const rowByCode = new Map<string, string>();
    const rows: CanvasBatchRow[] = codes.map((code) => {
        const task = tasks.find((item) => item.code === code);
        const reference = references.find((item) => item.code === code);
        const rowId = `manju-storyboard-row-${nanoid()}`;
        rowByCode.set(code, rowId);
        let backing: CanvasNodeData | undefined;
        if (reference && (reference.content || reference.storageKey)) {
            backing = createReferenceNode(reference, { x: table.position.x, y: table.position.y }, pkg);
            backing.id = `manju-storyboard-backing-${nanoid()}`;
            backing.position = { ...table.position };
            backing.metadata = {
                ...backing.metadata,
                manjuAssetTableId: table.id,
                manjuAssetRowId: rowId,
                manjuTaskCode: code,
            };
            backingNodes.push(backing);
        }
        return {
            id: rowId,
            enabled: true,
            inputNodeIds: backing ? [backing.id] : [],
            outputNodeId: backing?.id,
            prompt: stripEmbeddedManjuGlobalStyle(task?.prompt || "", pkg.style?.globalStylePrompt || pkg.style?.imageStylePrompt),
            manjuStoryboard: {
                taskId: task?.id || reference?.taskId || `storyboard:${code}`,
                code,
                title: task ? manjuTaskCanvasTitle(task) : reference?.title || code,
                group: task?.group === "plot-grid" ? "plot-grid" : "storyboard",
                referenceCodes: normalizeManjuVisualReferenceCodes(task?.referenceCodes || []),
            },
        };
    });
    table.metadata = {
        ...table.metadata,
        batchTable: {
            ...table.metadata?.batchTable,
            operation: "creative",
            concurrency: 5,
            rows,
            referenceColumns: [{ id: "manju-storyboard-reference", label: "资产参考" }],
            manjuStoryboardTable: {
                version: 1,
                projectId: pkg.projectId,
                productionRunId: pkg.stage === "p3_images" ? pkg.productionRunId : undefined,
                sourceStage: "p3_images",
            },
            manjuGenerationDefaults: {
                size: "16:9",
            },
        },
        manjuGlobalStylePrompt: pkg.style?.globalStylePrompt || pkg.style?.imageStylePrompt,
    };
    return { node: table, backingNodes, rowByCode };
}

function createReferenceNode(reference: ManjuCanvasReference, position: Position, pkg: ManjuCanvasPackage) {
    const ready = Boolean(reference.content || reference.storageKey);
    const node = createCanvasNode(CanvasNodeType.Image, position, {
        content: reference.content || "",
        storageKey: reference.storageKey,
        assetId: reference.assetId,
        status: ready ? "success" : "idle",
        nodeRole: "result",
        resultOrigin: "imported",
        workflowKind: "final",
        workflowTitle: reference.title,
        workflowDescription: ready ? "来自 P2 画布的真实资产引用" : "缺少 P2 资源；可在画布上传补齐后作为参考",
        assetCategory: referenceCategory(reference),
        assetTags: unique(["漫剧P2参考", reference.assetCode || reference.code]),
        manjuProjectId: pkg.projectId,
        manjuStage: "p2_assets",
        manjuTaskId: reference.taskId,
        manjuAssetCode: reference.assetCode || reference.code,
        manjuTaskCode: reference.code,
        importSource: {
            provider: "manju",
            projectId: pkg.projectId,
            stage: "p2_assets",
            taskId: reference.taskId,
            taskCode: reference.code,
            sourceArtifact: "ASSET_DELIVERY",
        },
    });
    node.title = reference.title;
    return node;
}

function createTaskNode(task: ManjuCanvasTask, pkg: ManjuCanvasPackage, index: number, center: Position) {
    const globalStylePrompt = task.globalStylePrompt || (task.kind === "image" ? pkg.style?.imageStylePrompt : undefined);
    const taskPrompt = stripEmbeddedManjuGlobalStyle(task.prompt, globalStylePrompt || pkg.style?.globalStylePrompt);
    const position = {
        x: center.x,
        y: center.y + index * MANJU_LAYOUT_GAP,
    };
    const metadata: CanvasNodeMetadata = {
        content: "",
        status: "idle",
        prompt: taskPrompt,
        composerContent: taskPrompt,
        generationMode: task.kind,
        workflowKind: workflowKindForTask(task),
        workflowTitle: task.title,
        workflowDescription: taskPrompt,
        assetCategory: task.category,
        assetTags: unique(["漫剧", pkg.stage, task.group || "", ...(task.tags || [])]),
        manjuProjectId: pkg.projectId,
        manjuStage: pkg.stage,
        manjuRunId: pkg.assetRunId || pkg.productionRunId,
        manjuPackageHash: pkg.hash,
        manjuTaskId: task.id,
        manjuTaskCode: task.code,
        manjuAssetCode: task.assetCode,
        manjuSourceArtifact: task.sourceKind,
        manjuReferenceCodes: manjuTaskReferenceCodes(task),
        manjuGlobalStylePrompt: globalStylePrompt,
        importSource: {
            provider: "manju",
            projectId: pkg.projectId,
            stage: pkg.stage,
            taskId: task.id,
            taskCode: task.code,
            sourceArtifact: task.sourceKind,
            packageHash: pkg.hash,
        },
    };
    if (task.kind === "video") {
        metadata.videoEditOperation = task.referenceCodes?.length ? "reference_to_video" : "text_to_video";
        metadata.seconds = String(task.seconds || 5);
        metadata.size = task.aspectRatio;
    } else {
        metadata.generationType = "generation";
        metadata.size = task.aspectRatio;
    }
    const node = createCanvasNode(task.kind === "video" ? CanvasNodeType.Video : CanvasNodeType.Image, position, metadata);
    node.title = manjuTaskCanvasTitle(task);
    return node;
}

function manjuTaskCanvasTitle(task: ManjuCanvasTask) {
    const code = task.code.trim();
    const assetCode = task.assetCode?.trim() || "";
    const descriptor = task.title
        .split("·")
        .map((part) => part.trim())
        .filter((part) => part && part !== code && part !== assetCode)
        .join(" · ");
    return unique([code, assetCode, descriptor]).filter(Boolean).join(" · ");
}

function stripEmbeddedManjuGlobalStyle(prompt: string, globalStylePrompt?: string) {
    let taskPrompt = prompt.trim();
    const globalStyle = (globalStylePrompt || "").trim();
    if (globalStyle && taskPrompt.includes(globalStyle)) {
        taskPrompt = taskPrompt.replace(globalStyle, "").trim();
    }
    if (taskPrompt.includes(MANJU_STYLE_EXECUTION_SEPARATOR)) {
        taskPrompt = taskPrompt.split(MANJU_STYLE_EXECUTION_SEPARATOR).at(-1)?.trim() || "";
    }
    return taskPrompt;
}

function styleContext(pkg: ManjuCanvasPackage) {
    const style = pkg.style;
    const styleBase = style?.styleBase || "未提供";
    const aspectRatio = style?.aspectRatio || "未提供";
    const parameters = style?.promptParameters || "未提供";
    const globalStyle = style?.globalStylePrompt?.trim();
    const imageStyle = style?.imageStylePrompt?.trim();
    const referenceAnalysis = style?.referenceAnalysis?.trim();
    return [
        imageStyle || globalStyle || ["【STYLE｜视觉风格 · 全片锁死】", `画风卡：${style?.cardId || "未提供"}`, `影像基底：${styleBase}`, `成片画幅：${aspectRatio}`, `视觉参数：${parameters}`].join("\n"),
        referenceAnalysis && !imageStyle?.includes(referenceAnalysis) ? `\n【风格参考图分析】\n${referenceAnalysis}\n只迁移光线、色彩、材质、镜头和后期质感，禁止复刻参考图构图、人物身份、服装和场景。` : "",
        globalStyle && globalStyle !== imageStyle ? `\n${globalStyle}` : "",
    ].join("\n");
}

function createStyleReferenceNode(pkg: ManjuCanvasPackage, position: Position) {
    const storageKey = pkg.style?.referenceStorageKey?.trim() || "";
    const storageResourceId = storageKey.startsWith("resource:") ? storageKey.slice("resource:".length) : "";
    const resourceId = pkg.style?.referenceResourceId?.trim() || storageResourceId;
    const content = resourceId ? `/api/resources/${encodeURIComponent(resourceId)}/file` : "";
    const node = createCanvasNode(CanvasNodeType.Image, position, {
        content,
        previewContent: content,
        storageKey: storageKey || (resourceId ? `resource:${resourceId}` : undefined),
        assetId: pkg.style?.referenceAssetId?.trim() || undefined,
        status: "success",
        nodeRole: "result",
        resultOrigin: "imported",
        workflowKind: "styleboard",
        workflowTitle: "全局风格参考图",
        workflowDescription: "只迁移光线、色彩、材质、镜头和后期质感，不复刻构图与人物",
        prompt: "作为全片强风格参考：只迁移光线、主辅色、材质、镜头、颗粒与后期质感；不复刻构图、人物身份、服装、文字和具体场景。",
        assetCategory: "material",
        assetTags: unique(["漫剧全局风格参考", pkg.style?.cardId || ""]),
        manjuProjectId: pkg.projectId,
        manjuStage: "p2_assets",
        manjuSharedStyleReference: true,
        manjuPackageHash: pkg.hash,
        manjuStyleReferenceAnalysis: pkg.style?.referenceAnalysis,
    });
    node.title = "VisiDeep AI · 全局风格参考图";
    return node;
}

function workflowKindForTask(task: ManjuCanvasTask): CanvasNodeMetadata["workflowKind"] {
    if (task.kind === "video") return "shot";
    if (task.group === "storyboard" || task.group === "plot-grid") return "storyboard";
    if (task.category === "character") return "character";
    if (task.category === "environment") return "scene";
    return "final";
}

function referenceCategory(reference: ManjuCanvasReference): NonNullable<CanvasNodeMetadata["assetCategory"]> {
    if (reference.category === "character" || reference.category === "environment" || reference.category === "prop" || reference.category === "material" || reference.category === "other") {
        return reference.category;
    }
    if ((reference.assetCode || reference.code).startsWith("CHR-")) return "character";
    if ((reference.assetCode || reference.code).startsWith("SCN-")) return "environment";
    if ((reference.assetCode || reference.code).startsWith("PRP-")) return "prop";
    return "material";
}

function taskOrder(task: ManjuCanvasTask) {
    if (task.group === "storyboard") return 0;
    if (task.group === "plot-grid") return 1;
    if (task.group === "segment") return 2;
    return 3;
}

function unique(values: string[]) {
    return Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)));
}

function connectionKey(connection: CanvasConnection) {
    return [connection.fromNodeId, connection.toNodeId, connection.fromHandleId || "", connection.toHandleId || "", connection.relation || ""].join("\u0000");
}
