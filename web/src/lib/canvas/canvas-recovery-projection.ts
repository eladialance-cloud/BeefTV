import type { CanvasProject } from "@/stores/canvas/use-canvas-store";
import type { CanvasNodeData, CanvasNodeMetadata } from "@/types/canvas";

/**
 * A recovered task may be attached to a new server node while the local merge
 * still contains the stale error overlay. Replace that overlay with the
 * recovered node at the same visual position and remap its graph references.
 */
export function reconcileRecoveredCanvasNode(
    project: CanvasProject,
    staleNodeId: string,
    recoveredNodeId: string,
    fallbackMetadata?: Partial<CanvasNodeMetadata>,
): CanvasProject {
    const staleNodeIdValue = staleNodeId.trim();
    const recoveredNodeIdValue = recoveredNodeId.trim();
    if (!staleNodeIdValue || !recoveredNodeIdValue || staleNodeIdValue === recoveredNodeIdValue) return project;

    const staleNode = project.nodes.find((node) => node.id === staleNodeIdValue);
    const recoveredNode = project.nodes.find((node) => node.id === recoveredNodeIdValue);
    if (!recoveredNode) return project;

    const nodes = project.nodes
        .filter((node) => node.id !== staleNodeIdValue)
        .map((node): CanvasNodeData => {
            if (node.id !== recoveredNodeIdValue) return remapNodeGraphReferences(node);
            const metadata: CanvasNodeMetadata = {
                ...(fallbackMetadata || {}),
                ...(staleNode?.metadata || {}),
                ...(node.metadata || {}),
            };
            for (const key of ["errorDetails", "generationErrorCode", "resourceReloadAvailable", "failedPromptFingerprint", "failedInputFingerprint"] as const) {
                delete metadata[key];
            }
            return remapNodeGraphReferences({
                ...node,
                title: staleNode?.title || node.title,
                position: staleNode?.position || node.position,
                width: staleNode?.width || node.width,
                height: staleNode?.height || node.height,
                parentId: staleNode?.parentId ?? node.parentId,
                metadata,
            });
        });
    const connections = project.connections.map((connection) => ({
        ...connection,
        fromNodeId: connection.fromNodeId === staleNodeIdValue ? recoveredNodeIdValue : connection.fromNodeId,
        toNodeId: connection.toNodeId === staleNodeIdValue ? recoveredNodeIdValue : connection.toNodeId,
    }));
    const timeline = project.timeline && {
        ...project.timeline,
        clips: project.timeline.clips.map((clip) => (
            clip.nodeId === staleNodeIdValue ? { ...clip, nodeId: recoveredNodeIdValue } : clip
        )),
    };
    return { ...project, nodes, connections, timeline };

    function remapRecoveredNodeId(nodeId: string) {
        return nodeId === staleNodeIdValue ? recoveredNodeIdValue : nodeId;
    }

    function remapNodeGraphReferences(node: CanvasNodeData): CanvasNodeData {
        const metadata = node.metadata;
        if (!metadata) return node;
        let changed = false;
        const batchTable = metadata.batchTable && {
            ...metadata.batchTable,
            rows: metadata.batchTable.rows.map((row) => {
                const inputNodeIds = row.inputNodeIds.map(remapRecoveredNodeId);
                const outputNodeId = row.outputNodeId === staleNodeIdValue ? recoveredNodeIdValue : row.outputNodeId;
                if (inputNodeIds.every((id, index) => id === row.inputNodeIds[index]) && outputNodeId === row.outputNodeId) return row;
                changed = true;
                return { ...row, inputNodeIds, outputNodeId };
            }),
        };
        const generationBatches = metadata.generationBatches?.map((batch) => {
            let batchChanged = false;
            const items = batch.items.map((item) => {
                if (item.nodeId !== staleNodeIdValue) return item;
                batchChanged = true;
                changed = true;
                return { ...item, nodeId: recoveredNodeIdValue };
            });
            return batchChanged ? { ...batch, items, updatedAt: new Date().toISOString() } : batch;
        });
        const primaryImageId = metadata.primaryImageId === staleNodeIdValue ? recoveredNodeIdValue : metadata.primaryImageId;
        const batchChildIds = metadata.batchChildIds?.map(remapRecoveredNodeId);
        const childIdsChanged = Boolean(metadata.batchChildIds?.some((id, index) => id !== batchChildIds?.[index]));
        if (primaryImageId !== metadata.primaryImageId || childIdsChanged) changed = true;
        if (!changed) return node;
        return {
            ...node,
            metadata: {
                ...metadata,
                ...(batchTable ? { batchTable } : {}),
                ...(generationBatches ? { generationBatches } : {}),
                ...(primaryImageId ? { primaryImageId } : {}),
                ...(batchChildIds ? { batchChildIds } : {}),
            },
        };
    }
}
