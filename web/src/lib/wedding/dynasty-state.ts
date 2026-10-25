import type { WeddingDynastyState, WeddingProductionState } from "@/lib/wedding/types";

export function setWeddingDynasties(
    state: WeddingProductionState,
    orderedIds: string[],
    now: string,
): WeddingProductionState {
    if (orderedIds.length < 1 || orderedIds.length > 5 || new Set(orderedIds).size !== orderedIds.length) return state;
    const byId = new Map(state.dynasties.map((dynasty) => [dynasty.id, dynasty]));
    if (orderedIds.some((id) => !byId.has(id))) return state;
    const previousOrder = state.dynasties.map((dynasty) => dynasty.id).join(",");
    const nextOrder = orderedIds.join(",");
    if (previousOrder === nextOrder) return state;

    const next = structuredClone(state);
    next.dynasties = orderedIds.map((id, order) => ({ ...byId.get(id)!, order }));
    const removed = state.dynasties.some((dynasty) => !orderedIds.includes(dynasty.id));
    if (removed) {
        markStagesStale(next, ["packaging", "delivery"]);
    } else {
        markStagesStale(next, ["delivery"]);
    }
    next.activity.push({
        id: `wedding-dynasties-${Date.parse(now)}-${next.activity.length}`,
        type: "stage_changed",
        stage: "dynasties",
        message: removed ? "朝代列表已调整并移除朝代" : "朝代播放顺序已更新",
        createdAt: now,
    });
    return next;
}

export function addWeddingDynastyState(
    state: WeddingProductionState,
    input: {
        id: string;
        name: string;
        nodeIds: Pick<
            WeddingDynastyState,
            "portraitMaleNodeId" | "portraitFemaleNodeId" | "sceneNodeId" | "videoNodeId"
        >;
    },
    now: string,
): WeddingProductionState {
    if (state.dynasties.length >= 5 || state.dynasties.some((dynasty) => dynasty.id === input.id)) return state;

    const next = structuredClone(state);
    next.dynasties.push({
        id: input.id,
        name: input.name,
        order: next.dynasties.length,
        ...input.nodeIds,
        portraitSelection: {},
        statuses: { portrait: "ready", scene: "ready", video: "ready" },
        qc: { overrides: {} },
    });
    next.stages.portraits.status = "ready";
    next.stages.scenes.status = "ready";
    next.stages.videos.status = "ready";
    markStagesStale(next, ["packaging", "delivery"]);
    next.activity.push({
        id: `wedding-dynasty-add-${Date.parse(now)}-${next.activity.length}`,
        type: "stage_changed",
        stage: "dynasties",
        dynastyId: input.id,
        message: `已新增朝代：${input.name}`,
        createdAt: now,
    });
    return next;
}

function markStagesStale(state: WeddingProductionState, stages: Array<keyof WeddingProductionState["stages"]>) {
    for (const stage of stages) {
        state.stages[stage] = { ...state.stages[stage], status: "stale" };
    }
}
