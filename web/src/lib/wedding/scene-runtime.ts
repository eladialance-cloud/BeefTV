import type {
    WeddingActivityEvent,
    WeddingProductionState,
    WeddingSceneVersion,
} from "@/lib/wedding/types";

export type WeddingSceneJob = {
    nodeId: string;
    dynastyId: string;
    kind: "scene";
};

export type WeddingSceneVersionInput = Omit<WeddingSceneVersion, "nodeId">;

export function weddingSceneJobs(
    production: WeddingProductionState,
    dynastyId?: string,
): WeddingSceneJob[] {
    return production.dynasties
        .filter((dynasty) => !dynastyId || dynasty.id === dynastyId)
        .sort((left, right) => left.order - right.order)
        .map((dynasty) => ({
            nodeId: dynasty.sceneNodeId,
            dynastyId: dynasty.id,
            kind: "scene" as const,
        }));
}

export function markWeddingSceneGenerating(
    state: WeddingProductionState,
    job: WeddingSceneJob,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty || dynasty.statuses.scene === "generating") return state;

    dynasty.statuses.scene = "generating";
    next.stages.scenes = { status: "generating", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "stage_changed",
        stage: "scenes",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：场景首帧开始生成`,
    }));
    return next;
}

export function recordWeddingSceneVersion(
    state: WeddingProductionState,
    job: WeddingSceneJob,
    version: WeddingSceneVersionInput,
    now = new Date().toISOString(),
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;

    dynasty.sceneVersions = [
        { ...version, nodeId: job.nodeId },
        ...(dynasty.sceneVersions || []).filter((item) => item.assetId !== version.assetId),
    ].slice(0, 12);
    dynasty.sceneSelection = version.assetId;
    dynasty.qc.scene = undefined;
    dynasty.statuses.scene = "needs_review";
    dynasty.statuses.video = "stale";
    next.stages.scenes = { status: "needs_review", updatedAt: now };
    for (const stage of ["videos", "packaging", "delivery"] as const) {
        next.stages[stage] = { ...next.stages[stage], status: "stale" };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_generated",
        stage: "scenes",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：场景首帧已生成，等待 QC 与确认`,
    }));
    return next;
}

export function selectWeddingSceneVersion(
    state: WeddingProductionState,
    dynastyId: string,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;

    dynasty.sceneSelection = assetId;
    dynasty.qc.scene = undefined;
    dynasty.statuses.scene = "needs_review";
    dynasty.statuses.video = "stale";
    next.stages.scenes = { status: "needs_review", updatedAt: now };
    for (const stage of ["videos", "packaging", "delivery"] as const) {
        next.stages[stage] = { ...next.stages[stage], status: "stale" };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "scenes",
        dynastyId,
        message: `${dynasty.name}：已切换场景首帧版本，需要重新 QC`,
    }));
    return next;
}

export function confirmWeddingScene(
    state: WeddingProductionState,
    dynastyId: string,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;

    dynasty.sceneSelection = assetId;
    const qcReady = dynasty.qc.scene?.verdict === "pass"
        || Boolean(dynasty.qc.overrides.scene?.reason.trim());
    dynasty.statuses.scene = qcReady ? "ready" : "needs_review";
    next.stages.scenes = { status: qcReady ? "ready" : "needs_review", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "scenes",
        dynastyId,
        message: `${dynasty.name}：场景首帧版本已确认`,
    }));
    return next;
}

export function failWeddingSceneGeneration(
    state: WeddingProductionState,
    job: WeddingSceneJob,
    reason: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;

    dynasty.statuses.scene = "failed";
    next.stages.scenes = { status: "needs_review", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "error",
        stage: "scenes",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：场景首帧生成失败 · ${reason.trim() || "未知错误"}`,
    }));
    return next;
}

function activity(
    now: string,
    index: number,
    event: Omit<WeddingActivityEvent, "id" | "createdAt">,
): WeddingActivityEvent {
    return {
        ...event,
        id: `wedding-scene-${Date.parse(now)}-${index}`,
        createdAt: now,
    };
}
