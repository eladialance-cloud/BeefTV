export type WeddingGenerationAsset = {
    id: string;
    kind: string;
    coverUrl?: string;
    data?: {
        dataUrl?: string;
        url?: string;
        storageKey?: string;
        width?: number;
        height?: number;
        bytes?: number;
        mimeType?: string;
    };
};

export type WeddingGenerationSummary = {
    attempted: number;
    succeeded: number;
    failed: number;
    errors: Array<{
        nodeId: string;
        dynastyId: string;
        message: string;
        gender?: "male" | "female";
    }>;
};
import type { ReferenceImage } from "@/types/image";


export function weddingImageReference(input: {
    assetId: string;
    name: string;
    asset?: WeddingGenerationAsset;
    storageKey?: string;
    previewUrl?: string;
}): ReferenceImage {
    const dataUrl = input.asset?.data?.dataUrl
        || input.asset?.data?.url
        || input.asset?.coverUrl
        || input.previewUrl
        || "";
    const storageKey = input.asset?.data?.storageKey || input.storageKey;
    if (!dataUrl || !storageKey) throw new Error(`${input.name}素材不可用`);
    return {
        id: input.assetId,
        name: input.name,
        type: input.asset?.data?.mimeType || "image/png",
        dataUrl,
        url: /^https?:\/\//i.test(dataUrl) ? dataUrl : undefined,
        storageKey,
    };
}
