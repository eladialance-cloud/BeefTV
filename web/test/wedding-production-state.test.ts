import { describe, expect, test } from "bun:test";

import {
    approveWeddingStage,
    createWeddingProductionState,
    invalidateWeddingDynasty,
    nextWeddingStage,
    weddingStageBlockers,
} from "@/lib/wedding/production-state";

const input = {
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
    source: {
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "tang-male",
        portraitFemaleNodeId: "tang-female",
        sceneNodeId: "tang-scene",
        videoNodeId: "tang-video",
    }],
    now: "2026-10-25T00:00:00.000Z",
};

describe("wedding production state", () => {
    test("maps every production stage to the next stage", () => {
        expect(nextWeddingStage("setup")).toBe("dynasties");
        expect(nextWeddingStage("dynasties")).toBe("portraits");
        expect(nextWeddingStage("portraits")).toBe("scenes");
        expect(nextWeddingStage("scenes")).toBe("videos");
        expect(nextWeddingStage("videos")).toBe("packaging");
        expect(nextWeddingStage("packaging")).toBe("delivery");
        expect(nextWeddingStage("delivery")).toBeUndefined();
    });

    test("creates seven stages and ordered dynasties", () => {
        const state = createWeddingProductionState(input);

        expect(Object.keys(state.stages)).toEqual([
            "setup",
            "dynasties",
            "portraits",
            "scenes",
            "videos",
            "packaging",
            "delivery",
        ]);
        expect(state.stages.setup.status).toBe("ready");
        expect(state.stages.dynasties.status).toBe("ready");
        expect(state.stages.portraits.status).toBe("ready");
        expect(state.stages.scenes.status).toBe("ready");
        expect(state.stages.videos.status).toBe("ready");
        expect(state.stages.packaging.status).toBe("not_started");
        expect(state.dynasties[0]).toMatchObject({
            id: "tang",
            name: "唐",
            order: 0,
            statuses: { portrait: "ready", scene: "ready", video: "ready" },
        });
    });

    test("prompt invalidation marks video and final stages stale", () => {
        const state = createWeddingProductionState(input);
        const changed = invalidateWeddingDynasty(state, "tang", "prompt");

        expect(changed).not.toBe(state);
        expect(changed.dynasties[0].statuses.video).toBe("stale");
        expect(changed.stages.videos.status).toBe("stale");
        expect(changed.stages.delivery.status).toBe("stale");
        expect(changed.activity.at(-1)).toMatchObject({
            stage: "videos",
            dynastyId: "tang",
            type: "asset_staled",
        });
    });

    test("portrait invalidation marks scene, video and final stages stale", () => {
        const state = createWeddingProductionState(input);
        const changed = invalidateWeddingDynasty(state, "tang", "portrait");

        expect(changed.dynasties[0].statuses.scene).toBe("stale");
        expect(changed.dynasties[0].statuses.video).toBe("stale");
        expect(changed.stages.scenes.status).toBe("stale");
        expect(changed.stages.videos.status).toBe("stale");
        expect(changed.stages.delivery.status).toBe("stale");
    });

    test("cannot approve delivery while video is unconfirmed", () => {
        const state = createWeddingProductionState(input);
        const ready = {
            ...state,
            dynasties: state.dynasties.map((dynasty) => ({
                ...dynasty,
                statuses: { ...dynasty.statuses, video: "needs_review" as const },
            })),
        };

        expect(weddingStageBlockers(ready, "delivery")).toContain("唐：视频尚未确认");
        expect(approveWeddingStage(ready, "delivery", "2026-10-25T01:00:00.000Z")).toBe(ready);
    });

    test("can approve a stage when all blockers are cleared", () => {
        const state = createWeddingProductionState(input);
        const reviewed = {
            ...state,
            dynasties: state.dynasties.map((dynasty) => ({
                ...dynasty,
                portraitSelection: { male: "male-asset", female: "female-asset" },
                portraitApproval: { male: true, female: true },
            })),
        };
        const approved = approveWeddingStage(reviewed, "portraits", "2026-10-25T01:00:00.000Z");

        expect(approved.stages.portraits).toMatchObject({
            status: "approved",
            approvedAt: "2026-10-25T01:00:00.000Z",
        });
        expect(approved.activity.at(-1)).toMatchObject({
            type: "asset_approved",
            stage: "portraits",
        });
    });
});
