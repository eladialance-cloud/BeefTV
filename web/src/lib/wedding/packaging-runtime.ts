import type {
    WeddingPackagingState,
    WeddingProductionState,
} from "@/lib/wedding/types";

export type WeddingPackagingPatch = Partial<Omit<WeddingPackagingState, "intro" | "outro">> & {
    intro?: Partial<WeddingPackagingState["intro"]>;
    outro?: Partial<WeddingPackagingState["outro"]>;
};

export function updateWeddingPackaging(
    state: WeddingProductionState,
    patch: WeddingPackagingPatch,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    next.packaging = {
        ...next.packaging,
        ...patch,
        intro: { ...next.packaging.intro, ...(patch.intro || {}) },
        outro: { ...next.packaging.outro, ...(patch.outro || {}) },
        status: "ready",
        approvedAt: undefined,
    };
    next.stages.packaging = { status: "ready", updatedAt: now };
    next.stages.delivery = { ...next.stages.delivery, status: "stale" };
    return next;
}

export function markWeddingPackagingReady(
    state: WeddingProductionState,
    timelineRevision: number,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    next.packaging = {
        ...next.packaging,
        status: "ready",
        timelineRevision,
        previewReadyAt: now,
        approvedAt: undefined,
    };
    next.stages.packaging = { status: "ready", updatedAt: now };
    next.stages.delivery = { ...next.stages.delivery, status: "stale" };
    return next;
}
