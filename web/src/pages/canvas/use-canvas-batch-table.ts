import { useCallback, type Dispatch, type SetStateAction } from "react";
import { App } from "antd";
import { nanoid } from "nanoid";

import { NODE_DEFAULT_SIZE } from "@/constant/canvas";
import {
    batchInputColumns,
    batchPromptForRow,
    batchReferenceColumns,
    batchReferenceHandleId,
    createBatchRowsFromColumns,
    createInheritedBatchRow,
    moveBatchReferenceCell,
    removeLastBatchReferenceColumn,
    reorderBatchReferenceColumns,
} from "@/lib/canvas/canvas-batch-table";
import { createCanvasNode } from "@/lib/canvas/canvas-project-domain";
import { buildGenerationConfig, resetGenerationTaskMetadata } from "@/lib/canvas/canvas-project-generation";
import { appendManjuAssetRow, appendManjuStoryboardRow, manjuAssetInputHandleId, manjuAssetRowIdFromHandle, resolveManjuAssetConnectionSource } from "@/lib/canvas/canvas-manju-asset-table";
import { resolveBatchGenerationModels, resolveCanvasGenerationSettings } from "@/lib/canvas/canvas-manju-generation-settings";
import { navigateToSettings } from "@/lib/settings-navigation";
import { modelDisplayName, useConfigStore, useEffectiveConfig } from "@/stores/use-config-store";
import { CanvasNodeType, type CanvasBatchRow, type CanvasBatchTableData, type CanvasConnection, type CanvasGenerationBatchMode, type CanvasNodeData } from "@/types/canvas";

type Options = {
    nodesRef: { current: CanvasNodeData[] };
    connectionsRef: { current: CanvasConnection[] };
    setNodes: Dispatch<SetStateAction<CanvasNodeData[]>>;
    setConnections: Dispatch<SetStateAction<CanvasConnection[]>>;
    setSelectedNodeIds: Dispatch<SetStateAction<Set<string>>>;
    enqueueGenerationBatch: (sourceNodeId: string, mode: CanvasGenerationBatchMode, targets: Array<{ rowId: string; nodeId: string }>, options?: { concurrency?: number }) => string | undefined;
};

export function useCanvasBatchTable({ nodesRef, connectionsRef, setNodes, setConnections, setSelectedNodeIds, enqueueGenerationBatch }: Options) {
    const { message, modal } = App.useApp();
    const effectiveConfig = useEffectiveConfig();
    const isAiConfigReady = useConfigStore((state) => state.isAiConfigReady);

    const patchTable = useCallback(
        (nodeId: string, patch: Partial<CanvasBatchTableData>) => {
            setNodes((current) => current.map((node) => (node.id !== nodeId ? node : { ...node, metadata: { ...node.metadata, batchTable: { operation: "try_on", concurrency: 10, rows: [], ...node.metadata?.batchTable, ...patch } } })));
        },
        [setNodes],
    );

    const updateRow = useCallback(
        (nodeId: string, rowId: string, patch: Partial<CanvasBatchRow>) => {
            const node = nodesRef.current.find((item) => item.id === nodeId);
            if (!node?.metadata?.batchTable) return;
            patchTable(nodeId, { rows: node.metadata.batchTable.rows.map((row) => (row.id === rowId ? { ...row, ...patch } : row)) });
        },
        [nodesRef, patchTable],
    );

    const addRow = useCallback(
        (nodeId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            patchTable(nodeId, { rows: [...table.rows, createInheritedBatchRow(table.operation, table.rows)] });
        },
        [nodesRef, patchTable],
    );

    const addManjuRow = useCallback(
        (nodeId: string) => {
            const node = nodesRef.current.find((item) => item.id === nodeId);
            const table = node?.metadata?.batchTable;
            if (!table) return;
            const next = table.manjuStoryboardTable
                ? appendManjuStoryboardRow(table)
                : table.manjuAssetTable
                  ? appendManjuAssetRow(table, node.metadata?.manjuAssetActiveKind || "character")
                  : null;
            if (!next) return;
            patchTable(nodeId, { rows: next.rows });
        },
        [nodesRef, patchTable],
    );

    const removeRow = useCallback(
        (nodeId: string, rowId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            const row = table.rows.find((item) => item.id === rowId);
            const removedNodeIds = new Set(
                nodesRef.current
                    .filter((node) => node.id === row?.outputNodeId || (node.metadata?.manjuAssetTableId === nodeId && node.metadata?.manjuAssetRowId === rowId))
                    .map((node) => node.id),
            );
            patchTable(nodeId, { rows: table.rows.filter((row) => row.id !== rowId) });
            if (removedNodeIds.size) setNodes((current) => current.filter((node) => !removedNodeIds.has(node.id)));
            setConnections((current) =>
                current.filter(
                    (connection) =>
                        !removedNodeIds.has(connection.fromNodeId) &&
                        !removedNodeIds.has(connection.toNodeId) &&
                        !(connection.toNodeId === nodeId && connection.toHandleId === manjuAssetInputHandleId(rowId)),
                ),
            );
        },
        [nodesRef, patchTable, setConnections, setNodes],
    );

    const addReferenceColumn = useCallback(
        (nodeId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            const columns = batchReferenceColumns(table);
            if (columns.length >= 6) return message.info("最多支持 6 组参考图");
            const nextIndex = columns.length + 1;
            patchTable(nodeId, { referenceColumns: [...columns, { id: `reference-${nanoid()}`, label: `参考图 ${nextIndex}` }] });
        },
        [message, nodesRef, patchTable],
    );

    const removeReferenceColumn = useCallback(
        (nodeId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            const columns = batchReferenceColumns(table);
            const nextTable = removeLastBatchReferenceColumn(table);
            if (!nextTable) return message.info("至少保留 1 组参考图");
            const removed = columns.at(-1);
            patchTable(nodeId, nextTable);
            if (removed) {
                const handleId = batchReferenceHandleId(removed.id);
                setConnections((current) => current.filter((connection) => !(connection.toNodeId === nodeId && connection.toHandleId === handleId)));
            }
        },
        [message, nodesRef, patchTable, setConnections],
    );

    const syncRowsFromConnections = useCallback(
        (nodeId: string, silent = false) => {
            const node = nodesRef.current.find((item) => item.id === nodeId);
            const table = node?.metadata?.batchTable;
            if (!node || !table) return false;
            if (table.manjuAssetTable || table.manjuStoryboardTable) {
                const inputsByRowId = new Map<string, string[]>();
                connectionsRef.current
                    .filter((connection) => connection.toNodeId === nodeId)
                    .forEach((connection) => {
                        const rowId = manjuAssetRowIdFromHandle(connection.toHandleId, "input");
                        const resolved = resolveManjuAssetConnectionSource(connection, nodesRef.current) || nodesRef.current.find((node) => node.id === connection.fromNodeId);
                        if (!rowId || !resolved) return;
                        const inputs = inputsByRowId.get(rowId) || [];
                        if (!inputs.includes(resolved.id)) inputs.push(resolved.id);
                        inputsByRowId.set(rowId, inputs);
                    });
                if (!inputsByRowId.size) return true;
                let changed = false;
                const rows = table.rows.map((row) => {
                    const inputNodeIds = inputsByRowId.get(row.id);
                    if (!inputNodeIds?.length || JSON.stringify(row.inputNodeIds) === JSON.stringify(inputNodeIds)) return row;
                    changed = true;
                    return { ...row, inputNodeIds };
                });
                if (changed) patchTable(nodeId, { rows });
                return true;
            }
            const nodeById = new Map(nodesRef.current.map((item) => [item.id, item]));
            const columns = batchInputColumns(node, connectionsRef.current).map((column) =>
                column.filter((inputNodeId) => {
                    const input = nodeById.get(inputNodeId);
                    return input?.type === CanvasNodeType.Image && Boolean(input.metadata?.content || input.metadata?.storageKey);
                }),
            );
            if (!columns.some((column) => column.length)) {
                if (!silent) message.warning("请先把图片节点连接到批量创作表");
                return false;
            }
            const rows = createBatchRowsFromColumns(table.operation, columns, table.rows);
            if (!rows.length) {
                if (!silent) message.warning("批量换装至少需要一张人物图和一张服装图");
                return false;
            }
            if (JSON.stringify(rows) === JSON.stringify(table.rows)) return true;
            patchTable(nodeId, { rows });
            if (!silent) {
                const added = Math.max(0, rows.length - table.rows.length);
                message.success(added ? `已同步连线并新增 ${added} 行，原有任务均已保留` : "已同步最新连线，原有任务均已保留");
            }
            return true;
        },
        [connectionsRef, message, nodesRef, patchTable],
    );

    const fillRowsFromConnections = useCallback(
        (nodeId: string) => {
            syncRowsFromConnections(nodeId);
        },
        [syncRowsFromConnections],
    );

    const reorderReferenceColumns = useCallback(
        (nodeId: string, fromColumnId: string, toColumnId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            const nextTable = reorderBatchReferenceColumns(table, fromColumnId, toColumnId);
            if (nextTable !== table) patchTable(nodeId, nextTable);
        },
        [nodesRef, patchTable],
    );

    const moveReferenceCell = useCallback(
        (nodeId: string, sourceRowId: string, sourceColumnIndex: number, targetRowId: string, targetColumnIndex: number) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            if (!table) return;
            const nextTable = moveBatchReferenceCell(table, sourceRowId, sourceColumnIndex, targetRowId, targetColumnIndex);
            if (nextTable !== table) patchTable(nodeId, { rows: nextTable.rows });
        },
        [nodesRef, patchTable],
    );

    const generateRows = useCallback(
        async (nodeId: string, requestedRowIds?: string[]) => {
            const sourceNode = nodesRef.current.find((item) => item.id === nodeId);
            const table = sourceNode?.metadata?.batchTable;
            if (!sourceNode || !table) return;
            const activeNodeIds = new Set(
                (sourceNode.metadata?.generationBatches || [])
                    .filter((batch) => batch.mode === "batch_image")
                    .flatMap((batch) => batch.items.filter((item) => ["waiting", "submitting", "queued", "running"].includes(item.status)).map((item) => item.nodeId)),
            );
            const isManjuStoryboardTable = Boolean(table.manjuStoryboardTable);
            const isManjuAssetTable = Boolean(table.manjuAssetTable) || isManjuStoryboardTable;
            const requested = requestedRowIds?.length ? new Set(requestedRowIds) : null;
            const rows = table.rows.filter((row) => {
                const inputNodeIds = row.inputNodeIds.filter(Boolean);
                if (!row.enabled || (requested && !requested.has(row.id)) || !batchPromptForRow(table, row).trim()) return false;
                if (table.operation === "try_on" && inputNodeIds.length < 2) return false;
                if (!isManjuAssetTable && (!inputNodeIds.length || inputNodeIds.some((id) => !nodesRef.current.some((node) => node.id === id && node.type === CanvasNodeType.Image && Boolean(node.metadata?.content || node.metadata?.storageKey)))))
                    return false;
                if (isManjuAssetTable && inputNodeIds.some((id) => !nodesRef.current.some((node) => node.id === id && node.type === CanvasNodeType.Image && Boolean(node.metadata?.content || node.metadata?.storageKey)))) return false;
                const output = row.outputNodeId ? nodesRef.current.find((node) => node.id === row.outputNodeId) : undefined;
                if (output && activeNodeIds.has(output.id)) return false;
                // 操作列的单行生成允许对已完成结果重新生成；顶部批量按钮仍只提交未完成项。
                if (requested) return true;
                return !output?.metadata?.content;
            });
            if (!rows.length) return message.info("没有可提交的未完成任务，请检查参考图和提示词");
            const fallbackGenerationConfig = buildGenerationConfig(effectiveConfig, undefined, "image");
            const batchModels = isManjuAssetTable
                ? resolveBatchGenerationModels(table, rows, effectiveConfig)
                : [fallbackGenerationConfig.model];
            if (!batchModels.length || batchModels.some((model) => !isAiConfigReady(effectiveConfig, model))) {
                navigateToSettings({ continueCreation: true });
                return;
            }
            const confirmed = await new Promise<boolean>((resolve) =>
                modal.confirm({
                    title: `确认提交 ${rows.length} 个批量图片任务`,
                    content: `模型：${batchModels.map((model) => modelDisplayName(effectiveConfig, model)).join("、")}；并发上限：${table.concurrency}。这些任务可能消耗积分或产生外部模型费用。`,
                    okText: "确认生成",
                    cancelText: "取消",
                    centered: true,
                    onOk: () => resolve(true),
                    onCancel: () => resolve(false),
                }),
            );
            if (!confirmed) return;

            const imageSpec = NODE_DEFAULT_SIZE[CanvasNodeType.Image];
            const nextNodes = [...nodesRef.current];
            let nextConnections = [...connectionsRef.current];
            const outputByRowId = new Map<string, string>();
            const targets: Array<{ rowId: string; nodeId: string }> = [];
            rows.forEach((row, index) => {
                const existingIndex = row.outputNodeId ? nextNodes.findIndex((node) => node.id === row.outputNodeId && node.type === CanvasNodeType.Image) : -1;
                const prompt = batchPromptForRow(table, row).trim();
                const generationSettings = isManjuAssetTable ? resolveCanvasGenerationSettings(table, row, effectiveConfig) : undefined;
                const metadata = {
                    ...(existingIndex >= 0 ? resetGenerationTaskMetadata(nextNodes[existingIndex].metadata) : {}),
                    prompt,
                    composerContent: prompt,
                    model: generationSettings?.model || fallbackGenerationConfig.model,
                    ...(generationSettings
                        ? {
                              size: generationSettings.size,
                              quality: generationSettings.quality,
                              count: generationSettings.count,
                          }
                        : {}),
                    generationMode: "image" as const,
                    generationType: "edit" as const,
                    workflowKind: "final" as const,
                    workflowTitle: `${table.operation === "try_on" ? "换装" : "创意"}任务 ${index + 1}`,
                    status: "idle" as const,
                    batchSourceNodeId: nodeId,
                    batchRowId: row.id,
                    batchOperation: table.operation,
                    batchInputNodeIds: row.inputNodeIds,
                    ...(isManjuAssetTable
                        ? {
                              manjuStage: isManjuStoryboardTable ? ("p3_images" as const) : ("p2_assets" as const),
                              manjuTaskId: isManjuStoryboardTable ? row.manjuStoryboard?.taskId : row.manjuAsset?.taskId,
                              manjuAssetCode: isManjuStoryboardTable ? undefined : row.manjuAsset?.code,
                              manjuTaskCode: isManjuStoryboardTable ? row.manjuStoryboard?.code : row.manjuAsset?.taskCode,
                              manjuAssetTableId: nodeId,
                              manjuAssetRowId: row.id,
                              manjuGlobalStylePrompt: sourceNode.metadata?.manjuGlobalStylePrompt,
                              workflowKind: isManjuStoryboardTable ? ("storyboard" as const) : row.manjuAsset?.kind === "character" ? ("character" as const) : row.manjuAsset?.kind === "scene" ? ("scene" as const) : ("final" as const),
                          }
                        : {}),
                };
                const output =
                    existingIndex >= 0
                        ? { ...nextNodes[existingIndex], metadata }
                        : createCanvasNode(
                              CanvasNodeType.Image,
                              {
                                  x: sourceNode.position.x + sourceNode.width + 120 + imageSpec.width / 2,
                                  y: sourceNode.position.y + index * (imageSpec.height + 32) + imageSpec.height / 2,
                              },
                              metadata,
                          );
                output.title = isManjuStoryboardTable
                    ? `${row.manjuStoryboard?.code || "故事板"} · ${row.manjuStoryboard?.title || "故事板图"}`
                    : isManjuAssetTable
                      ? `${row.manjuAsset?.code || "数字资产"} · ${row.manjuAsset?.name || "资产图"}`
                      : `${table.operation === "try_on" ? "换装" : "创意"} · ${index + 1}`;
                if (existingIndex >= 0) nextNodes[existingIndex] = output;
                else nextNodes.push(output);
                nextConnections = nextConnections.filter((connection) => connection.toNodeId !== output.id);
                if (!isManjuAssetTable) {
                    row.inputNodeIds.filter(Boolean).forEach((inputNodeId) => nextConnections.push({ id: nanoid(), fromNodeId: inputNodeId, toNodeId: output.id }));
                    nextConnections.push({ id: nanoid(), fromNodeId: sourceNode.id, toNodeId: output.id, relation: "batch-output", storyboardRowId: row.id });
                }
                outputByRowId.set(row.id, output.id);
                targets.push({ rowId: row.id, nodeId: output.id });
            });
            const sourceIndex = nextNodes.findIndex((node) => node.id === sourceNode.id);
            nextNodes[sourceIndex] = { ...sourceNode, metadata: { ...sourceNode.metadata, batchTable: { ...table, rows: table.rows.map((row) => (outputByRowId.has(row.id) ? { ...row, outputNodeId: outputByRowId.get(row.id) } : row)) } } };
            nodesRef.current = nextNodes;
            connectionsRef.current = nextConnections;
            setNodes(nextNodes);
            setConnections(nextConnections);
            setSelectedNodeIds(isManjuAssetTable ? new Set([nodeId]) : new Set(targets.map((target) => target.nodeId)));
            if (enqueueGenerationBatch(nodeId, "batch_image", targets, { concurrency: table.concurrency })) message.success(`${targets.length} 个任务已加入并发队列`);
        },
        [connectionsRef, effectiveConfig, enqueueGenerationBatch, isAiConfigReady, message, modal, nodesRef, setConnections, setNodes, setSelectedNodeIds],
    );

    const adoptManjuAssetInput = useCallback(
        (nodeId: string, rowId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            const row = table?.rows.find((item) => item.id === rowId);
            const inputNodeId = row?.inputNodeIds.find(Boolean);
            if (!table || !row || !inputNodeId) return;
            patchTable(nodeId, {
                rows: table.rows.map((item) => (item.id === rowId ? { ...item, outputNodeId: inputNodeId } : item)),
            });
        },
        [nodesRef, patchTable],
    );

    const deleteManjuAssetResult = useCallback(
        (nodeId: string, rowId: string) => {
            const table = nodesRef.current.find((item) => item.id === nodeId)?.metadata?.batchTable;
            const row = table?.rows.find((item) => item.id === rowId);
            if (!table || !row?.outputNodeId) return;
            const removedNodeIds = new Set(
                nodesRef.current
                    .filter((node) => node.metadata?.manjuAssetTableId === nodeId && node.metadata?.manjuAssetRowId === rowId)
                    .map((node) => node.id),
            );
            if (nodesRef.current.some((node) => node.id === row.outputNodeId && node.metadata?.manjuAssetTableId === nodeId)) {
                removedNodeIds.add(row.outputNodeId);
            }
            patchTable(nodeId, {
                rows: table.rows.map((item) =>
                    item.id === rowId
                        ? {
                              ...item,
                              inputNodeIds: item.inputNodeIds.filter((id) => !removedNodeIds.has(id)),
                              outputNodeId: undefined,
                          }
                        : item,
                ),
            });
            if (removedNodeIds.size) setNodes((current) => current.filter((node) => !removedNodeIds.has(node.id)));
            setConnections((current) =>
                current.filter(
                    (connection) =>
                        !removedNodeIds.has(connection.fromNodeId) &&
                        !removedNodeIds.has(connection.toNodeId) &&
                        !(connection.toNodeId === nodeId && connection.toHandleId === manjuAssetInputHandleId(rowId)),
                ),
            );
            message.success("当前图片已删除，可重新上传或生成");
        },
        [message, nodesRef, patchTable, setConnections, setNodes],
    );

    return {
        addReferenceColumn,
        addRow,
        addManjuRow,
        adoptManjuAssetInput,
        deleteManjuAssetResult,
        fillRowsFromConnections,
        generateRows,
        moveReferenceCell,
        patchTable,
        removeReferenceColumn,
        removeRow,
        reorderReferenceColumns,
        syncRowsFromConnections,
        updateRow,
    };
}
