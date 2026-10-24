import { expect, test } from "bun:test";

import {
    clearWeddingPromptOverride,
    setWeddingPromptOverride,
    weddingVideoPrompt,
} from "@/lib/wedding/prompt";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const state = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
    source: {
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        maleName: "林屿",
        femaleName: "周棠",
        story: "",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "male",
        portraitFemaleNodeId: "female",
        sceneNodeId: "scene",
        videoNodeId: "video",
    }],
    now: "2026-10-25T00:00:00.000Z",
});

test("override wins over the template prompt and invalidates video", () => {
    const changed = setWeddingPromptOverride(
        state,
        "tang",
        "edited prompt",
        "2026-10-25T01:00:00.000Z",
    );

    expect(weddingVideoPrompt(changed, "tang", "default prompt")).toBe("edited prompt");
    expect(changed.dynasties[0]?.promptOverride).toBe("edited prompt");
    expect(changed.dynasties[0]?.promptOverrideUpdatedAt).toBe("2026-10-25T01:00:00.000Z");
    expect(changed.dynasties[0]?.statuses.video).toBe("stale");
    expect(changed.stages.videos.status).toBe("stale");
});

test("clearing override returns to the default prompt", () => {
    const edited = setWeddingPromptOverride(
        state,
        "tang",
        "edited",
        "2026-10-25T01:00:00.000Z",
    );
    const cleared = clearWeddingPromptOverride(
        edited,
        "tang",
        "2026-10-25T02:00:00.000Z",
    );

    expect(weddingVideoPrompt(cleared, "tang", "default prompt")).toBe("default prompt");
    expect(cleared.dynasties[0]?.promptOverride).toBeUndefined();
    expect(cleared.dynasties[0]?.promptOverrideUpdatedAt).toBeUndefined();
    expect(cleared.dynasties[0]?.statuses.video).toBe("stale");
});

test("unknown dynasty leaves the state unchanged", () => {
    expect(setWeddingPromptOverride(state, "missing", "edited", "2026-10-25T01:00:00.000Z")).toBe(state);
    expect(clearWeddingPromptOverride(state, "missing", "2026-10-25T01:00:00.000Z")).toBe(state);
});
