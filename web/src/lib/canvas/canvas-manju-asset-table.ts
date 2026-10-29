import { nanoid } from "nanoid";

import type { CanvasBatchRow, CanvasBatchTableData, CanvasConnection, CanvasManjuAssetKind, CanvasNodeData } from "@/types/canvas";

export const MANJU_ASSET_INPUT_HANDLE_PREFIX = "manju-asset-in:";
export const MANJU_ASSET_OUTPUT_HANDLE_PREFIX = "manju-asset-out:";
export const MANJU_ASSET_TABLE_HEADER_HEIGHT = 88;
export const MANJU_ASSET_TABLE_ROW_HEIGHT = 132;
export const MANJU_ASSET_KIND_ORDER: CanvasManjuAssetKind[] = ["character", "scene", "prop"];

export function manjuAssetInputHandleId(rowId: string) {
    return `${MANJU_ASSET_INPUT_HANDLE_PREFIX}${rowId}`;
}

export function manjuAssetOutputHandleId(rowId: string) {
    return `${MANJU_ASSET_OUTPUT_HANDLE_PREFIX}${rowId}`;
}

export function manjuAssetRowIdFromHandle(handleId?: string, direction?: "input" | "output") {
    const input = handleId?.startsWith(MANJU_ASSET_INPUT_HANDLE_PREFIX);
    const output = handleId?.startsWith(MANJU_ASSET_OUTPUT_HANDLE_PREFIX);
    if (direction === "input" && !input) return undefined;
    if (direction === "output" && !output) return undefined;
    if (!input && !output) return undefined;
    return handleId!.slice((input ? MANJU_ASSET_INPUT_HANDLE_PREFIX : MANJU_ASSET_OUTPUT_HANDLE_PREFIX).length);
}

export function isManjuAssetTable(node?: CanvasNodeData) {
    return Boolean(node?.type === "batch-table" && node.metadata?.batchTable?.manjuAssetTable);
}

export function isManjuStoryboardTable(node?: CanvasNodeData) {
    return Boolean(node?.type === "batch-table" && node.metadata?.batchTable?.manjuStoryboardTable);
}

export function isManjuManagedTable(node?: CanvasNodeData) {
    return isManjuAssetTable(node) || isManjuStoryboardTable(node);
}

export function appendManjuAssetRow(table: CanvasBatchTableData, kind: CanvasManjuAssetKind): CanvasBatchTableData {
    const prefix = kind === "character" ? "CHR" : kind === "scene" ? "SCN" : "PRP";
    const usedCodes = new Set(table.rows.map((row) => row.manjuAsset?.code?.trim().toUpperCase()).filter(Boolean));
    let number = 1;
    while (usedCodes.has(`${prefix}-${String(number).padStart(2, "0")}`)) number += 1;
    const code = `${prefix}-${String(number).padStart(2, "0")}`;
    const rowId = `manju-asset-row-${nanoid()}`;
    const row: CanvasBatchRow = {
        id: rowId,
        enabled: true,
        inputNodeIds: [],
        prompt: "",
        manjuAsset: {
            taskId: `manual:asset:${rowId}`,
            code,
            kind,
            name: `自定义${manjuAssetKindLabel(kind)}${number}`,
        },
    };
    return { ...table, rows: [...table.rows, row] };
}

export function appendManjuStoryboardRow(table: CanvasBatchTableData): CanvasBatchTableData {
    const highestSegment = table.rows.reduce((highest, row) => {
        const match = row.manjuStoryboard?.code?.trim().match(/^段\s*(\d+)$/);
        return match ? Math.max(highest, Number(match[1]) || 0) : highest;
    }, 0);
    const number = highestSegment + 1;
    const code = `段${number}`;
    const rowId = `manju-storyboard-row-${nanoid()}`;
    const row: CanvasBatchRow = {
        id: rowId,
        enabled: true,
        inputNodeIds: [],
        prompt: "",
        manjuStoryboard: {
            taskId: `manual:storyboard:${rowId}`,
            code,
            title: `自定义故事板${number}`,
            group: "storyboard",
            referenceCodes: [],
        },
    };
    return { ...table, rows: [...table.rows, row] };
}

export function manjuCanvasAssetRevision(nodes: CanvasNodeData[]) {
    return nodes
        .flatMap((node) => {
            const metadata = node.metadata;
            if (!metadata?.manjuAssetTableId && !metadata?.manjuAssetCode && metadata?.manjuStage !== "p2_assets") return [];
            const content = metadata.content || "";
            const preview = metadata.previewContent || "";
            return [`${node.id}:${metadata.status || ""}:${metadata.manjuAssetRowId || ""}:${metadata.storageKey || ""}:${metadata.assetId || ""}:${content.length}:${content.slice(-48)}:${preview.length}`];
        })
        .sort()
        .join("\n");
}

export function manjuAssetRows(table: CanvasBatchTableData | undefined, kind?: CanvasManjuAssetKind) {
    const rows = table?.rows || [];
    return kind ? rows.filter((row) => row.manjuAsset?.kind === kind) : rows;
}

export function preferredManjuAssetKind(table: CanvasBatchTableData | undefined, preferred: CanvasManjuAssetKind = "character"): CanvasManjuAssetKind {
    if (manjuAssetRows(table, preferred).length) return preferred;
    return MANJU_ASSET_KIND_ORDER.find((kind) => manjuAssetRows(table, kind).length) || preferred;
}

export function manjuAssetRowByHandle(node: CanvasNodeData | undefined, handleId: string | undefined, direction?: "input" | "output"): CanvasBatchRow | undefined {
    if (!isManjuManagedTable(node)) return undefined;
    const rowId = manjuAssetRowIdFromHandle(handleId, direction);
    return rowId ? node?.metadata?.batchTable?.rows.find((row) => row.id === rowId) : undefined;
}

export function manjuAssetRowOutputNode(node: CanvasNodeData, row: CanvasBatchRow, nodes: CanvasNodeData[]) {
    const currentId = row.outputNodeId || row.inputNodeIds.find(Boolean);
    return currentId ? nodes.find((item) => item.id === currentId) : undefined;
}

export function manjuAssetRowMediaNode(row: CanvasBatchRow, nodes: CanvasNodeData[]) {
    return buildManjuAssetRowMediaMap([row], nodes).get(row.id);
}

export function buildManjuAssetRowMediaMap(rows: CanvasBatchRow[], nodes: CanvasNodeData[]) {
    return buildManjuAssetRowMediaMapWithOptions(rows, nodes, true);
}

export function buildManjuAssetRowOutputMediaMap(rows: CanvasBatchRow[], nodes: CanvasNodeData[]) {
    return buildManjuAssetRowMediaMapWithOptions(rows, nodes, false);
}

function buildManjuAssetRowMediaMapWithOptions(rows: CanvasBatchRow[], nodes: CanvasNodeData[], includeInputs: boolean) {
    const nodeById = new Map(nodes.map((node) => [node.id, node]));
    const mediaIndex = buildManjuAssetMediaIndex(nodes, nodeById);
    return new Map(rows.map((row) => [row.id, manjuAssetRowMediaNodeFromMap(row, mediaIndex, includeInputs)]));
}

export function resolveManjuAssetConnectionSource(connection: Pick<CanvasConnection, "fromNodeId" | "fromHandleId">, nodes: CanvasNodeData[]) {
    const source = nodes.find((node) => node.id === connection.fromNodeId);
    const row = manjuAssetRowByHandle(source, connection.fromHandleId, "output");
    return source && row ? manjuAssetRowMediaNode(row, nodes) : undefined;
}

export function manjuAssetHandleY(node: CanvasNodeData, handleId?: string, scrollTop = 0) {
    if (!isManjuManagedTable(node)) return undefined;
    const rowId = manjuAssetRowIdFromHandle(handleId);
    if (!rowId) return undefined;
    const rows = node.metadata?.batchTable?.rows || [];
    if (node.metadata?.batchTable?.manjuStoryboardTable) {
        const index = rows.findIndex((row) => row.id === rowId);
        if (index < 0) return undefined;
        const localY = index * MANJU_ASSET_TABLE_ROW_HEIGHT + MANJU_ASSET_TABLE_ROW_HEIGHT / 2 - scrollTop;
        return node.position.y + MANJU_ASSET_TABLE_HEADER_HEIGHT + Math.max(8, localY);
    }
    const activeKind = node.metadata?.manjuAssetActiveKind || "character";
    const visibleRows = rows.filter((row) => row.manjuAsset?.kind === activeKind);
    const index = visibleRows.findIndex((row) => row.id === rowId);
    if (index < 0) return undefined;
    const localY = index * MANJU_ASSET_TABLE_ROW_HEIGHT + MANJU_ASSET_TABLE_ROW_HEIGHT / 2 - scrollTop;
    return node.position.y + MANJU_ASSET_TABLE_HEADER_HEIGHT + Math.max(8, localY);
}

export function manjuAssetHandleAtY(node: CanvasNodeData, worldY: number, scrollTop = 0, hitRadius = 24) {
    if (!isManjuManagedTable(node)) return undefined;
    const rows = node.metadata?.batchTable?.rows || [];
    const nearest: { value: { rowId: string; distance: number } | null } = { value: null };
    rows.forEach((row) => {
        const y = manjuAssetHandleY(node, manjuAssetInputHandleId(row.id), scrollTop);
        if (y === undefined) return;
        const distance = Math.abs(worldY - y);
        if (!nearest.value || distance < nearest.value.distance) nearest.value = { rowId: row.id, distance };
    });
    return nearest.value && nearest.value.distance <= hitRadius ? manjuAssetInputHandleId(nearest.value.rowId) : undefined;
}

export function manjuAssetKindForCategory(category?: string): CanvasManjuAssetKind {
    if (category === "character") return "character";
    if (category === "environment" || category === "scene") return "scene";
    return "prop";
}

export function manjuAssetKindLabel(kind: CanvasManjuAssetKind) {
    if (kind === "character") return "人物";
    if (kind === "scene") return "场景";
    return "道具";
}

export type ManjuStoryboardAssetReference = {
    code: string;
    name: string;
    kind: CanvasManjuAssetKind;
    rowId?: string;
    node?: CanvasNodeData;
    pending: boolean;
};

type ManjuAssetRowCandidate = {
    table: CanvasNodeData;
    row: CanvasBatchRow;
    index: number;
};

type ManjuAssetMediaIndex = {
    byId: ReadonlyMap<string, CanvasNodeData>;
    byRowId: ReadonlyMap<string, CanvasNodeData[]>;
    byCode: ReadonlyMap<string, CanvasNodeData[]>;
};

export function buildManjuStoryboardAssetReferenceMap(
    storyboardTable: CanvasNodeData | undefined,
    storyboardRows: CanvasBatchRow[],
    nodes: CanvasNodeData[],
    nodeById = new Map(nodes.map((node) => [node.id, node])),
) {
    const projectId = storyboardTable?.metadata?.batchTable?.manjuStoryboardTable?.projectId;
    const mediaIndex = buildManjuAssetMediaIndex(nodes, nodeById);
    const assetRowByCode = new Map<string, ManjuAssetRowCandidate[]>();
    let rowIndex = 0;
    manjuAssetTablesForResolution(nodes, projectId).forEach((table) => {
        (table.metadata?.batchTable?.rows || []).forEach((row) => {
            const code = row.manjuAsset?.code;
            if (!code) return;
            const candidate = { table, row, index: rowIndex++ };
            if (!assetRowByCode.has(code)) assetRowByCode.set(code, []);
            assetRowByCode.get(code)!.push(candidate);
            const canonical = manjuReferenceCodeKey(code);
            if (!assetRowByCode.has(canonical)) assetRowByCode.set(canonical, []);
            if (canonical !== code) assetRowByCode.get(canonical)!.push(candidate);
        });
    });
    const resolveCandidate = (code: string) => {
        const candidates = uniqueAssetRowCandidates([
            ...(assetRowByCode.get(code) || []),
            ...(assetRowByCode.get(manjuReferenceCodeKey(code)) || []),
        ]);
        return selectManjuAssetRowCandidate(candidates, mediaIndex);
    };
    return new Map(
        storyboardRows.map((storyboardRow) => [
            storyboardRow.id,
            (storyboardRow.manjuStoryboard?.referenceCodes || []).map((code): ManjuStoryboardAssetReference => {
                const selected = resolveCandidate(code);
                const row = selected?.row;
                const node = selected?.node;
                return {
                    code,
                    name: row?.manjuAsset?.name || code,
                    kind: row?.manjuAsset?.kind || manjuKindForCode(code),
                    rowId: row?.id,
                    node,
                    pending: !hasManjuPreview(node),
                };
            }),
        ]),
    );
}

export function resolveManjuStoryboardAssetReferences(storyboardTable: CanvasNodeData | undefined, storyboardRow: CanvasBatchRow, nodes: CanvasNodeData[]): ManjuStoryboardAssetReference[] {
    return buildManjuStoryboardAssetReferenceMap(storyboardTable, [storyboardRow], nodes).get(storyboardRow.id) || [];
}

export function resolveManjuInternalReferenceNodes(target: CanvasNodeData, nodes: CanvasNodeData[]): CanvasNodeData[] {
    const referenceCodes = manjuInternalReferenceCodes(target, nodes);
    return uniqueNodes(
        referenceCodes
            .map((code) => resolveManjuReferenceCode(code, nodes))
            .filter((node): node is CanvasNodeData => Boolean(node && node.id !== target.id)),
    );
}

export function cleanupManjuInternalConnections(nodes: CanvasNodeData[], connections: CanvasConnection[]) {
    const nodeById = new Map(nodes.map((node) => [node.id, node]));
    const retained = connections.filter((connection) => {
        if (connection.id.startsWith("manju-auto:")) return false;
        const source = nodeById.get(connection.fromNodeId);
        const target = nodeById.get(connection.toNodeId);
        if (!source || !target) return false;
        return !isManjuInternalCanvasNode(source) && !isManjuInternalCanvasNode(target);
    });
    return retained.length === connections.length ? connections : retained;
}

function isManjuInternalCanvasNode(node: CanvasNodeData) {
    return Boolean(
        isManjuManagedTable(node)
        || node.metadata?.manjuStage
        || node.metadata?.manjuAssetTableId
        || node.metadata?.manjuAssetCode
        || node.metadata?.manjuSharedStyleNode
        || node.metadata?.manjuSharedStyleReference,
    );
}

function manjuInternalReferenceCodes(target: CanvasNodeData, nodes: CanvasNodeData[]) {
    if (target.metadata?.manjuStage === "p3_images" && target.metadata.manjuAssetTableId && target.metadata.manjuAssetRowId) {
        const table = nodes.find((node) => node.id === target.metadata?.manjuAssetTableId);
        if (isManjuStoryboardTable(table)) {
            const row = table?.metadata?.batchTable?.rows.find((item) => item.id === target.metadata?.manjuAssetRowId);
            return row?.manjuStoryboard?.referenceCodes || [];
        }
    }
    return target.metadata?.manjuReferenceCodes || [];
}

function resolveManjuReferenceCode(code: string, nodes: CanvasNodeData[]) {
    const storyboardTable = nodes.find(isManjuStoryboardTable);
    const projectId = storyboardTable?.metadata?.batchTable?.manjuStoryboardTable?.projectId;
    const canonical = manjuReferenceCodeKey(code);
    const nodeById = new Map(nodes.map((node) => [node.id, node]));
    const mediaIndex = buildManjuAssetMediaIndex(nodes, nodeById);
    const assetCandidates = manjuAssetTablesForResolution(nodes, projectId)
        .flatMap((table) => (table.metadata?.batchTable?.rows || []).map((row, index) => ({ table, row, index })))
        .filter((candidate) => candidate.row.manjuAsset?.code && manjuReferenceCodeKey(candidate.row.manjuAsset.code) === canonical);
    const selected = selectManjuAssetRowCandidate(assetCandidates, mediaIndex);
    if (selected?.node) return selected.node;
    if (selected?.row) return manjuAssetRowMediaNode(selected.row, nodes);
    const storyboardRow = storyboardTable?.metadata?.batchTable?.rows.find((row) => row.manjuStoryboard?.code && manjuReferenceCodeKey(row.manjuStoryboard.code) === canonical);
    return storyboardRow ? manjuAssetRowMediaNode(storyboardRow, nodes) : undefined;
}

function manjuAssetTablesForResolution(nodes: CanvasNodeData[], projectId?: string) {
    const tables = nodes.filter(isManjuAssetTable);
    if (!projectId) return tables;
    const projectTables = tables.filter((table) => table.metadata?.batchTable?.manjuAssetTable?.projectId === projectId);
    return projectTables.length ? projectTables : tables;
}

function uniqueAssetRowCandidates(candidates: ManjuAssetRowCandidate[]) {
    const seen = new Set<string>();
    return candidates.filter((candidate) => {
        const key = `${candidate.table.id}:${candidate.row.id}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    });
}

function selectManjuAssetRowCandidate(candidates: ManjuAssetRowCandidate[], mediaIndex: ManjuAssetMediaIndex) {
    let selected: { row: CanvasBatchRow; node?: CanvasNodeData; score: number; index: number } | undefined;
    candidates.forEach((candidate) => {
        const node = manjuAssetRowMediaNodeFromMap(candidate.row, mediaIndex, true);
        const score = (hasManjuMedia(node) ? 4 : node ? 2 : 0)
            + (candidate.row.outputNodeId ? 1 : 0)
            + (candidate.row.inputNodeIds.length ? 1 : 0);
        if (!selected || score > selected.score || (score === selected.score && candidate.index < selected.index)) {
            selected = { row: candidate.row, node, score, index: candidate.index };
        }
    });
    return selected;
}

export function manjuReferenceCodeKey(code: string) {
    return code.trim().replace(/^CNH-/i, "CHR-");
}

function manjuKindForCode(code: string): CanvasManjuAssetKind {
    if (code.startsWith("SCN-")) return "scene";
    if (code.startsWith("PRP-")) return "prop";
    return "character";
}

function hasManjuMedia(node: CanvasNodeData | undefined) {
    return Boolean(node?.metadata?.content || node?.metadata?.previewContent || node?.metadata?.storageKey || node?.metadata?.assetId);
}

function hasManjuPreview(node: CanvasNodeData | undefined) {
    return hasManjuMedia(node);
}

function buildManjuAssetMediaIndex(nodes: CanvasNodeData[], nodeById: ReadonlyMap<string, CanvasNodeData>): ManjuAssetMediaIndex {
    const byRowId = new Map<string, CanvasNodeData[]>();
    const byCode = new Map<string, CanvasNodeData[]>();
    nodes.forEach((node) => {
        const rowId = node.metadata?.manjuAssetRowId;
        if (rowId) {
            const matches = byRowId.get(rowId) || [];
            matches.push(node);
            byRowId.set(rowId, matches);
        }
        const nodeCode = node.metadata?.manjuAssetCode;
        if (nodeCode) {
            const key = manjuReferenceCodeKey(nodeCode);
            const matches = byCode.get(key) || [];
            matches.push(node);
            byCode.set(key, matches);
        }
    });
    return { byId: nodeById, byRowId, byCode };
}

function manjuAssetRowMediaNodeFromMap(row: CanvasBatchRow, mediaIndex: ManjuAssetMediaIndex, includeInputs: boolean) {
    const direct = [row.outputNodeId, ...(includeInputs ? row.inputNodeIds : [])]
        .filter((id): id is string => Boolean(id))
        .map((id) => mediaIndex.byId.get(id))
        .filter((node): node is CanvasNodeData => Boolean(node));
    const code = row.manjuAsset?.code;
    const codeKey = code ? manjuReferenceCodeKey(code) : "";
    const indexed = [
        ...(mediaIndex.byRowId.get(row.id) || []),
        ...(codeKey ? mediaIndex.byCode.get(codeKey) || [] : []),
    ];
    const candidates = uniqueNodes([...direct, ...indexed]);
    return candidates.find(hasManjuMedia) || candidates.find(hasManjuPreview) || candidates[0];
}

function uniqueNodes(nodes: CanvasNodeData[]) {
    const seen = new Set<string>();
    return nodes.filter((node) => {
        if (seen.has(node.id)) return false;
        seen.add(node.id);
        return true;
    });
}
