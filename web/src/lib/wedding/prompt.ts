import { invalidateWeddingDynasty } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";

export function weddingVideoPrompt(
    state: WeddingProductionState,
    dynastyId: string,
    defaultPrompt: string,
) {
    const override = state.dynasties.find((dynasty) => dynasty.id === dynastyId)?.promptOverride?.trim();
    return override || defaultPrompt;
}

export function setWeddingPromptOverride(
    state: WeddingProductionState,
    dynastyId: string,
    prompt: string,
    now: string,
): WeddingProductionState {
    const dynasty = state.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;
    const value = prompt.trim();
    if (!value) return clearWeddingPromptOverride(state, dynastyId, now);

    const invalidated = invalidateWeddingDynasty(state, dynastyId, "prompt");
    return {
        ...invalidated,
        dynasties: invalidated.dynasties.map((item) => item.id === dynastyId
            ? { ...item, promptOverride: value, promptOverrideUpdatedAt: now }
            : item),
        activity: [...invalidated.activity, {
            id: `wedding-prompt-${Date.parse(now)}-${invalidated.activity.length}`,
            type: "prompt_changed",
            stage: "videos",
            dynastyId,
            message: `${dynasty.name}：视频提示词已更新`,
            createdAt: now,
        }],
    };
}

export function clearWeddingPromptOverride(
    state: WeddingProductionState,
    dynastyId: string,
    now: string,
): WeddingProductionState {
    const dynasty = state.dynasties.find((item) => item.id === dynastyId);
    if (!dynasty) return state;
    const invalidated = invalidateWeddingDynasty(state, dynastyId, "prompt");
    return {
        ...invalidated,
        dynasties: invalidated.dynasties.map((item) => {
            if (item.id !== dynastyId) return item;
            const { promptOverride: _promptOverride, promptOverrideUpdatedAt: _updatedAt, ...rest } = item;
            return rest;
        }),
        activity: [...invalidated.activity, {
            id: `wedding-prompt-reset-${Date.parse(now)}-${invalidated.activity.length}`,
            type: "prompt_changed",
            stage: "videos",
            dynastyId,
            message: `${dynasty.name}：已恢复朝代默认提示词`,
            createdAt: now,
        }],
    };
}
