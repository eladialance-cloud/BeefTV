import type { PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";

export function weddingWorkbenchPathForTemplate(
    template: Pick<PublicCreationTemplate, "itemKey">,
    canvasId: string,
) {
    return template.itemKey === "cross-era-wedding-volc"
        ? `/wedding/${encodeURIComponent(canvasId)}`
        : `/canvas/${encodeURIComponent(canvasId)}`;
}
