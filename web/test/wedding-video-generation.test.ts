import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { generateWeddingVideos } from "@/services/wedding-video-generation";
import { defaultConfig } from "@/stores/use-config-store";
import type { GenerationTask } from "@/services/api/task-center";
import type { CanvasNodeData } from "@/types/canvas";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
    source: {
        groomPhotoNodeId: "groom-photo",
        bridePhotoNodeId: "bride-photo",
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [
        {
            id: "tang",
            name: "唐",
            order: 0,
            portraitMaleNodeId: "male-tang",
            portraitFemaleNodeId: "female-tang",
            sceneNodeId: "scene-tang",
            videoNodeId: "video-tang",
        },
        {
            id: "ming",
            name: "明",
            order: 1,
            portraitMaleNodeId: "male-ming",
            portraitFemaleNodeId: "female-ming",
            sceneNodeId: "scene-ming",
            videoNodeId: "video-ming",
        },
    ],
    now: "2026-10-25T00:00:00.000Z",
});
const tang = production.dynasties[0]!;
tang.portraitSelection = { male: "asset-male", female: "asset-female" };
tang.portraitApproval = { male: true, female: true };
tang.portraitVersionHistory = {
    male: [{
        assetId: "asset-male",
        nodeId: "male-tang",
        storageKey: "resource:male",
        previewUrl: "https://example.com/male.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
    female: [{
        assetId: "asset-female",
        nodeId: "female-tang",
        storageKey: "resource:female",
        previewUrl: "https://example.com/female.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
};
tang.sceneSelection = "asset-scene";
tang.sceneVersions = [{
    assetId: "asset-scene",
    nodeId: "scene-tang",
    storageKey: "resource:scene",
    previewUrl: "https://example.com/scene.png",
    createdAt: "2026-10-25T02:00:00.000Z",
}];
tang.qc.scene = {
    verdict: "pass",
    couplePresent: true,
    faceMatchMale: true,
    faceMatchFemale: true,
    outfitMatch: true,
    sceneMatch: true,
    extraPerson: false,
    watermarkOrText: false,
    reason: "通过",
    suggestion: "",
    checkedAt: "2026-10-25T03:00:00.000Z",
};
tang.statuses.scene = "approved";
tang.promptOverride = "PRODUCER EDITED VIDEO PROMPT";

test("generates video from confirmed portraits and scene with producer prompt override", async () => {
    const nodes: CanvasNodeData[] = [{
        id: "video-tang",
        type: "video",
        title: "05 唐 · 朝代视频",
        position: { x: 1200, y: 0 },
        width: 520,
        height: 360,
        metadata: {
            generationMode: "video",
            nodeRole: "generator",
            videoEditOperation: "reference_to_video",
            seconds: "8",
            size: "16:9",
            vquality: "720p",
            pluginData: {
                creationTemplateId: "template-1",
                creationTemplateKey: "cross-era-wedding-volc",
                creationTemplateDynamicRef: "dynastyVideo",
                creationTemplateDynastyId: "tang",
                creationTemplateVariables: {
                    maleName: "林屿",
                    femaleName: "周棠",
                    customDesc: "杭州西湖相识",
                    dynasties: ["tang", "ming"],
                    ratio: "16:9",
                    resolution: "720p",
                },
            },
        },
    }];
    let currentProject = { id: "canvas-1", nodes, weddingProduction: production };
    let submitted: Parameters<typeof import("@/services/api/generation-task").runBackendGenerationTask>[0] | undefined;
    const succeededTask: GenerationTask = {
        id: "task-video-tang",
        type: "canvas_video",
        status: "succeeded",
        progress: 100,
        prompt: "05 唐 · 朝代视频",
        attempts: 1,
        createdAt: "2026-10-25T04:00:00.000Z",
        updatedAt: "2026-10-25T04:08:00.000Z",
        outputs: [{ outputIndex: 0, mediaType: "video", materializedAssetId: "asset-video-tang" }],
    };
    const assets = {
        "asset-male": {
            id: "asset-male",
            kind: "image",
            data: {
                dataUrl: "https://example.com/male.png",
                storageKey: "resource:male",
                width: 1024,
                height: 1024,
                bytes: 100,
                mimeType: "image/png",
            },
        },
        "asset-female": {
            id: "asset-female",
            kind: "image",
            data: {
                dataUrl: "https://example.com/female.png",
                storageKey: "resource:female",
                width: 1024,
                height: 1024,
                bytes: 100,
                mimeType: "image/png",
            },
        },
        "asset-scene": {
            id: "asset-scene",
            kind: "image",
            data: {
                dataUrl: "https://example.com/scene.png",
                storageKey: "resource:scene",
                width: 1536,
                height: 864,
                bytes: 200,
                mimeType: "image/png",
            },
        },
        "asset-video-tang": {
            id: "asset-video-tang",
            kind: "video",
            coverUrl: "https://example.com/video-poster.jpg",
            data: {
                url: "https://example.com/video-tang.mp4",
                storageKey: "resource:video-tang",
                width: 1280,
                height: 720,
                durationMs: 8000,
                bytes: 2000,
                mimeType: "video/mp4",
            },
        },
    };

    const summary = await generateWeddingVideos({
        projectId: "canvas-1",
        production,
        config: {
            ...defaultConfig,
            channels: [{
                id: "channel-video",
                name: "视频渠道",
                baseUrl: "https://example.com",
                apiKey: "key",
                apiFormat: "openai",
                interfaceType: "openai-compatible-video",
                models: ["video-model"],
                modelProfiles: [{ model: "video-model", protocol: "openai-compatible-video" }],
                headers: [],
                enabled: true,
                scope: "user",
            }],
            videoModel: "channel-video::video-model",
            model: "channel-video::video-model",
            videoSeconds: "8",
            size: "16:9",
            vquality: "720p",
        },
    }, "tang", {
        force: true,
        dependencies: {
            getProject: () => currentProject,
            persist: async (_projectId, patch) => {
                currentProject = { ...currentProject, ...patch };
            },
            runTask: async (options) => {
                submitted = options;
                options.onTaskUpdate?.(succeededTask);
                return {
                    mode: "video",
                    video: { dataUrl: "https://example.com/video-tang.mp4", durationMs: 8000 },
                };
            },
            consumeTask: async (task, nodeId, outputIndex, consumer) => {
                const output = task.outputs?.find((item) => item.outputIndex === outputIndex)!;
                await consumer({ task, output, effectKey: "effect-video" });
                return task;
            },
            getAsset: (assetId) => assets[assetId as keyof typeof assets],
        },
    });

    expect(summary).toEqual({ attempted: 1, succeeded: 1, failed: 0, errors: [] });
    expect(submitted).toMatchObject({
        projectId: "canvas-1",
        mode: "video",
        prompt: "05 唐 · 朝代视频",
        referenceImages: [
            { id: "asset-male", storageKey: "resource:male" },
            { id: "asset-female", storageKey: "resource:female" },
            { id: "asset-scene", storageKey: "resource:scene" },
        ],
        metadata: {
            source: "wedding-workbench",
            stage: "videos",
            dynastyId: "tang",
            videoEditOperation: "reference_to_video",
            creationTemplate: {
                id: "template-1",
                nodeRef: "dynastyVideo",
                dynastyId: "tang",
                promptOverride: "PRODUCER EDITED VIDEO PROMPT",
            },
        },
    });
    expect(currentProject.nodes[0]?.metadata).toMatchObject({
        status: "success",
        content: "https://example.com/video-tang.mp4",
        storageKey: "resource:video-tang",
        assetId: "asset-video-tang",
        durationMs: 8000,
    });
    expect(currentProject.weddingProduction.dynasties[0]).toMatchObject({
        videoSelection: "asset-video-tang",
        videoVersions: [{
            assetId: "asset-video-tang",
            nodeId: "video-tang",
            taskId: "task-video-tang",
            durationMs: 8000,
        }],
        qc: {
            video: {
                verdict: "pending",
            },
        },
    });
    expect(currentProject.weddingProduction.packaging.status).toBe("stale");
});
