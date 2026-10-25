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
        durationMs?: number;
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
import type {
    WeddingDynastyState,
    WeddingPortraitGender,
} from "@/lib/wedding/types";


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

export function weddingPortraitReference(
    dynasty: WeddingDynastyState,
    gender: WeddingPortraitGender,
    getAsset: (assetId: string) => WeddingGenerationAsset | undefined,
): ReferenceImage {
    const assetId = dynasty.portraitSelection?.[gender];
    const version = dynasty.portraitVersionHistory?.[gender]?.find((item) => item.assetId === assetId);
    const asset = assetId ? getAsset(assetId) : undefined;
    if (!assetId || !dynasty.portraitApproval?.[gender]) {
        throw new Error(`请先确认${gender === "male" ? "新郎" : "新娘"}定妆照`);
    }
    try {
        return weddingImageReference({
            assetId,
            name: `${dynasty.name}-${gender}-portrait.png`,
            asset,
            storageKey: version?.storageKey,
            previewUrl: version?.previewUrl,
        });
    } catch {
        throw new Error(`请先确认${gender === "male" ? "新郎" : "新娘"}定妆照`);
    }
}

export function weddingSceneReference(
    dynasty: WeddingDynastyState,
    getAsset: (assetId: string) => WeddingGenerationAsset | undefined,
    options: { requireQc?: boolean } = {},
): ReferenceImage {
    const assetId = dynasty.sceneSelection;
    const version = dynasty.sceneVersions?.find((item) => item.assetId === assetId);
    const qcReady = dynasty.qc.scene?.verdict === "pass" || Boolean(dynasty.qc.overrides.scene?.reason.trim());
    if (!assetId || (options.requireQc !== false && !qcReady)) {
        throw new Error("请先确认通过 QC 的场景首帧");
    }
    try {
        return weddingImageReference({
            assetId,
            name: `${dynasty.name}-scene.png`,
            asset: getAsset(assetId),
            storageKey: version?.storageKey,
            previewUrl: version?.previewUrl,
        });
    } catch {
        throw new Error("场景首帧素材不可用");
    }
}
