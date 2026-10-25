import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import {
    approveWeddingSetup,
    reconcileWeddingSourcePhotoReadiness,
    replaceWeddingSourcePhoto,
    updateWeddingSource,
} from "@/lib/wedding/setup-state";

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

test("changing duration invalidates videos without regenerating scenes", () => {
    const changed = updateWeddingSource(
        base,
        { durationSeconds: 12 },
        "2026-10-25T01:00:00.000Z",
    );

    expect(changed.source.durationSeconds).toBe(12);
    expect(changed.dynasties[0]?.statuses.scene).toBe(base.dynasties[0]?.statuses.scene);
    expect(changed.dynasties[0]?.statuses.video).toBe("stale");
    expect(changed.stages.videos.status).toBe("stale");
    expect(changed.stages.packaging.status).toBe("stale");
    expect(changed.stages.delivery.status).toBe("stale");
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

test("registered source node ids without uploaded photo content remain blocked", () => {
    const noContent = {
        ...base,
        source: {
            ...base.source,
            groomPhotoReady: false,
            bridePhotoReady: false,
        },
    };

    expect(approveWeddingSetup(noContent, "2026-10-25T02:00:00.000Z")).toBe(noContent);
});

test("replacing a source photo invalidates existing portrait versions even when the node id is unchanged", () => {
    const withPortrait = {
        ...base,
        dynasties: base.dynasties.map((dynasty) => ({
            ...dynasty,
            portraitSelection: { male: "male-asset", female: "female-asset" },
            portraitApproval: { male: true, female: true },
            portraitVersionHistory: {
                male: [{ assetId: "male-asset", nodeId: dynasty.portraitMaleNodeId, createdAt: base.activity[0]!.createdAt }],
                female: [{ assetId: "female-asset", nodeId: dynasty.portraitFemaleNodeId, createdAt: base.activity[0]!.createdAt }],
            },
        })),
    };
    const replaced = replaceWeddingSourcePhoto(
        withPortrait,
        "groom",
        withPortrait.source.groomPhotoNodeId,
        "2026-10-25T03:00:00.000Z",
    );

    expect(replaced.source.groomPhotoReady).toBe(true);
    expect(replaced.dynasties[0]?.portraitSelection).toEqual({});
    expect(replaced.dynasties[0]?.portraitVersionHistory).toEqual({});
    expect(replaced.dynasties[0]?.statuses).toEqual({
        portrait: "stale",
        scene: "stale",
        video: "stale",
    });
    expect(replaced.stages.delivery.status).toBe("stale");
});

test("reconciles legacy photo ids against actual canvas node content", () => {
    const reconciled = reconcileWeddingSourcePhotoReadiness(base, [
        { id: "groom-node", metadata: { uploadField: "groomPhoto" } },
        { id: "bride-node", metadata: { uploadField: "bridePhoto", content: "https://example.com/bride.jpg", storageKey: "resource:bride" } },
    ]);

    expect(reconciled.source).toMatchObject({
        groomPhotoNodeId: "",
        groomPhotoReady: false,
        bridePhotoNodeId: "bride-node",
        bridePhotoReady: true,
    });
});
