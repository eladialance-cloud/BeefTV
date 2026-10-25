import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { approveWeddingSetup, updateWeddingSource } from "@/lib/wedding/setup-state";

const base = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
    source: {
        groomPhotoNodeId: "groom-old",
        bridePhotoNodeId: "bride-old",
        maleName: "林屿",
        femaleName: "周棠",
        story: "旧故事",
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

test("changing a source photo invalidates every downstream asset", () => {
    const changed = updateWeddingSource(
        base,
        { groomPhotoNodeId: "groom-new" },
        "2026-10-25T01:00:00.000Z",
    );

    expect(changed.dynasties[0]?.statuses).toEqual({
        portrait: "stale",
        scene: "stale",
        video: "stale",
    });
    expect(changed.stages.portraits.status).toBe("stale");
    expect(changed.stages.scenes.status).toBe("stale");
    expect(changed.stages.delivery.status).toBe("stale");
});

test("changing output settings preserves portraits but invalidates scenes and videos", () => {
    const changed = updateWeddingSource(
        base,
        { ratio: "9:16" },
        "2026-10-25T01:00:00.000Z",
    );

    expect(changed.dynasties[0]?.statuses.portrait).toBe("ready");
    expect(changed.dynasties[0]?.statuses.scene).toBe("stale");
    expect(changed.dynasties[0]?.statuses.video).toBe("stale");
});

test("unchanged source values keep the original state reference", () => {
    expect(updateWeddingSource(
        base,
        { maleName: "林屿", ratio: "16:9" },
        "2026-10-25T01:00:00.000Z",
    )).toBe(base);
});

test("setup cannot be approved without photos and consent", () => {
    const incomplete = updateWeddingSource(
        base,
        { groomPhotoNodeId: "", consentConfirmed: false },
        "2026-10-25T01:00:00.000Z",
    );
    expect(approveWeddingSetup(incomplete, "2026-10-25T02:00:00.000Z")).toBe(incomplete);
    expect(approveWeddingSetup(base, "2026-10-25T02:00:00.000Z").stages.setup.status).toBe("approved");
});
