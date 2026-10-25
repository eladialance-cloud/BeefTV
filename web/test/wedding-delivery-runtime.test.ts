import { expect, test } from "bun:test";

import {
    confirmWeddingDelivery,
    markWeddingDeliveryRendering,
    recordWeddingDeliveryResult,
    verifyWeddingDeliveryPlayable,
} from "@/lib/wedding/delivery-runtime";
import { createWeddingProductionState } from "@/lib/wedding/production-state";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
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
production.dynasties[0]!.statuses = { portrait: "approved", scene: "approved", video: "approved" };
production.packaging = {
    ...production.packaging,
    status: "approved",
    titleCardNodeIds: ["intro", "outro"],
    bgmNodeId: "bgm",
    timelineRevision: 1,
    previewReadyAt: "2026-10-25T08:00:00.000Z",
};
production.stages.delivery = { status: "ready", updatedAt: "2026-10-25T08:00:00.000Z" };

test("render lifecycle preserves task identity and requires playable verification", () => {
    const rendering = markWeddingDeliveryRendering(production, "task-render", "2026-10-25T09:00:00.000Z");
    expect(rendering.delivery).toMatchObject({
        status: "rendering",
        taskId: "task-render",
    });

    const completed = recordWeddingDeliveryResult(rendering, {
        resourceId: "resource:film",
        fileName: "cross-era-wedding.mp4",
        durationMs: 27000,
        size: 123456,
    }, "2026-10-25T09:10:00.000Z");
    expect(completed.delivery).toMatchObject({
        status: "needs_review",
        taskId: "task-render",
        resourceId: "resource:film",
        fileName: "cross-era-wedding.mp4",
    });
    expect(confirmWeddingDelivery(completed, "2026-10-25T09:20:00.000Z")).toBe(completed);

    const playable = verifyWeddingDeliveryPlayable(completed, {
        durationMs: 27000,
        videoWidth: 1280,
        videoHeight: 720,
    }, "2026-10-25T09:15:00.000Z");
    expect(playable.delivery?.playableVerifiedAt).toBe("2026-10-25T09:15:00.000Z");
    expect(playable.delivery?.qc).toMatchObject({
        durationMs: 27000,
        width: 1280,
        height: 720,
    });
    const approved = confirmWeddingDelivery(playable, "2026-10-25T09:20:00.000Z");
    expect(approved.delivery?.status).toBe("approved");
    expect(approved.stages.delivery.status).toBe("approved");
});

test("zero-duration or missing output cannot be verified playable", () => {
    const completed = recordWeddingDeliveryResult(production, {
        resourceId: "resource:film",
        durationMs: 0,
    }, "2026-10-25T09:10:00.000Z");
    expect(verifyWeddingDeliveryPlayable(completed, {
        durationMs: 0,
        videoWidth: 0,
        videoHeight: 0,
    }, "2026-10-25T09:15:00.000Z")).toBe(completed);
});
