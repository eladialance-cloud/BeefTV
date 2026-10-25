import type {
    WeddingActivityEvent,
    WeddingProductionState,
    WeddingVideoVersion,
} from "@/lib/wedding/types";

export type WeddingVideoJob = {
    nodeId: string;
    dynastyId: string;
    kind: "video";
};

export type WeddingVideoVersionInput = Omit<WeddingVideoVersion, "nodeId">;

export function weddingVideoJobs(
    production: WeddingProductionState,
    dynastyId?: string,
): WeddingVideoJob[] {
    return production.dynasties
        .filter((dynasty) => !dynastyId || dynasty.id === dynastyId)
        .sort((left, right) => left.order - right.order)
        .map((dynasty) => ({
            nodeId: dynasty.videoNodeId,
            dynastyId: dynasty.id,
            kind: "video" as const,
        }));
}

export function markWeddingVideoGenerating(
    state: WeddingProductionState,
    job: WeddingVideoJob,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty || dynasty.statuses.video === "generating") return state;
    dynasty.statuses.video = "generating";
    dynasty.qc.video = pendingVideoQc("等待自动 QC", now);
    next.stages.videos = { status: "generating", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "stage_changed",
        stage: "videos",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：朝代视频开始生成`,
    }));
    return next;
}

export function recordWeddingVideoVersion(
    state: WeddingProductionState,
    job: WeddingVideoJob,
    version: WeddingVideoVersionInput,
    now = new Date().toISOString(),
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;
    dynasty.videoVersions = [
        { ...version, nodeId: job.nodeId },
        ...(dynasty.videoVersions || []).filter((item) => item.assetId !== version.assetId),
    ].slice(0, 12);
    dynasty.videoSelection = version.assetId;
    dynasty.qc.video = pendingVideoQc("等待自动 QC", now);
    dynasty.statuses.video = "needs_review";
    next.stages.videos = { status: "needs_review", updatedAt: now };
    next.packaging = { ...next.packaging, status: "stale" };
    for (const stage of ["packaging", "delivery"] as const) {
        next.stages[stage] = { ...next.stages[stage], status: "stale" };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_generated",
        stage: "videos",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：朝代视频已生成，等待 QC 与确认`,
    }));
    return next;
}

export function selectWeddingVideoVersion(
    state: WeddingProductionState,
    dynastyId: string,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;
    dynasty.videoSelection = assetId;
    dynasty.qc.video = pendingVideoQc("已切换版本，等待自动 QC", now);
    dynasty.statuses.video = "needs_review";
    next.stages.videos = { status: "needs_review", updatedAt: now };
    next.packaging = { ...next.packaging, status: "stale" };
    for (const stage of ["packaging", "delivery"] as const) {
        next.stages[stage] = { ...next.stages[stage], status: "stale" };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "videos",
        dynastyId,
        message: `${dynasty.name}：已切换视频版本，需要重新 QC`,
    }));
    return next;
}

export function confirmWeddingVideo(
    state: WeddingProductionState,
    dynastyId: string,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;
    dynasty.videoSelection = assetId;
    const qcReady = dynasty.qc.video?.verdict === "pass"
        || Boolean(dynasty.qc.overrides.video?.reason.trim());
    dynasty.statuses.video = qcReady ? "ready" : "needs_review";
    next.stages.videos = { status: qcReady ? "ready" : "needs_review", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "videos",
        dynastyId,
        message: `${dynasty.name}：朝代视频版本已确认`,
    }));
    return next;
}

export function failWeddingVideoGeneration(
    state: WeddingProductionState,
    job: WeddingVideoJob,
    reason: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;
    dynasty.statuses.video = "failed";
    dynasty.qc.video = pendingVideoQc("生成失败，等待重试", now);
    next.stages.videos = { status: "needs_review", updatedAt: now };
    next.stages.delivery = { ...next.stages.delivery, status: "stale" };
    next.activity.push(activity(now, next.activity.length, {
        type: "error",
        stage: "videos",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：朝代视频生成失败 · ${reason.trim() || "未知错误"}`,
    }));
    return next;
}

function pendingVideoQc(reason: string, now: string) {
    return {
        verdict: "pending" as const,
        faceStable: false,
        outfitStable: false,
        actionReadable: false,
        extraPerson: false,
        watermarkOrText: false,
        reason,
        checkedAt: now,
    };
}

function activity(
    now: string,
    index: number,
    event: Omit<WeddingActivityEvent, "id" | "createdAt">,
): WeddingActivityEvent {
    return {
        ...event,
        id: `wedding-video-${Date.parse(now)}-${index}`,
        createdAt: now,
    };
}
