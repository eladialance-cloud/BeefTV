import type { PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";
import { compactApiParams, http } from "@/services/api/request";

type PublicCreationTemplateRecord = Omit<PublicCreationTemplate, "payloadJson"> & {
    payload: unknown;
    scene?: string;
    coverUrl?: string;
    sortOrder?: number;
    updatedAt?: string;
};

export async function listPublicCreationTemplates(options: { kind?: string; scene?: string; signal?: AbortSignal } = {}) {
    const data = await http.get<{ items: PublicCreationTemplateRecord[] }>("/creation-templates", {
        signal: options.signal,
        params: compactApiParams({
            kind: options.kind || "workflow",
            scene: options.scene,
        }),
    });
    return data.items.map((item): PublicCreationTemplate => ({
        id: item.id,
        itemKey: item.itemKey,
        name: item.name,
        description: item.description,
        canvasType: item.canvasType,
        defaultModel: item.defaultModel,
        payloadJson: JSON.stringify(item.payload),
    }));
}

export type CreationPromptPreview = {
    defaultPrompt: string;
    nodeRef: string;
    dynastyId: string;
    settings: Record<string, unknown>;
};

export function previewCreationPrompt(
    templateId: string,
    input: { nodeRef: "dynastyVideo"; dynastyId: string; variables: Record<string, unknown> },
    signal?: AbortSignal,
) {
    return http.post<CreationPromptPreview>(
        `/creation-templates/${encodeURIComponent(templateId)}/prompt-preview`,
        input,
        { signal },
    );
}
