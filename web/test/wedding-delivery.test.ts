import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { renderWeddingDelivery, weddingDeliveryManifest } from "@/services/wedding-delivery";
import type { TimelineProject } from "@/types/timeline";

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
production.dynasties[0]!.videoSelection = "video-asset";
production.dynasties[0]!.videoVersions = [{
    assetId: "video-asset",
    nodeId: "video",
    previewUrl: "https://example.com/video.mp4",
    taskId: "video-task",
    createdAt: "2026-10-25T07:00:00.000Z",
}];
production.dynasties[0]!.promptOverride = "PRODUCER PROMPT";
production.packaging = {
    ...production.packaging,
    status: "approved",
    titleCardNodeIds: ["intro", "outro"],
    bgmNodeId: "bgm",
    timelineRevision: 1,
    previewReadyAt: "2026-10-25T08:00:00.000Z",
};
const timeline: TimelineProject = {
    version: 2,
    tracks: [{ id: "video-1", kind: "video", label: "成片", order: 0 }],
    clips: [],
    durationMs: 27000,
};

test("submits the exact persisted timeline and records the render result", async () => {
    let persisted = production;
    let submittedTimeline: TimelineProject | undefined;
    const result = await renderWeddingDelivery({
        projectId: "canvas-1",
        production,
        timeline,
        clientOperationId: "render-1",
    }, {
            runRender: async (input) => {
                submittedTimeline = input.timeline;
                input.onTaskUpdate?.({
                    id: "render-task",
                    type: "timeline_render",
                    status: "running",
                    prompt: "timeline",
                    attempts: 1,
                    createdAt: "2026-10-25T09:00:00.000Z",
                    updatedAt: "2026-10-25T09:01:00.000Z",
                    progress: 50,
                });
                return {
                    task: {
                        id: "render-task",
                        type: "timeline_render",
                        status: "succeeded",
                        prompt: "timeline",
                        attempts: 1,
                        createdAt: "2026-10-25T09:00:00.000Z",
                        updatedAt: "2026-10-25T09:10:00.000Z",
                    },
                    result: {
                        resourceId: "resource:film",
                        fileName: "cross-era-wedding.mp4",
                        durationMs: 27000,
                        size: 123456,
                    },
                };
            },
            persist: async (_projectId, next) => {
                persisted = next;
            },
    });

    expect(submittedTimeline).toEqual(timeline);
    expect(result.resourceId).toBe("resource:film");
    expect(persisted.delivery).toMatchObject({
        status: "needs_review",
        taskId: "render-task",
        resourceId: "resource:film",
    });
});

test("exports a production manifest with versions, prompts and QC", () => {
    const completed = {
        ...production,
        delivery: {
            status: "approved" as const,
            taskId: "render-task",
            resourceId: "resource:film",
            fileName: "cross-era-wedding.mp4",
            playableVerifiedAt: "2026-10-25T09:15:00.000Z",
        },
    };
    const manifest = JSON.parse(weddingDeliveryManifest(completed));
    expect(manifest).toMatchObject({
        template: production.template,
        source: production.source,
        delivery: {
            taskId: "render-task",
            resourceId: "resource:film",
        },
        dynasties: [{
            id: "tang",
            promptOverride: "PRODUCER PROMPT",
            videoSelection: "video-asset",
        }],
    });
});
