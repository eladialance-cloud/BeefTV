import { approveWeddingStage } from "@/lib/wedding/production-state";
import type {
    WeddingDeliveryResult,
    WeddingDeliveryState,
    WeddingProductionState,
} from "@/lib/wedding/types";

export function markWeddingDeliveryRendering(
    state: WeddingProductionState,
    taskId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    next.delivery = {
        ...(next.delivery || { status: "not_started" }),
        status: "rendering",
        taskId: taskId || next.delivery?.taskId,
        error: undefined,
    };
    next.stages.delivery = { status: "generating", updatedAt: now };
    return next;
}

export function recordWeddingDeliveryResult(
    state: WeddingProductionState,
    result: WeddingDeliveryResult,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    next.delivery = {
        ...(next.delivery || { status: "not_started" }),
        status: "needs_review",
        taskId: next.delivery?.taskId,
        resourceId: result.resourceId,
        fileName: result.fileName,
        durationMs: result.durationMs,
        size: result.size,
        error: undefined,
    };
    next.stages.delivery = { status: "needs_review", updatedAt: now };
    return next;
}

export function failWeddingDeliveryRendering(
    state: WeddingProductionState,
    reason: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    next.delivery = {
        ...(next.delivery || { status: "not_started" }),
        status: "failed",
        error: reason.trim() || "成片渲染失败",
    };
    next.stages.delivery = { status: "failed", updatedAt: now } as WeddingProductionState["stages"]["delivery"];
    return next;
}

export function verifyWeddingDeliveryPlayable(
    state: WeddingProductionState,
    metadata: { durationMs: number; videoWidth: number; videoHeight: number },
    now: string,
): WeddingProductionState {
    if (!state.delivery?.resourceId
        || !Number.isFinite(metadata.durationMs)
        || metadata.durationMs <= 0
        || metadata.videoWidth <= 0
        || metadata.videoHeight <= 0) return state;
    const next = structuredClone(state);
    const delivery = next.delivery;
    if (!delivery) return state;
    next.delivery = {
        ...delivery,
        status: delivery.status,
        playableVerifiedAt: now,
        qc: {
            durationMs: metadata.durationMs,
            width: metadata.videoWidth,
            height: metadata.videoHeight,
            checkedAt: now,
        },
    };
    return next;
}

export function confirmWeddingDelivery(
    state: WeddingProductionState,
    now: string,
): WeddingProductionState {
    if (!state.delivery?.resourceId || !state.delivery.playableVerifiedAt) return state;
    const approved = approveWeddingStage(state, "delivery", now);
    if (approved === state) return state;
    const next = structuredClone(approved);
    next.delivery = {
        ...next.delivery!,
        status: "approved",
        approvedAt: now,
    };
    return next;
}

export function deliveryStateOrDefault(state?: WeddingDeliveryState): WeddingDeliveryState {
    return state || { status: "not_started" };
}
