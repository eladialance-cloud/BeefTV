import { expect, test } from "bun:test";

import { addWeddingDynastyState, setWeddingDynasties } from "@/lib/wedding/dynasty-state";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const createState = () => createWeddingProductionState({
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
    dynasties: [
        { id: "tang", name: "唐", order: 0, portraitMaleNodeId: "male-tang", portraitFemaleNodeId: "female-tang", sceneNodeId: "scene-tang", videoNodeId: "video-tang" },
        { id: "ming", name: "明", order: 1, portraitMaleNodeId: "male-ming", portraitFemaleNodeId: "female-ming", sceneNodeId: "scene-ming", videoNodeId: "video-ming" },
    ],
    now: "2026-10-25T00:00:00.000Z",
});

test("reorders dynasties and invalidates only final delivery", () => {
    const changed = setWeddingDynasties(createState(), ["ming", "tang"], "2026-10-25T01:00:00.000Z");
    expect(changed.dynasties.map((item) => [item.id, item.order])).toEqual([
        ["ming", 0],
        ["tang", 1],
    ]);
    expect(changed.stages.packaging.status).toBe("not_started");
    expect(changed.stages.delivery.status).toBe("stale");
});

test("removing a dynasty invalidates packaging and delivery", () => {
    const changed = setWeddingDynasties(createState(), ["tang"], "2026-10-25T01:00:00.000Z");
    expect(changed.dynasties).toHaveLength(1);
    expect(changed.stages.packaging.status).toBe("stale");
    expect(changed.stages.delivery.status).toBe("stale");
});

test("rejects empty or duplicate dynasty selections", () => {
    const state = createState();
    expect(setWeddingDynasties(state, [], "2026-10-25T01:00:00.000Z")).toBe(state);
    expect(setWeddingDynasties(state, ["tang", "tang"], "2026-10-25T01:00:00.000Z")).toBe(state);
});

test("adds a dynasty with four required node references", () => {
    const changed = addWeddingDynastyState(createState(), {
        id: "song",
        name: "宋",
        nodeIds: {
            portraitMaleNodeId: "male-song",
            portraitFemaleNodeId: "female-song",
            sceneNodeId: "scene-song",
            videoNodeId: "video-song",
        },
    }, "2026-10-25T01:00:00.000Z");
    expect(changed.dynasties.at(-1)).toMatchObject({
        id: "song",
        name: "宋",
        order: 2,
        portraitMaleNodeId: "male-song",
        videoNodeId: "video-song",
    });
    expect(changed.stages.delivery.status).toBe("stale");
});
