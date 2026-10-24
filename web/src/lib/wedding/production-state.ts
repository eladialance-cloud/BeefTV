import {
    WEDDING_STAGE_IDS,
    type WeddingActivityEvent,
    type WeddingDynastyState,
    type WeddingItemStatus,
    type WeddingProductionState,
    type WeddingStageId,
} from "@/lib/wedding/types";

export type CreateWeddingProductionInput = {
    template: WeddingProductionState["template"];
    source: WeddingProductionState["source"];
    dynasties: Array<Pick<
        WeddingDynastyState,
        "id" | "name" | "order" | "portraitMaleNodeId" | "portraitFemaleNodeId" | "sceneNodeId" | "videoNodeId"
    >>;
    now: string;
};

const READY_ITEM_STATUSES = new Set<WeddingItemStatus>(["ready", "approved"]);

export function createWeddingProductionState(input: CreateWeddingProductionInput): WeddingProductionState {
    const stageTimestamp = input.now;
    const hasDynasties = input.dynasties.length > 0;
    const stages = Object.fromEntries(WEDDING_STAGE_IDS.map((stage) => [
        stage,
        {
            status: stage === "setup"
                || stage === "dynasties"
                || (hasDynasties && (stage === "portraits" || stage === "scenes" || stage === "videos"))
                ? "ready"
                : "not_started",
            updatedAt: stageTimestamp,
        },
    ])) as WeddingProductionState["stages"];
    const dynasties = input.dynasties.map((dynasty, index) => ({
        ...dynasty,
        order: dynasty.order ?? index,
        portraitSelection: {},
        statuses: {
            portrait: "ready",
            scene: "ready",
            video: "ready",
        },
        qc: { overrides: {} },
    })) satisfies WeddingDynastyState[];

    return {
        schemaVersion: 1,
        template: input.template,
        activeStage: "setup",
        stages,
        source: input.source,
        dynasties,
        packaging: {
            status: "not_started",
            intro: { text: "几生几世 跨时空相爱", durationSeconds: 4, enabled: true },
            outro: { text: "今生今世 永不分离", durationSeconds: 5, enabled: true },
            subtitlesEnabled: true,
            transitionSeconds: 1,
            bgmMode: "generated",
            titleCardNodeIds: [],
        },
        activity: [{
            id: activityId(stageTimestamp, 0),
            type: "stage_changed",
            stage: "setup",
            message: "跨时空婚礼电影制作项目已创建",
            createdAt: stageTimestamp,
        }],
    };
}

export function approveWeddingStage(
    state: WeddingProductionState,
    stage: WeddingStageId,
    now: string,
): WeddingProductionState {
    if (weddingStageBlockers(state, stage).length > 0) return state;

    const next = cloneState(state);
    next.stages[stage] = { status: "approved", updatedAt: now, approvedAt: now };
    if (stage === "portraits" || stage === "scenes" || stage === "videos") {
        const item = stage === "portraits" ? "portrait" : stage === "scenes" ? "scene" : "video";
        next.dynasties = next.dynasties.map((dynasty) => ({
            ...dynasty,
            statuses: { ...dynasty.statuses, [item]: "approved" },
        }));
    }
    if (stage === "packaging") {
        next.packaging = { ...next.packaging, status: "approved", approvedAt: now };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_approved",
        stage,
        message: `${stageLabel(stage)}已确认`,
    }));
    return next;
}

export function invalidateWeddingDynasty(
    state: WeddingProductionState,
    dynastyId: string,
    from: "portrait" | "scene" | "video" | "prompt",
): WeddingProductionState {
    const dynasty = state.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;

    const next = cloneState(state);
    const target = next.dynasties.find((item) => item.id === dynastyId)!;
    if (from === "portrait") {
        target.statuses.scene = "stale";
        target.statuses.video = "stale";
        markStagesStale(next, ["scenes", "videos", "packaging", "delivery"]);
    } else if (from === "scene") {
        target.statuses.video = "stale";
        markStagesStale(next, ["videos", "packaging", "delivery"]);
    } else {
        target.statuses.video = "stale";
        markStagesStale(next, ["videos", "delivery"]);
    }
    next.activity.push(activity(
        new Date().toISOString(),
        next.activity.length,
        {
            type: "asset_staled",
            stage: from === "portrait" ? "scenes" : "videos",
            dynastyId,
            message: `${dynasty.name}：${invalidationLabel(from)}已变化，下游结果失效`,
        },
    ));
    return next;
}

export function weddingStageBlockers(state: WeddingProductionState, stage: WeddingStageId): string[] {
    if (stage === "setup") {
        const blockers: string[] = [];
        if (!state.source.groomPhotoNodeId) blockers.push("新郎照片尚未上传");
        if (!state.source.bridePhotoNodeId) blockers.push("新娘照片尚未上传");
        if (!state.source.consentConfirmed) blockers.push("照片授权尚未确认");
        return blockers;
    }
    if (stage === "dynasties") {
        const ids = state.dynasties.map((dynasty) => dynasty.id);
        if (ids.length < 2 || ids.length > 5) return ["必须选择 2 至 5 个朝代"];
        if (new Set(ids).size !== ids.length) return ["朝代选择存在重复项"];
        return [];
    }
    if (stage === "portraits") {
        return state.dynasties.flatMap((dynasty) => READY_ITEM_STATUSES.has(dynasty.statuses.portrait)
            ? []
            : [`${dynasty.name}：定妆照尚未准备`]);
    }
    if (stage === "scenes") {
        return state.dynasties.flatMap((dynasty) => READY_ITEM_STATUSES.has(dynasty.statuses.scene)
            ? []
            : [`${dynasty.name}：场景首帧尚未准备`]);
    }
    if (stage === "videos") {
        return state.dynasties.flatMap((dynasty) => READY_ITEM_STATUSES.has(dynasty.statuses.video)
            ? []
            : [`${dynasty.name}：视频尚未准备`]);
    }
    if (stage === "packaging") {
        return state.packaging.status === "ready" || state.packaging.status === "approved"
            ? []
            : ["包装素材尚未准备"];
    }

    const blockers: string[] = [];
    for (const dynasty of state.dynasties) {
        if (dynasty.statuses.portrait !== "approved") blockers.push(`${dynasty.name}：定妆照尚未确认`);
        if (dynasty.statuses.scene !== "approved") blockers.push(`${dynasty.name}：场景首帧尚未确认`);
        if (dynasty.statuses.video !== "approved") blockers.push(`${dynasty.name}：视频尚未确认`);
    }
    if (state.packaging.status !== "approved") blockers.push("包装制作尚未确认");
    return blockers;
}

function markStagesStale(state: WeddingProductionState, stages: WeddingStageId[]) {
    for (const stage of stages) {
        state.stages[stage] = { ...state.stages[stage], status: "stale" };
    }
}

function cloneState(state: WeddingProductionState): WeddingProductionState {
    return {
        ...state,
        template: { ...state.template },
        stages: Object.fromEntries(WEDDING_STAGE_IDS.map((stage) => [
            stage,
            { ...state.stages[stage] },
        ])) as WeddingProductionState["stages"],
        source: { ...state.source },
        dynasties: state.dynasties.map((dynasty) => ({
            ...dynasty,
            portraitSelection: { ...dynasty.portraitSelection },
            statuses: { ...dynasty.statuses },
            qc: {
                ...dynasty.qc,
                ...(dynasty.qc.scene ? { scene: { ...dynasty.qc.scene } } : {}),
                ...(dynasty.qc.video ? { video: { ...dynasty.qc.video } } : {}),
                overrides: { ...dynasty.qc.overrides },
            },
        })),
        packaging: {
            ...state.packaging,
            intro: { ...state.packaging.intro },
            outro: { ...state.packaging.outro },
            titleCardNodeIds: [...state.packaging.titleCardNodeIds],
        },
        activity: state.activity.map((event) => ({ ...event })),
    };
}

function activity(
    now: string,
    index: number,
    event: Omit<WeddingActivityEvent, "id" | "createdAt">,
): WeddingActivityEvent {
    return { ...event, id: activityId(now, index), createdAt: now };
}

function activityId(now: string, index: number) {
    return `wedding-activity-${Date.parse(now)}-${index}`;
}

function stageLabel(stage: WeddingStageId) {
    const labels: Record<WeddingStageId, string> = {
        setup: "项目设定",
        dynasties: "朝代规划",
        portraits: "定妆照",
        scenes: "场景首帧",
        videos: "视频导演",
        packaging: "包装制作",
        delivery: "成片交付",
    };
    return labels[stage];
}

function invalidationLabel(from: "portrait" | "scene" | "video" | "prompt") {
    if (from === "portrait") return "定妆照";
    if (from === "scene") return "场景首帧";
    if (from === "prompt") return "视频提示词";
    return "视频版本";
}
