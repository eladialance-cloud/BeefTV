import type {
    WeddingActivityEvent,
    WeddingPortraitGender,
    WeddingPortraitVersion,
    WeddingProductionState,
} from "@/lib/wedding/types";

export type WeddingGenerationJob = {
    nodeId: string;
    dynastyId: string;
    kind: "portrait";
    gender: WeddingPortraitGender;
    promptSourceNodeId: string;
};

export type WeddingPortraitVersionInput = Omit<WeddingPortraitVersion, "nodeId">;

export function weddingPortraitJobs(
    production: WeddingProductionState,
    dynastyId?: string,
): WeddingGenerationJob[] {
    return production.dynasties
        .filter((dynasty) => !dynastyId || dynasty.id === dynastyId)
        .sort((left, right) => left.order - right.order)
        .flatMap((dynasty) => [
            {
                nodeId: dynasty.portraitMaleNodeId,
                dynastyId: dynasty.id,
                kind: "portrait" as const,
                gender: "male" as const,
                promptSourceNodeId: production.source.groomPhotoNodeId,
            },
            {
                nodeId: dynasty.portraitFemaleNodeId,
                dynastyId: dynasty.id,
                kind: "portrait" as const,
                gender: "female" as const,
                promptSourceNodeId: production.source.bridePhotoNodeId,
            },
        ]);
}

export function markWeddingPortraitGenerating(
    state: WeddingProductionState,
    job: WeddingGenerationJob,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty || dynasty.statuses.portrait === "generating") return state;

    dynasty.statuses.portrait = "generating";
    dynasty.portraitApproval = {
        ...(dynasty.portraitApproval || {}),
        [job.gender]: false,
    };
    next.stages.portraits = { status: "generating", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "stage_changed",
        stage: "portraits",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：${genderLabel(job.gender)}定妆照开始生成`,
    }));
    return next;
}

export function recordWeddingPortraitVersion(
    state: WeddingProductionState,
    job: WeddingGenerationJob,
    version: WeddingPortraitVersionInput,
    now = new Date().toISOString(),
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;

    const history = dynasty.portraitVersionHistory || {};
    const versions = history[job.gender] || [];
    history[job.gender] = [
        {
            ...version,
            nodeId: job.nodeId,
        },
        ...versions.filter((item) => item.assetId !== version.assetId),
    ].slice(0, 12);
    dynasty.portraitVersionHistory = history;
    dynasty.portraitSelection = { ...dynasty.portraitSelection, [job.gender]: version.assetId };
    dynasty.portraitApproval = {
        ...(dynasty.portraitApproval || {}),
        [job.gender]: false,
    };
    dynasty.statuses.portrait = "needs_review";
    dynasty.statuses.scene = "stale";
    dynasty.statuses.video = "stale";
    next.stages.portraits = { status: "needs_review", updatedAt: now };
    for (const stage of ["scenes", "videos", "packaging", "delivery"] as const) {
        next.stages[stage] = { ...next.stages[stage], status: "stale" };
    }
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_generated",
        stage: "portraits",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：${genderLabel(job.gender)}定妆照已生成，等待确认`,
    }));
    return next;
}

export function confirmWeddingPortrait(
    state: WeddingProductionState,
    dynastyId: string,
    gender: WeddingPortraitGender,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;

    dynasty.portraitSelection = { ...dynasty.portraitSelection, [gender]: assetId };
    dynasty.portraitApproval = {
        ...(dynasty.portraitApproval || {}),
        [gender]: true,
    };
    if (dynasty.portraitApproval.male && dynasty.portraitApproval.female) {
        dynasty.statuses.portrait = "ready";
    }
    next.stages.portraits = {
        status: next.dynasties.every((item) => item.portraitApproval?.male && item.portraitApproval?.female)
            ? "ready"
            : "needs_review",
        updatedAt: now,
    };
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "portraits",
        dynastyId,
        message: `${dynasty.name}：${genderLabel(gender)}定妆照已确认`,
    }));
    return next;
}

export function selectWeddingPortraitVersion(
    state: WeddingProductionState,
    dynastyId: string,
    gender: WeddingPortraitGender,
    assetId: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty || !assetId.trim()) return state;

    dynasty.portraitSelection = { ...dynasty.portraitSelection, [gender]: assetId };
    dynasty.portraitApproval = {
        ...(dynasty.portraitApproval || {}),
        [gender]: false,
    };
    dynasty.statuses.portrait = "needs_review";
    next.stages.portraits = { status: "needs_review", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "asset_selected",
        stage: "portraits",
        dynastyId,
        message: `${dynasty.name}：已切换${genderLabel(gender)}定妆照版本，等待确认`,
    }));
    return next;
}

export function failWeddingPortraitGeneration(
    state: WeddingProductionState,
    job: WeddingGenerationJob,
    reason: string,
    now: string,
): WeddingProductionState {
    const next = structuredClone(state);
    const dynasty = next.dynasties.find((item) => item.id === job.dynastyId);
    if (!dynasty) return state;

    dynasty.statuses.portrait = "failed";
    dynasty.portraitApproval = {
        ...(dynasty.portraitApproval || {}),
        [job.gender]: false,
    };
    next.stages.portraits = { status: "needs_review", updatedAt: now };
    next.activity.push(activity(now, next.activity.length, {
        type: "error",
        stage: "portraits",
        dynastyId: dynasty.id,
        message: `${dynasty.name}：${genderLabel(job.gender)}定妆照生成失败 · ${reason.trim() || "未知错误"}`,
    }));
    return next;
}

function genderLabel(gender: WeddingPortraitGender) {
    return gender === "male" ? "新郎" : "新娘";
}

function activity(
    now: string,
    index: number,
    event: Omit<WeddingActivityEvent, "id" | "createdAt">,
): WeddingActivityEvent {
    return {
        ...event,
        id: `wedding-portrait-${Date.parse(now)}-${index}`,
        createdAt: now,
    };
}
