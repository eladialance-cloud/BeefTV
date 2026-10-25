import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import {
    confirmWeddingPortrait,
    failWeddingPortraitGeneration,
    markWeddingPortraitGenerating,
    recordWeddingPortraitVersion,
    selectWeddingPortraitVersion,
    weddingPortraitJobs,
} from "@/lib/wedding/portrait-runtime";
import { approveWeddingStage, weddingStageBlockers } from "@/lib/wedding/production-state";
import { defaultConfig } from "@/stores/use-config-store";
import { generateWeddingPortraits } from "@/services/wedding-generation";
import type { CanvasNodeData } from "@/types/canvas";
import type { GenerationTask } from "@/services/api/task-center";

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

test("maps two portrait jobs per dynasty to the correct source photos", () => {
    expect(weddingPortraitJobs(production)).toEqual([
        { nodeId: "male-tang", dynastyId: "tang", kind: "portrait", gender: "male", promptSourceNodeId: "groom-photo" },
        { nodeId: "female-tang", dynastyId: "tang", kind: "portrait", gender: "female", promptSourceNodeId: "bride-photo" },
        { nodeId: "male-ming", dynastyId: "ming", kind: "portrait", gender: "male", promptSourceNodeId: "groom-photo" },
        { nodeId: "female-ming", dynastyId: "ming", kind: "portrait", gender: "female", promptSourceNodeId: "bride-photo" },
    ]);
});

test("recording a generated portrait requires explicit approval and keeps the other gender isolated", () => {
    const [maleJob, femaleJob] = weddingPortraitJobs(production, "tang");
    const previous = {
        ...production,
        dynasties: production.dynasties.map((dynasty) => dynasty.id === "tang"
            ? {
                ...dynasty,
                portraitApproval: { male: true, female: true },
                portraitSelection: { male: "old-male", female: "old-female" },
                portraitVersionHistory: {
                    male: [{
                        assetId: "old-male",
                        nodeId: dynasty.portraitMaleNodeId,
                        createdAt: "2026-10-24T00:00:00.000Z",
                    }],
                    female: [{
                        assetId: "old-female",
                        nodeId: dynasty.portraitFemaleNodeId,
                        createdAt: "2026-10-24T00:00:00.000Z",
                    }],
                },
            }
            : dynasty),
    };
    const generating = markWeddingPortraitGenerating(previous, maleJob!, "2026-10-25T01:00:00.000Z");
    expect(generating.dynasties[0]?.portraitApproval).toEqual({ male: false, female: true });
    expect(generating.dynasties[0]?.statuses.portrait).toBe("generating");
    expect(generating.dynasties[1]).toEqual(previous.dynasties[1]);

    const generated = recordWeddingPortraitVersion(generating, maleJob!, {
        assetId: "asset-male-v2",
        storageKey: "image:asset-male-v2",
        previewUrl: "https://example.com/male-v2.png",
        taskId: "task-male-v2",
        createdAt: "2026-10-25T01:02:00.000Z",
    });
    expect(generated.dynasties[0]?.portraitVersionHistory?.male).toHaveLength(2);
    expect(generated.dynasties[0]?.portraitVersionHistory?.male[0]).toMatchObject({ assetId: "asset-male-v2" });
    expect(generated.dynasties[0]?.portraitApproval).toEqual({ male: false, female: true });
    expect(generated.dynasties[0]?.statuses.portrait).toBe("needs_review");
    expect(generated.dynasties[1]).toEqual(previous.dynasties[1]);

    const approved = confirmWeddingPortrait(generated, "tang", "male", "asset-male-v2", "2026-10-25T01:03:00.000Z");
    expect(approved.dynasties[0]?.portraitApproval).toEqual({ male: true, female: true });
    expect(weddingStageBlockers(approved, "portraits")).toContain("明：新郎定妆照尚未确认");
    expect(approveWeddingStage(approved, "portraits", "2026-10-25T01:04:00.000Z")).toBe(approved);
    expect(femaleJob?.gender).toBe("female");
});

test("selecting a portrait version revokes only that gender approval", () => {
    const reviewed = {
        ...production,
        dynasties: production.dynasties.map((dynasty) => dynasty.id === "tang"
            ? {
                ...dynasty,
                portraitSelection: { male: "asset-new", female: "asset-female" },
                portraitApproval: { male: true, female: true },
                portraitVersionHistory: {
                    male: [{ assetId: "asset-old", nodeId: dynasty.portraitMaleNodeId, createdAt: "2026-10-24T00:00:00.000Z" }],
                },
            }
            : dynasty),
    };
    const selected = selectWeddingPortraitVersion(
        reviewed,
        "tang",
        "male",
        "asset-old",
        "2026-10-25T02:30:00.000Z",
    );

    expect(selected.dynasties[0]?.portraitSelection).toEqual({ male: "asset-old", female: "asset-female" });
    expect(selected.dynasties[0]?.portraitApproval).toEqual({ male: false, female: true });
    expect(selected.dynasties[0]?.statuses.portrait).toBe("needs_review");
    expect(selected.dynasties[1]).toEqual(reviewed.dynasties[1]);
});

test("a failed portrait marks only its dynasty and the other dynasty remains reviewable", () => {
    const [maleTang] = weddingPortraitJobs(production, "tang");
    const failed = failWeddingPortraitGeneration(
        production,
        maleTang!,
        "参考照片读取失败",
        "2026-10-25T02:00:00.000Z",
    );

    expect(failed.dynasties[0]?.statuses.portrait).toBe("failed");
    expect(failed.stages.portraits.status).toBe("needs_review");
    expect(failed.dynasties[1]?.statuses.portrait).toBe("ready");
    expect(failed.activity.at(-1)).toMatchObject({
        type: "error",
        stage: "portraits",
        dynastyId: "tang",
        message: "唐：新郎定妆照生成失败 · 参考照片读取失败",
    });
});

test("generates a portrait through the existing canvas task chain and persists the selected version", async () => {
    const nodes: CanvasNodeData[] = [
        {
            id: "groom-photo",
            type: "image",
            title: "新郎原始照片",
            position: { x: 0, y: 0 },
            width: 320,
            height: 240,
            metadata: {
                content: "https://example.com/groom.jpg",
                storageKey: "image:groom-photo",
                mimeType: "image/jpeg",
            },
        },
        {
            id: "male-tang",
            type: "image",
            title: "02 唐 · 新郎肖像",
            position: { x: 400, y: 0 },
            width: 420,
            height: 280,
            metadata: {
                generationMode: "image",
                nodeRole: "generator",
                pluginData: {
                    creationTemplateId: "template-1",
                    creationTemplateKey: "cross-era-wedding-volc",
                    creationTemplateDynamicRef: "dynastyPortraitMale",
                    creationTemplateDynastyId: "tang",
                    creationTemplateVariables: {
                        maleName: "林屿",
                        femaleName: "周棠",
                        customDesc: "杭州西湖相识",
                        dynasties: ["tang"],
                        ratio: "16:9",
                    },
                },
            },
        },
    ];
    let currentProject = {
        id: "canvas-1",
        nodes,
        weddingProduction: production,
    };
    let submitted: Parameters<typeof import("@/services/api/generation-task").runBackendGenerationTask>[0] | undefined;
    const succeededTask: GenerationTask = {
        id: "task-male-tang",
        type: "canvas_image",
        status: "succeeded",
        progress: 100,
        prompt: "02 唐 · 新郎肖像",
        attempts: 1,
        createdAt: "2026-10-25T03:00:00.000Z",
        updatedAt: "2026-10-25T03:01:00.000Z",
        outputs: [{ outputIndex: 0, mediaType: "image", materializedAssetId: "asset-male-tang" }],
    };

    const summary = await generateWeddingPortraits({
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
        nodeIds: ["male-tang"],
        dependencies: {
            getProject: () => currentProject,
            persist: async (_projectId, patch) => {
                currentProject = { ...currentProject, ...patch };
            },
            runTask: async (options) => {
                submitted = options;
                options.onTaskUpdate?.(succeededTask);
                return { mode: "image", images: [{ dataUrl: "https://example.com/male-tang.png" }] };
            },
            consumeTask: async (task, nodeId, outputIndex, consumer) => {
                const output = task.outputs?.find((item) => item.outputIndex === outputIndex)!;
                await consumer({ task, output, effectKey: "effect-1" });
                return task;
            },
            getAsset: (assetId) => assetId === "asset-male-tang"
                ? {
                    id: assetId,
                    kind: "image",
                    title: "唐 · 新郎肖像",
                    coverUrl: "https://example.com/male-tang.png",
                    data: {
                        dataUrl: "https://example.com/male-tang.png",
                        storageKey: "image:asset-male-tang",
                        width: 1024,
                        height: 1024,
                        bytes: 1234,
                        mimeType: "image/png",
                    },
                }
                : undefined,
        },
    });

    expect(summary).toEqual({ attempted: 1, succeeded: 1, failed: 0, errors: [] });
    expect(submitted).toMatchObject({
        mode: "image",
        prompt: "02 唐 · 新郎肖像",
        projectId: "canvas-1",
        referenceImages: [{ id: "groom-photo", storageKey: "image:groom-photo" }],
        metadata: {
            source: "wedding-workbench",
            stage: "portraits",
            dynastyId: "tang",
            portraitGender: "male",
            promptSourceNodeId: "groom-photo",
            creationTemplate: {
                id: "template-1",
                key: "cross-era-wedding-volc",
                nodeRef: "dynastyPortraitMale",
                dynastyId: "tang",
            },
        },
    });
    expect(currentProject.nodes.find((node) => node.id === "male-tang")?.metadata).toMatchObject({
        status: "success",
        content: "https://example.com/male-tang.png",
        storageKey: "image:asset-male-tang",
        assetId: "asset-male-tang",
    });
    expect(currentProject.weddingProduction.dynasties[0]).toMatchObject({
        portraitSelection: { male: "asset-male-tang" },
        portraitApproval: { male: false },
        portraitVersionHistory: {
            male: [{
                assetId: "asset-male-tang",
                nodeId: "male-tang",
                taskId: "task-male-tang",
            }],
        },
    });
});
