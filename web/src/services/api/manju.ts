import { http } from "@/services/api/request";

export type ManjuMemberRole = "owner" | "producer" | "writer" | "director" | "reviewer" | "viewer";

export type ManjuStyleBaseID = "live-action" | "2d-animation" | "3d-cg" | "ink-2d";
export type ManjuLockKey = "track" | "source" | "videoModel" | "aspectRatio" | "deliveryMode" | "styleCard" | "seamMode";
export type ManjuTrack = "domestic" | "overseas" | "explainer" | "other";
export type ManjuSource = "original" | "adaptation" | "benchmark" | "trend";
export type ManjuVideoModel = "seedance-2.5" | "seedance-2.0" | "minimax-h3";
export type ManjuAspectRatio = "16:9" | "9:16" | "21:9";
export type ManjuDeliveryMode = "board" | "markdown" | "both";
export type ManjuSeamMode = "" | "tail-frame-freeze" | "hard-cut" | "insert-shot" | "creative-transition";

export type ManjuStyleBase = {
    id: ManjuStyleBaseID;
    name: string;
};

export type ManjuStyleFamily = {
    id: string;
    name: string;
    feature: string;
};

export type ManjuStyleCard = {
    id: string;
    number: number;
    name: string;
    group: string;
    family: string;
    fightRating: number;
    effectRating: number;
    grade: "A" | "B" | "C";
    nativeGenre: string;
    promptParameters: string;
    avoidanceWarning: string;
    compatibleStyleBases: ManjuStyleBaseID[];
};

export type ManjuStyleCatalog = {
    version: string;
    bases: ManjuStyleBase[];
    families: ManjuStyleFamily[];
    items: ManjuStyleCard[];
};

export type ManjuProjectSummary = {
    id: string;
    name: string;
    type: string;
    aspectRatio: string;
    sourceType: string;
    status: string;
    accessRole: ManjuMemberRole;
    revision: number;
    updatedAt: string;
};

export type ManjuProject = {
    id: string;
    userId: string;
    name: string;
    type: string;
    aspectRatio: string;
    sourceType: string;
    description: string;
    stylePresetId: string;
    styleProfileJson?: string;
    defaultImageModel?: string;
    defaultVideoModel?: string;
    status: string;
    revision: number;
    createdAt: string;
    updatedAt: string;
};

export type ManjuMember = {
    id: string;
    projectId: string;
    userId: string;
    role: Exclude<ManjuMemberRole, "owner">;
    status: "active" | "disabled";
    invitedBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuLockInput = {
    track: ManjuTrack | "";
    source: ManjuSource | "";
    videoModel: ManjuVideoModel | "";
    aspectRatio: ManjuAspectRatio | "";
    deliveryMode: ManjuDeliveryMode | "";
    seamMode?: ManjuSeamMode;
    targetDurationSeconds: number;
    styleBase: ManjuStyleBaseID | "";
    styleCardId: string;
    styleCustomDescription?: string;
    styleVisualPrompt?: string;
    styleMotionParameters?: string;
    styleForbiddenItems?: string[];
    styleReferenceResourceId?: string;
    styleReferenceAnalysis?: string;
    styleReferencePalette?: string;
    assetMode?: "M0" | "M1" | "M2";
    confirmedLocks: ManjuLockKey[];
};

export type ManjuLockSnapshot = Omit<ManjuLockInput, "seamMode" | "confirmedLocks"> & {
    seamMode?: ManjuLockInput["seamMode"];
    confirmedLocks?: ManjuLockKey[];
    segmentCount: number;
    singleSegmentMaxSeconds: number;
    styleCardName: string;
    styleFamily: string;
    styleFightRating: number;
    styleEffectRating: number;
    styleGrade: string;
    styleCatalogVersion: string;
    stylePromptParameters: string;
    styleImagePrompt?: string;
    stylePerformancePrompt?: string;
    styleReferenceResourceId?: string;
    styleReferenceStorageKey?: string;
    styleReferenceAnalysis?: string;
    styleReferencePalette?: string;
};

export type ManjuLockReceipt = {
    id: string;
    projectId: string;
    revision: number;
    status: "locked" | "superseded";
    snapshot: ManjuLockSnapshot;
    segmentCount: number;
    contentHash: string;
    createdBy: string;
    createdAt: string;
};

export type ManjuModelSelection = {
    channelId: string;
    modelKey: string;
};

export type ManjuProjectModelBinding = {
    id: string;
    projectId: string;
    text: ManjuModelSelection;
    image: ManjuModelSelection;
    video: ManjuModelSelection;
    updatedBy: string;
    updatedAt: string;
};

export type ManjuProjectDetail = {
    project: ManjuProject;
    accessRole: ManjuMemberRole;
    members?: ManjuMember[];
    lock?: ManjuLockReceipt;
    models?: ManjuProjectModelBinding;
};

export type ManjuProjectStateView = {
    id: string;
    lockRevision: number;
    stageKey: string;
    schemaVersion: string;
    state: Record<string, unknown>;
    contentHash: string;
    createdBy: string;
    createdAt: string;
};

export type ManjuRuleLibraryStatus = {
    version: string;
    embedded: boolean;
    fileCount: number;
    contentHash: string;
    files: string[];
    officialScripts: string[];
    projectState?: ManjuProjectStateView;
};

export type ManjuUserDeliverable = {
    artifactId: string;
    phase: "P2" | "P3" | "P4";
    kind: string;
    version: number;
    content: string;
    contentHash: string;
    createdAt: string;
};

export type ManjuScriptMode = "STEPWISE" | "FULL_SCRIPT_PIPELINE";
export type ManjuScriptBranch = "original" | "adaptation" | "benchmark" | "polish";
export type ManjuScriptInputKind = "novel" | "outline" | "finished_script";
export type ManjuTrendResolution = "" | "original" | "benchmark";
export type ManjuRoundtableDecision = "PROCEED_CURRENT" | "REFINE_OR_REWRITE";
export type ManjuRoundtableGrade = "S" | "A" | "B" | "NOT_READY" | "UNKNOWN";
export type ManjuScriptStageStatus = "NOT_STARTED" | "IN_PROGRESS" | "AWAITING_CONFIRMATION" | "AWAITING_USER_DECISION" | "LOCKED" | "STALE" | "BLOCKED";
export type ManjuScriptStageKey =
    | "source_registration"
    | "recon_intelligence"
    | "ip_development"
    | "series_bible"
    | "episode_planning"
    | "script_draft"
    | "roundtable_review"
    | "polish_draft"
    | "polish_findings"
    | "polish_confirmation";
export type ManjuScriptArtifactKind =
    | "SOURCE_MANIFEST.json"
    | "RECON_INTELLIGENCE_CARD.md"
    | "IP_DEVELOPMENT.md"
    | "SERIES_BIBLE.md"
    | "EPISODE_PLAN.md"
    | "SCRIPT_DRAFT.md"
    | "ROUNDTABLE_REVIEW.md"
    | "POLISHED_SCRIPT.md"
    | "POLISH_FINDINGS.md"
    | "POLISH_CONFIRM_CARD.md"
    | "STAGE_OUTPUT.md";

export type ManjuScriptRun = {
    id: string;
    projectId: string;
    branch: ManjuScriptBranch;
    mode: ManjuScriptMode;
    inputKind?: "" | ManjuScriptInputKind;
    trendResolution?: ManjuTrendResolution;
    status: "ACTIVE" | "COMPLETED" | "ABANDONED";
    currentStage: string;
    lockRevision: number;
    sourceId?: string;
    unitTitle: string;
    sourceDocumentId?: string;
    sourceChapterIds: string[];
    sourceRangeLabel: string;
    sourceSetHash: string;
    baseRunId?: string;
    reuseGlobalStages: boolean;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuSourceChapter = {
    id: string;
    projectId: string;
    documentId: string;
    position: number;
    title: string;
    rangeLabel: string;
    contentHash: string;
    characterCount: number;
    createdAt: string;
};

export type ManjuSourceDocument = {
    id: string;
    projectId: string;
    title: string;
    authorizationScope: string;
    fileName: string;
    contentHash: string;
    characterCount: number;
    chapterCount: number;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuSourceDocumentDetail = {
    document: ManjuSourceDocument;
    chapters: ManjuSourceChapter[];
};

export type ManjuSourceLibraryState = {
    contentHash: string;
    documentCount: number;
    chapterCount: number;
};

export type ManjuScriptSource = {
    id: string;
    projectId: string;
    runId: string;
    branch: ManjuScriptBranch;
    title: string;
    authorizationScope: string;
    rangeLabel: string;
    contentHash: string;
    characterCount: number;
    extractionStatus: string;
    metadata?: Record<string, string>;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuScriptStage = {
    id: string;
    key: ManjuScriptStageKey;
    title: string;
    position: number;
    status: ManjuScriptStageStatus;
    version: number;
    taskId?: string;
    artifactId?: string;
    error?: string;
    confirmedBy?: string;
    confirmedAt?: string;
    startedAt?: string;
    completedAt?: string;
    updatedAt: string;
    draft?: ManjuStageDraft;
    reviewGrade?: ManjuRoundtableGrade;
};

export type ManjuStageDraft = {
    id: string;
    version: number;
    content: string;
    contentHash: string;
    validationJson: string;
    source: "MODEL_FAILED" | "MANUAL_FAILED" | "MANUAL_PASSED";
    createdBy: string;
    createdAt: string;
};

export type ManjuScriptArtifact = {
    id: string;
    stageId: string;
    kind: ManjuScriptArtifactKind;
    version: number;
    content: string;
    contentHash: string;
    taskId?: string;
    createdAt: string;
};

export type ManjuScriptRunDetail = {
    run: ManjuScriptRun;
    source?: ManjuScriptSource;
    stages: ManjuScriptStage[];
    artifacts?: ManjuScriptArtifact[];
    roundtableDecisions?: ManjuRoundtableDecisionRecord[];
};

export type ManjuRoundtableDecisionRecord = {
    id: string;
    projectId: string;
    runId: string;
    stageId: string;
    round: number;
    version: number;
    decision: ManjuRoundtableDecision;
    actorId: string;
    note?: string;
    reviewArtifactId: string;
    reviewArtifactHash: string;
    grade: ManjuRoundtableGrade;
    riskAccepted: boolean;
    createdAt: string;
};

export type ManjuScriptSourceInput = {
    title: string;
    authorizationScope: string;
    rangeLabel: string;
    content: string;
    metadata?: Record<string, string>;
};

export type ManjuScriptRunInput = {
    mode: ManjuScriptMode;
    inputKind?: ManjuScriptInputKind;
    trendResolution?: ManjuTrendResolution;
    title?: string;
    sourceDocumentId?: string;
    chapterIds?: string[];
    baseRunId?: string;
    reuseGlobalStages?: boolean;
};

export type ManjuAssetMode = "M0" | "M1" | "M2";
export type ManjuAssetStageStatus = "NOT_STARTED" | "IN_PROGRESS" | "AWAITING_CONFIRMATION" | "LOCKED" | "STALE" | "BLOCKED";

export type ManjuAssetRun = {
    id: string;
    projectId: string;
    scriptRunId: string;
    baseAssetRunId?: string;
    reuseAssets: boolean;
    inheritedAssetCount: number;
    assetMode: ManjuAssetMode;
    status: "ACTIVE" | "COMPLETED" | "SUPERSEDED";
    currentStage: string;
    lockRevision: number;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuAssetStage = {
    id: string;
    key: string;
    title: string;
    position: number;
    status: ManjuAssetStageStatus;
    version: number;
    taskId?: string;
    artifactId?: string;
    error?: string;
    confirmedBy?: string;
    confirmedAt?: string;
    startedAt?: string;
    completedAt?: string;
    updatedAt: string;
    draft?: ManjuStageDraft;
};

export type ManjuAssetArtifact = {
    id: string;
    stageId: string;
    kind: string;
    version: number;
    content: string;
    contentHash: string;
    taskId?: string;
    createdAt: string;
};

export type ManjuAssetReference = {
    id: string;
    code: string;
    assetType: string;
    resourceId: string;
    url?: string;
    status: "PENDING" | "READY";
    notes?: string;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuAssetRunDetail = {
    run: ManjuAssetRun;
    scriptRun: ManjuScriptRun;
    stages: ManjuAssetStage[];
    artifacts?: ManjuAssetArtifact[];
    references?: ManjuAssetReference[];
};

export type ManjuAssetReferenceInput = {
    code: string;
    assetType: string;
    resourceId: string;
    url?: string;
    notes?: string;
};

export type ManjuProductionStageStatus = "NOT_STARTED" | "IN_PROGRESS" | "AWAITING_CONFIRMATION" | "LOCKED" | "BLOCKED" | "STALE";

export type ManjuProductionRun = {
    id: string;
    projectId: string;
    assetRunId: string;
    status: "ACTIVE" | "COMPLETED" | "SUPERSEDED";
    currentStage: string;
    lockRevision: number;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuProductionStage = {
    id: string;
    key: string;
    title: string;
    position: number;
    status: ManjuProductionStageStatus;
    version: number;
    taskId?: string;
    artifactId?: string;
    error?: string;
    confirmedBy?: string;
    confirmedAt?: string;
    startedAt?: string;
    completedAt?: string;
    updatedAt: string;
    draft?: ManjuStageDraft;
    machineCheckStatus?: "PASS" | "LEGACY_NON_COMPLIANT";
    machineCheckNote?: string;
    machineCheckWarnings?: string[];
    machineCheckProofVerified?: boolean;
};

export type ManjuCompiledSegmentPreview = {
    segmentCode: string;
    totalChars: number;
    characters: number;
    sceneProps: number;
    shots: number;
    style: number;
    bansAndOther: number;
    body: string;
    overHardLimit: boolean;
};

export type ManjuCompilePreview = {
    model: string;
    hardLimit: number;
    passed: boolean;
    issues?: string[];
    warnings?: string[];
    feed: string;
    segments: ManjuCompiledSegmentPreview[];
};

export type ManjuProductionArtifact = {
    id: string;
    stageId: string;
    kind: string;
    version: number;
    content: string;
    contentHash: string;
    taskId?: string;
    createdAt: string;
};

export type ManjuProductionImageStatus = "QUEUED" | "RUNNING" | "READY" | "FAILED" | "SUPERSEDED";

export type ManjuProductionPrevisStatus = "QUEUED" | "RUNNING" | "READY" | "FAILED";

export type ManjuProductionPrevis = {
    id: string;
    runId: string;
    taskId?: string;
    artifactId?: string;
    status: ManjuProductionPrevisStatus;
    version: number;
    error?: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuProductionImage = {
    id: string;
    runId: string;
    artifactId?: string;
    position: number;
    segmentCode: string;
    shotLabel?: string;
    attempt: number;
    status: ManjuProductionImageStatus;
    taskId?: string;
    resourceId?: string;
    url?: string;
    error?: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuProductionVideoStatus = "QUEUED" | "RUNNING" | "READY" | "FAILED";

export type ManjuProductionVideo = {
    id: string;
    position: number;
    segmentCode: string;
    durationSeconds: number;
    attempt: number;
    status: ManjuProductionVideoStatus;
    taskId?: string;
    resourceId?: string;
    url?: string;
    mimeType?: string;
    error?: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuProductionRunDetail = {
    run: ManjuProductionRun;
    assetRun: ManjuAssetRun;
    scriptRun: ManjuScriptRun;
    stages: ManjuProductionStage[];
    artifacts?: ManjuProductionArtifact[];
    previs?: ManjuProductionPrevis;
    images?: ManjuProductionImage[];
    videos?: ManjuProductionVideo[];
};

export type ManjuCanvasPackageStage = "p2_assets" | "p3_images" | "p4_videos";

export type ManjuCanvasTask = {
    id: string;
    kind: "image" | "video";
    code: string;
    assetCode?: string;
    title: string;
    prompt: string;
    category: "character" | "environment" | "prop" | "material" | "other";
    tags?: string[];
    aspectRatio?: string;
    seconds?: number;
    referenceCodes?: string[];
    globalStylePrompt?: string;
    sourceKind: string;
    group?: string;
};

export type ManjuCanvasPackage = {
    version: number;
    projectId: string;
    stage: ManjuCanvasPackageStage;
    assetRunId?: string;
    productionRunId?: string;
    sourceArtifact: string;
    title: string;
    hash: string;
    style?: {
        cardId?: string;
        cardName?: string;
        styleBase?: string;
        aspectRatio?: string;
        promptParameters?: string;
        imageStylePrompt?: string;
        performancePrompt?: string;
        globalStylePrompt?: string;
        referenceResourceId?: string;
        referenceAssetId?: string;
        referenceStorageKey?: string;
        referenceAnalysis?: string;
        referencePalette?: string;
    };
    tasks: ManjuCanvasTask[];
    exemptions?: ManjuCanvasExemption[];
};

export type ManjuCanvasExemption = {
    kind: "storyboard" | "plot";
    code: string;
    reason: string;
};

export type ManjuCanvasImport = {
    id: string;
    projectId: string;
    stage: ManjuCanvasPackageStage;
    assetRunId?: string;
    productionRunId?: string;
    canvasId: string;
    packageHash: string;
    taskCount: number;
    status: string;
    createdBy: string;
    createdAt: string;
    updatedAt: string;
};

export type ManjuCanvasImportReference = {
    code: string;
    assetCode?: string;
    assetId?: string;
    taskId?: string;
    title: string;
    prompt?: string;
    content?: string;
    storageKey?: string;
    category?: string;
    kind: "image" | "video";
};

export function listManjuProjects() {
    return http.get<{ items: ManjuProjectSummary[] }>("/manju/projects");
}

export function deleteManjuProject(projectId: string) {
    return http.delete<{ deleted: boolean; id: string }>(`/manju/projects/${encodeURIComponent(projectId)}`);
}

export function getManjuProject(projectId: string) {
    return http.get<{ item: ManjuProjectDetail }>(`/manju/projects/${encodeURIComponent(projectId)}`);
}

export function getManjuStyleCatalog() {
    return http.get<ManjuStyleCatalog>("/manju/style-catalog");
}

export function getManjuRuleLibrary(projectId: string) {
    return http.get<{ item: ManjuRuleLibraryStatus }>(`/manju/projects/${encodeURIComponent(projectId)}/rule-library`);
}

export function getManjuUserDeliverables(projectId: string) {
    return http.get<{ items: ManjuUserDeliverable[] }>(`/manju/projects/${encodeURIComponent(projectId)}/deliverables`);
}

export function saveManjuLock(projectId: string, input: ManjuLockInput) {
    return http.put<{ item: ManjuLockReceipt }>(`/manju/projects/${encodeURIComponent(projectId)}/lock`, input);
}

export function saveManjuModels(projectId: string, input: Pick<ManjuProjectModelBinding, "text" | "image" | "video">) {
    return http.put<{ item: ManjuProjectModelBinding }>(`/manju/projects/${encodeURIComponent(projectId)}/models`, input);
}

export function inviteManjuMember(projectId: string, input: { userId: string; role: Exclude<ManjuMemberRole, "owner"> }) {
    return http.post<{ item: ManjuMember }>(`/manju/projects/${encodeURIComponent(projectId)}/members`, input);
}

export function removeManjuMember(projectId: string, memberId: string) {
    return http.delete<{ deleted: boolean }>(`/manju/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(memberId)}`);
}

export function listManjuScriptRuns(projectId: string) {
    return http.get<{ items: ManjuScriptRun[] }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs`);
}

export function listManjuSourceDocuments(projectId: string) {
    return http.get<{ items: ManjuSourceDocument[] }>(`/manju/projects/${encodeURIComponent(projectId)}/source-documents`);
}

export function getManjuSourceDocument(projectId: string, documentId: string) {
    return http.get<{ item: ManjuSourceDocumentDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/source-documents/${encodeURIComponent(documentId)}`);
}

export function getManjuSourceLibraryState(projectId: string) {
    return http.get<{ item: ManjuSourceLibraryState }>(`/manju/projects/${encodeURIComponent(projectId)}/source-library-state`);
}

export function createManjuSourceDocument(projectId: string, input: { title: string; authorizationScope: string; fileName: string; content: string }) {
    return http.post<{ item: ManjuSourceDocumentDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/source-documents`, input);
}

export function createManjuScriptRun(
    projectId: string,
    input: ManjuScriptRunInput,
) {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs`, input);
}

export function getManjuScriptRun(projectId: string, runId: string) {
    return http.get<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}`);
}

export function registerManjuScriptSource(projectId: string, runId: string, input: ManjuScriptSourceInput) {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/source`, input);
}

export function runManjuScriptStage(projectId: string, runId: string, stageKey: string, input: { instruction: string }) {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/run`, input);
}

export function regenerateManjuScriptArtifact(projectId: string, runId: string, stageKey: string, instruction = "") {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/regenerate-artifact`, { instruction });
}

export function manualCorrectManjuScriptArtifact(projectId: string, runId: string, stageKey: string, input: { content: string; note?: string }) {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/manual-correct`, input);
}

export function confirmManjuScriptStage(projectId: string, runId: string, stageKey: string) {
    return http.post<{ item: ManjuScriptRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/confirm`, {});
}

export function decideManjuRoundtable(
    projectId: string,
    runId: string,
    input: { decision: ManjuRoundtableDecision; acceptQualityRisk?: boolean; note?: string },
) {
    return http.post<{ item: ManjuScriptRunDetail }>(
        `/manju/projects/${encodeURIComponent(projectId)}/script-runs/${encodeURIComponent(runId)}/stages/roundtable_review/decide`,
        input,
    );
}

export function createManjuProject(input: { name: string; aspectRatio: ManjuAspectRatio; sourceType: Exclude<ManjuSource, ""> }) {
    return http.post<{ project: ManjuProject }>("/projects", { ...input, type: "manju" });
}

export function listManjuAssetRuns(projectId: string) {
    return http.get<{ items: ManjuAssetRun[] }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs`);
}

export function createManjuAssetRun(
    projectId: string,
    input: {
        scriptRunId: string;
        assetMode?: ManjuAssetMode;
        baseAssetRunId?: string;
        reuseAssets?: boolean;
    },
) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs`, input);
}

export function regenerateManjuAssetRun(
    projectId: string,
    runId: string,
    input: { instruction?: string; reuseAssets?: boolean },
) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/regenerate`, input);
}

export function regenerateManjuAssetArtifact(projectId: string, runId: string, stageKey: string, instruction = "") {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/regenerate-artifact`, { instruction });
}

export function manualCorrectManjuAssetArtifact(projectId: string, runId: string, stageKey: string, input: { content: string; note?: string }) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/manual-correct`, input);
}

export function getManjuAssetRun(projectId: string, runId: string) {
    return http.get<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}`);
}

export function runManjuAssetStage(projectId: string, runId: string, stageKey: string, input: { instruction: string }) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/run`, input);
}

export function confirmManjuAssetManifest(projectId: string, runId: string) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/confirm-manifest`, {});
}

export function confirmManjuAssetRun(projectId: string, runId: string, note = "") {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/confirm`, { note });
}

export function registerManjuAssetReference(projectId: string, runId: string, input: ManjuAssetReferenceInput) {
    return http.post<{ item: ManjuAssetRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/asset-runs/${encodeURIComponent(runId)}/references`, input);
}

export function createManjuProductionRun(projectId: string, input: { assetRunId: string }) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs`, input);
}

export function listManjuProductionRuns(projectId: string) {
    return http.get<{ items: ManjuProductionRun[] }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs`);
}

export function getManjuProductionRun(projectId: string, runId: string) {
    return http.get<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}`);
}

export function runManjuProductionStage(projectId: string, runId: string, stageKey: string, input: { instruction: string }) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/run`, input);
}

export function regenerateManjuProductionStage(projectId: string, runId: string, stageKey: string) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/regenerate`, {});
}

export function regenerateManjuProductionArtifact(projectId: string, runId: string, stageKey: string, instruction = "") {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/regenerate-artifact`, { instruction });
}

export function manualCorrectManjuProductionArtifact(projectId: string, runId: string, stageKey: string, input: { content: string; note?: string }) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/manual-correct`, input);
}

export function previewManjuProductionStoryboardCompile(projectId: string, runId: string, content: string) {
    return http.post<{ item: ManjuCompilePreview }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/storyboard-compile-preview`, { content });
}

export function generateManjuProductionPrevis(projectId: string, runId: string) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/previs-storyboard`, {});
}

export function confirmManjuProductionStage(projectId: string, runId: string, stageKey: string, note = "") {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/stages/${encodeURIComponent(stageKey)}/confirm`, { note });
}

export function startManjuStoryboardImages(projectId: string, runId: string) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/storyboard-images`, {});
}

export function startManjuProductionVideos(projectId: string, runId: string) {
    return http.post<{ item: ManjuProductionRunDetail }>(`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/videos`, {});
}

export function getManjuCanvasPackage(projectId: string, stage: ManjuCanvasPackageStage, runId: string) {
    return http.get<{ item: ManjuCanvasPackage }>(`/manju/projects/${encodeURIComponent(projectId)}/canvas-packages/${encodeURIComponent(stage)}`, { params: { runId } });
}

export function listManjuCanvasImports(projectId: string) {
    return http.get<{ items: ManjuCanvasImport[] }>(`/manju/projects/${encodeURIComponent(projectId)}/canvas-imports`);
}

export function saveManjuCanvasImport(projectId: string, input: { stage: ManjuCanvasPackageStage; assetRunId?: string; productionRunId?: string; canvasId: string; packageHash: string }) {
    return http.post<{ item: ManjuCanvasImport }>(`/manju/projects/${encodeURIComponent(projectId)}/canvas-imports`, input);
}

export function getManjuCanvasImportReferences(projectId: string, stage: ManjuCanvasPackageStage) {
    return http.get<{ items: ManjuCanvasImportReference[] }>(`/manju/projects/${encodeURIComponent(projectId)}/canvas-imports/${encodeURIComponent(stage)}/references`);
}
