import { expect, test } from "bun:test";

import {
    markWeddingPackagingReady,
    updateWeddingPackaging,
} from "@/lib/wedding/packaging-runtime";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const production = createWeddingProductionState({
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
        portraitMaleNodeId: "male",
        portraitFemaleNodeId: "female",
        sceneNodeId: "scene",
        videoNodeId: "video",
    }],
    now: "2026-10-25T00:00:00.000Z",
});

test("editing packaging makes only final delivery stale", () => {
    const changed = updateWeddingPackaging(production, {
        intro: { text: "三生石上", durationSeconds: 4, enabled: true },
        subtitlesEnabled: false,
    }, "2026-10-25T08:00:00.000Z");

    expect(changed.packaging.intro.text).toBe("三生石上");
    expect(changed.packaging.subtitlesEnabled).toBe(false);
    expect(changed.packaging.status).toBe("ready");
    expect(changed.stages.delivery.status).toBe("stale");
    expect(changed.dynasties[0]?.statuses.video).toBe(production.dynasties[0]?.statuses.video);
});

test("marking a timeline ready persists its revision and clears packaging approval", () => {
    const changed = markWeddingPackagingReady(
        {
            ...production,
            packaging: {
                ...production.packaging,
                status: "approved",
                approvedAt: "2026-10-25T07:00:00.000Z",
            },
        },
        3,
        "2026-10-25T08:10:00.000Z",
    );

    expect(changed.packaging.status).toBe("ready");
    expect(changed.packaging.timelineRevision).toBe(3);
    expect(changed.packaging.previewReadyAt).toBe("2026-10-25T08:10:00.000Z");
    expect(changed.packaging.approvedAt).toBeUndefined();
    expect(changed.stages.delivery.status).toBe("stale");
});
