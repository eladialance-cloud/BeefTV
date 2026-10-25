export const WEDDING_STAGE_IDS = [
    "setup",
    "dynasties",
    "portraits",
    "scenes",
    "videos",
    "packaging",
    "delivery",
] as const;

export type WeddingStageId = (typeof WEDDING_STAGE_IDS)[number];

export type WeddingItemStatus =
    | "not_started"
    | "ready"
    | "generating"
    | "needs_review"
    | "approved"
    | "failed"
    | "stale";

export type WeddingStageState = {
    status: WeddingItemStatus;
    updatedAt: string;
    approvedAt?: string;
};

export type WeddingSceneQc = {
    verdict: "pass" | "fail" | "error";
    couplePresent: boolean;
    faceMatchMale: boolean;
    faceMatchFemale: boolean;
    outfitMatch: boolean;
    sceneMatch: boolean;
    extraPerson: boolean;
    watermarkOrText: boolean;
    reason: string;
    suggestion: string;
    checkedAt: string;
};

export type WeddingVideoQc = {
    verdict: "pass" | "fail" | "error" | "pending";
    faceStable: boolean;
    outfitStable: boolean;
    actionReadable: boolean;
    extraPerson: boolean;
    watermarkOrText: boolean;
    reason: string;
    checkedAt: string;
};

export type WeddingQcOverride = {
    reason: string;
    acceptedAt: string;
};

export type WeddingPortraitGender = "male" | "female";

export type WeddingPortraitVersion = {
    assetId: string;
    nodeId: string;
    storageKey?: string;
    previewUrl?: string;
    taskId?: string;
    createdAt: string;
};

export type WeddingDynastyState = {
    id: string;
    name: string;
    order: number;
    portraitMaleNodeId: string;
    portraitFemaleNodeId: string;
    sceneNodeId: string;
    videoNodeId: string;
    portraitSelection: { male?: string; female?: string };
    portraitApproval?: Partial<Record<WeddingPortraitGender, boolean>>;
    portraitVersionHistory?: Partial<Record<WeddingPortraitGender, WeddingPortraitVersion[]>>;
    sceneSelection?: string;
    videoSelection?: string;
    promptOverride?: string;
    promptOverrideUpdatedAt?: string;
    statuses: Record<"portrait" | "scene" | "video", WeddingItemStatus>;
    qc: {
        scene?: WeddingSceneQc;
        video?: WeddingVideoQc;
        overrides: Partial<Record<"scene" | "video", WeddingQcOverride>>;
    };
};

export type WeddingPackagingState = {
    status: WeddingItemStatus;
    intro: { text: string; durationSeconds: number; enabled: boolean };
    outro: { text: string; durationSeconds: number; enabled: boolean };
    subtitlesEnabled: boolean;
    transitionSeconds: number;
    bgmMode: "generated" | "uploaded" | "none";
    bgmNodeId?: string;
    titleCardNodeIds: string[];
    timelineRevision?: number;
    outputNodeId?: string;
    approvedAt?: string;
};

export type WeddingActivityEvent = {
    id: string;
    type:
        | "stage_changed"
        | "asset_generated"
        | "asset_selected"
        | "asset_approved"
        | "prompt_changed"
        | "asset_staled"
        | "render_finished"
        | "error";
    stage: WeddingStageId;
    dynastyId?: string;
    message: string;
    createdAt: string;
};

export type WeddingProductionState = {
    schemaVersion: 1;
    template: { id: string; key: string; version: string };
    activeStage: WeddingStageId;
    stages: Record<WeddingStageId, WeddingStageState>;
    source: {
        groomPhotoNodeId: string;
        bridePhotoNodeId: string;
        maleName: string;
        femaleName: string;
        story: string;
        consentConfirmed: boolean;
        ratio: string;
        resolution: string;
    };
    dynasties: WeddingDynastyState[];
    packaging: WeddingPackagingState;
    activity: WeddingActivityEvent[];
};
