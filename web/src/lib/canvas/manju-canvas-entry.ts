import type { ManjuCanvasImport } from "@/services/api/manju";

export function primaryManjuProjectCanvas(imports: readonly ManjuCanvasImport[]) {
    return imports.find((item) => item.stage === "p2_assets")
        || imports.find((item) => item.stage === "p3_images")
        || imports.find((item) => item.stage === "p4_videos");
}

export function manjuP5CanvasPath(projectId: string, canvasId: string) {
    return `/canvas/${encodeURIComponent(canvasId)}?from=manju&manjuProjectId=${encodeURIComponent(projectId)}&stage=p5`;
}
