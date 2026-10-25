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
