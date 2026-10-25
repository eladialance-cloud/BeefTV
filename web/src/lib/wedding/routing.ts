import type { PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";

export function weddingWorkbenchPathForTemplate(
    template: Pick<PublicCreationTemplate, "itemKey">,
    canvasId: string,
) {
    return template.itemKey === "cross-era-wedding-volc"
        ? weddingWorkbenchCanvasPath(canvasId)
        : `/canvas/${encodeURIComponent(canvasId)}`;
}

export function weddingWorkbenchCanvasPath(canvasId: string) {
    return `/canvas/${encodeURIComponent(canvasId)}?canvasView=wedding`;
}

export function legacyWeddingWorkbenchPath(canvasId: string) {
    return weddingWorkbenchCanvasPath(canvasId);
}

export function isWeddingCanvasView(search: string | URLSearchParams) {
    const params = typeof search === "string" ? new URLSearchParams(search) : search;
    return params.get("canvasView") === "wedding";
}
