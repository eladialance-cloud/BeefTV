import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type { WeddingProductionState, WeddingStageId } from "@/lib/wedding/types";

type WeddingSource = WeddingProductionState["source"];
type WeddingSourcePatch = Partial<WeddingSource>;

export function updateWeddingSource(
    state: WeddingProductionState,
    patch: WeddingSourcePatch,
    now: string,
): WeddingProductionState {
    const changedFields = Object.entries(patch).filter(([key, value]) => state.source[key as keyof WeddingSource] !== value);
    if (!changedFields.length) return state;

    const next = structuredClone(state);
    next.source = { ...next.source, ...patch };
    const photoChanged = changedFields.some(([key]) => key === "groomPhotoNodeId" || key === "bridePhotoNodeId");
    const outputChanged = changedFields.some(([key]) => key === "story" || key === "ratio" || key === "resolution");

    if (photoChanged) {
        for (const dynasty of next.dynasties) {
            dynasty.statuses.portrait = "stale";
            dynasty.statuses.scene = "stale";
            dynasty.statuses.video = "stale";
        }
        markStagesStale(next, ["portraits", "scenes", "videos", "packaging", "delivery"]);
    } else if (outputChanged) {
        for (const dynasty of next.dynasties) {
            dynasty.statuses.scene = "stale";
            dynasty.statuses.video = "stale";
        }
        markStagesStale(next, ["scenes", "videos", "packaging", "delivery"]);
    }

    next.activity.push({
        id: `wedding-setup-${Date.parse(now)}-${next.activity.length}`,
        type: "stage_changed",
        stage: photoChanged ? "portraits" : outputChanged ? "scenes" : "setup",
        message: `项目设定已更新：${changedFields.map(([key]) => setupFieldLabel(key)).join("、")}`,
        createdAt: now,
    });
    return next;
}

export function approveWeddingSetup(
    state: WeddingProductionState,
    now: string,
): WeddingProductionState {
    if (weddingStageBlockers(state, "setup").length > 0) return state;

    const next = structuredClone(state);
    next.stages.setup = { status: "approved", updatedAt: now, approvedAt: now };
    next.activity.push({
        id: `wedding-setup-approved-${Date.parse(now)}-${next.activity.length}`,
        type: "asset_approved",
        stage: "setup",
        message: "项目设定已确认",
        createdAt: now,
    });
    return next;
}

function markStagesStale(state: WeddingProductionState, stages: WeddingStageId[]) {
    for (const stage of stages) {
        state.stages[stage] = { ...state.stages[stage], status: "stale" };
    }
}

function setupFieldLabel(key: string) {
    const labels: Record<string, string> = {
        groomPhotoNodeId: "新郎照片",
        bridePhotoNodeId: "新娘照片",
        maleName: "新郎称呼",
        femaleName: "新娘称呼",
        story: "爱情故事",
        consentConfirmed: "照片授权",
        ratio: "画幅",
        resolution: "分辨率",
    };
    return labels[key] || key;
}
