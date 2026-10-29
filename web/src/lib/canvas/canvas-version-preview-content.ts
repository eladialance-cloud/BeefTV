import { isManjuAssetTable, isManjuStoryboardTable } from "@/lib/canvas/canvas-manju-asset-table";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

export type CanvasVersionPreviewContentKind = "manju-table" | "batch-table" | "default";

export function canvasVersionPreviewContentKind(node: CanvasNodeData): CanvasVersionPreviewContentKind {
    if (node.type !== CanvasNodeType.BatchTable) return "default";
    return isManjuAssetTable(node) || isManjuStoryboardTable(node) ? "manju-table" : "batch-table";
}
