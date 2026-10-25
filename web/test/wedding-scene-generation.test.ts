import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { generateWeddingScenes } from "@/services/wedding-scene-generation";
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
production.dynasties[0]!.portraitSelection = { male: "asset-male-tang", female: "asset-female-tang" };
production.dynasties[0]!.portraitApproval = { male: true, female: true };
production.dynasties[0]!.portraitVersionHistory = {
    male: [{
        assetId: "asset-male-tang",
        nodeId: "male-tang",
        storageKey: "resource:male-tang",
        previewUrl: "https://example.com/male-tang.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
    female: [{
        assetId: "asset-female-tang",
        nodeId: "female-tang",
        storageKey: "resource:female-tang",
        previewUrl: "https://example.com/female-tang.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
};

test("generates a scene from both confirmed portraits through the canvas task chain", async () => {
    const nodes: CanvasNodeData[] = [{
        id: "scene-tang",
        type: "image",
        title: "04 唐 · 双人婚礼场景",
        position: { x: 800, y: 0 },
        width: 520,
        height: 320,
        metadata: {
            generationMode: "image",
            nodeRole: "generator",
            pluginData: {
                creationTemplateId: "template-1",
                creationTemplateKey: "cross-era-wedding-volc",
                creationTemplateDynamicRef: "dynastyScene",
                creationTemplateDynastyId: "tang",
                creationTemplateVariables: {
                    maleName: "林屿",
                    femaleName: "周棠",
                    customDesc: "杭州西湖相识",
                    dynasties: ["tang", "ming"],
                    ratio: "16:9",
                },
            },
        },
    }];
    let currentProject = {
        id: "canvas-1",
        nodes,
        weddingProduction: production,
    };
    let submitted: Parameters<typeof import("@/services/api/generation-task").runBackendGenerationTask>[0] | undefined;
    const succeededTask: GenerationTask = {
        id: "task-scene-tang",
        type: "canvas_image",
        status: "succeeded",
        progress: 100,
        prompt: "04 唐 · 双人婚礼场景",
        attempts: 1,
        createdAt: "2026-10-25T03:00:00.000Z",
        updatedAt: "2026-10-25T03:02:00.000Z",
        outputs: [{ outputIndex: 0, mediaType: "image", materializedAssetId: "asset-scene-tang" }],
    };

    const summary = await generateWeddingScenes({
        projectId: "canvas-1",
        production,
        config: {
            ...defaultConfig,
            channels: [{
                id: "channel-image",
                name: "图片渠道",
                baseUrl: "https://example.com",
                apiKey: "key",
                apiFormat: "openai",
                interfaceType: "openai-image",
                models: ["image-model"],
                modelProfiles: [{ model: "image-model", protocol: "openai-image" }],
                headers: [],
                enabled: true,
                scope: "user",
            }],
            imageModel: "channel-image::image-model",
            model: "channel-image::image-model",
            size: "16:9",
            count: "1",
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
                return { mode: "image", images: [{ dataUrl: "https://example.com/scene-tang.png" }] };
            },
            consumeTask: async (task, nodeId, outputIndex, consumer) => {
                const output = task.outputs?.find((item) => item.outputIndex === outputIndex)!;
                await consumer({ task, output, effectKey: "effect-scene" });
                return task;
            },
            getAsset: (assetId) => {
                const assets = {
                    "asset-male-tang": {
                        id: "asset-male-tang",
                        kind: "image",
                        coverUrl: "https://example.com/male-tang.png",
                        data: {
                            dataUrl: "https://example.com/male-tang.png",
                            storageKey: "resource:male-tang",
                            width: 1024,
                            height: 1024,
                            bytes: 100,
                            mimeType: "image/png",
                        },
                    },
                    "asset-female-tang": {
                        id: "asset-female-tang",
                        kind: "image",
                        coverUrl: "https://example.com/female-tang.png",
                        data: {
                            dataUrl: "https://example.com/female-tang.png",
                            storageKey: "resource:female-tang",
                            width: 1024,
                            height: 1024,
                            bytes: 100,
                            mimeType: "image/png",
                        },
                    },
                    "asset-scene-tang": {
                        id: "asset-scene-tang",
                        kind: "image",
                        coverUrl: "https://example.com/scene-tang.png",
                        data: {
                            dataUrl: "https://example.com/scene-tang.png",
                            storageKey: "resource:scene-tang",
                            width: 1536,
                            height: 864,
                            bytes: 200,
                            mimeType: "image/png",
                        },
                    },
                };
                return assets[assetId as keyof typeof assets];
            },
        },
    });

    expect(summary).toEqual({ attempted: 1, succeeded: 1, failed: 0, errors: [] });
    expect(submitted).toMatchObject({
        projectId: "canvas-1",
        mode: "image",
        prompt: "04 唐 · 双人婚礼场景",
        referenceImages: [
            { id: "asset-male-tang", storageKey: "resource:male-tang" },
            { id: "asset-female-tang", storageKey: "resource:female-tang" },
        ],
        metadata: {
            source: "wedding-workbench",
            stage: "scenes",
            dynastyId: "tang",
            creationTemplate: {
                id: "template-1",
                nodeRef: "dynastyScene",
                dynastyId: "tang",
            },
        },
    });
    expect(currentProject.nodes[0]?.metadata).toMatchObject({
        status: "success",
        content: "https://example.com/scene-tang.png",
        storageKey: "resource:scene-tang",
        assetId: "asset-scene-tang",
    });
    expect(currentProject.weddingProduction.dynasties[0]).toMatchObject({
        sceneSelection: "asset-scene-tang",
        sceneVersions: [{
            assetId: "asset-scene-tang",
            nodeId: "scene-tang",
            taskId: "task-scene-tang",
        }],
    });
    expect(currentProject.weddingProduction.dynasties[0]?.qc.scene).toBeUndefined();
});
