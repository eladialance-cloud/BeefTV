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
    return `/wedding/${encodeURIComponent(canvasId)}`;
}

export function legacyWeddingWorkbenchPath(canvasId: string) {
    return weddingWorkbenchCanvasPath(canvasId);
}

export function isWeddingCanvasView(search: string | URLSearchParams) {
    const params = typeof search === "string" ? new URLSearchParams(search) : search;
    return params.get("canvasView") === "wedding";
}

export function weddingTemplateInitialVariables(
    template: Pick<PublicCreationTemplate, "payloadJson">,
): Record<string, unknown> {
    let payload: Record<string, unknown> = {};
    try {
        const parsed = JSON.parse(template.payloadJson);
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
            payload = parsed as Record<string, unknown>;
        }
    } catch {
        // Creator validation reports malformed payloads; creation uses safe defaults.
    }
    const variables = payload.variables && typeof payload.variables === "object" && !Array.isArray(payload.variables)
        ? payload.variables as Record<string, unknown>
        : {};
    const selectedDynasties = Array.isArray(variables.dynasties)
        ? variables.dynasties.filter((value): value is string => typeof value === "string" && Boolean(value.trim()))
        : [];
    return {
        ...variables,
        maleName: typeof variables.maleName === "string" && variables.maleName.trim() ? variables.maleName : "新郎",
        femaleName: typeof variables.femaleName === "string" && variables.femaleName.trim() ? variables.femaleName : "新娘",
        customDesc: typeof variables.customDesc === "string" ? variables.customDesc : "",
        dynasties: selectedDynasties.length >= 2 ? selectedDynasties : ["tang", "ming", "modern"],
        ratio: variables.ratio === "9:16" ? "9:16" : "16:9",
        duration: Number(variables.duration) || 8,
        resolution: ["480p", "720p", "1080p"].includes(String(variables.resolution)) ? variables.resolution : "720p",
        consentConfirmed: false,
    };
}
