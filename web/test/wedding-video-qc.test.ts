import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import {
    applyWeddingVideoQc,
    overrideWeddingVideoQc,
    parseWeddingVideoQc,
} from "@/lib/wedding/video-qc";
import { runWeddingVideoQc } from "@/services/wedding-video-generation";
import { defaultConfig } from "@/stores/use-config-store";

const now = "2026-10-25T07:00:00.000Z";
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
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "male-tang",
        portraitFemaleNodeId: "female-tang",
        sceneNodeId: "scene-tang",
        videoNodeId: "video-tang",
    }],
    now: "2026-10-25T00:00:00.000Z",
});
const tang = production.dynasties[0]!;
tang.portraitSelection = { male: "asset-male", female: "asset-female" };
tang.portraitApproval = { male: true, female: true };
tang.portraitVersionHistory = {
    male: [{ assetId: "asset-male", nodeId: "male-tang", storageKey: "resource:male", previewUrl: "https://example.com/male.png", createdAt: now }],
    female: [{ assetId: "asset-female", nodeId: "female-tang", storageKey: "resource:female", previewUrl: "https://example.com/female.png", createdAt: now }],
};
tang.sceneSelection = "asset-scene";
tang.sceneVersions = [{
    assetId: "asset-scene",
    nodeId: "scene-tang",
    storageKey: "resource:scene",
    previewUrl: "https://example.com/scene.png",
    createdAt: now,
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
    checkedAt: now,
};
tang.videoSelection = "asset-video";
tang.videoVersions = [{
    assetId: "asset-video",
    nodeId: "video-tang",
    storageKey: "resource:video",
    previewUrl: "https://example.com/video.mp4",
    durationMs: 8000,
    createdAt: now,
}];

function qcPayload(patch: Record<string, unknown> = {}) {
    return {
        verdict: "pass",
        faceStable: true,
        outfitStable: true,
        actionReadable: true,
        extraPerson: false,
        watermarkOrText: false,
        reason: "人物与服饰稳定，动作可读。",
        ...patch,
    };
}

test("parses video QC and derives failure from hard checks", () => {
    const qc = parseWeddingVideoQc(JSON.stringify(qcPayload({
        faceStable: false,
        reason: "视频中段脸部漂移",
    })), now);
    expect(qc).toMatchObject({
        verdict: "fail",
        faceStable: false,
        reason: "视频中段脸部漂移",
        checkedAt: now,
    });
});

test("video QC override requires explicit reason", () => {
    const failed = applyWeddingVideoQc(production, "tang", parseWeddingVideoQc(
        JSON.stringify(qcPayload({ actionReadable: false })),
        now,
    ), now);
    expect(failed.dynasties[0]?.statuses.video).toBe("needs_review");
    expect(overrideWeddingVideoQc(failed, "tang", "", now)).toBe(failed);
    const overridden = overrideWeddingVideoQc(failed, "tang", "动作幅度轻微但叙事可理解。", now);
    expect(overridden.dynasties[0]?.qc.overrides.video?.reason).toBe("动作幅度轻微但叙事可理解。");
    expect(overridden.dynasties[0]?.statuses.video).toBe("ready");
});

test("runs video QC with portraits, scene and selected video references", async () => {
    const nodes = [{
        id: "video-tang",
        type: "video",
        title: "唐 · 朝代视频",
        position: { x: 0, y: 0 },
        width: 520,
        height: 360,
        metadata: {
            content: "https://example.com/video.mp4",
            storageKey: "resource:video",
            assetId: "asset-video",
        },
    }];
    let currentProject = { id: "canvas-1", nodes, weddingProduction: production };
    let submitted: Parameters<typeof import("@/services/api/generation-task").runBackendGenerationTask>[0] | undefined;
    const assets = {
        "asset-male": { id: "asset-male", kind: "image", data: { dataUrl: "https://example.com/male.png", storageKey: "resource:male", width: 1024, height: 1024, bytes: 100, mimeType: "image/png" } },
        "asset-female": { id: "asset-female", kind: "image", data: { dataUrl: "https://example.com/female.png", storageKey: "resource:female", width: 1024, height: 1024, bytes: 100, mimeType: "image/png" } },
        "asset-scene": { id: "asset-scene", kind: "image", data: { dataUrl: "https://example.com/scene.png", storageKey: "resource:scene", width: 1536, height: 864, bytes: 200, mimeType: "image/png" } },
        "asset-video": { id: "asset-video", kind: "video", data: { url: "https://example.com/video.mp4", storageKey: "resource:video", width: 1280, height: 720, durationMs: 8000, bytes: 2000, mimeType: "video/mp4" } },
    };

    const qc = await runWeddingVideoQc({
        projectId: "canvas-1",
        production,
        config: {
            ...defaultConfig,
            channels: [{
                id: "channel-text",
                name: "视频检查",
                baseUrl: "https://example.com",
                apiKey: "key",
                apiFormat: "openai",
                interfaceType: "openai-compatible",
                models: ["vision-model"],
                modelProfiles: [{ model: "vision-model", protocol: "openai-compatible" }],
                headers: [],
                enabled: true,
                scope: "user",
            }],
            textModel: "channel-text::vision-model",
            model: "channel-text::vision-model",
        },
    }, "tang", {
        dependencies: {
            getProject: () => currentProject,
            persist: async (_projectId, patch) => {
                currentProject = { ...currentProject, ...patch };
            },
            runTask: async (options) => {
                submitted = options;
                return { mode: "text", text: JSON.stringify(qcPayload()) };
            },
            getAsset: (assetId) => assets[assetId as keyof typeof assets],
        },
    });

    expect(qc.verdict).toBe("pass");
    expect(submitted).toMatchObject({
        mode: "text",
        referenceImages: [
            { id: "asset-male" },
            { id: "asset-female" },
            { id: "asset-scene" },
        ],
        referenceVideos: [{ id: "asset-video", storageKey: "resource:video" }],
        metadata: {
            source: "wedding-workbench",
            stage: "video-qc",
            dynastyId: "tang",
        },
    });
    expect(currentProject.weddingProduction.dynasties[0]?.qc.video).toMatchObject({
        verdict: "pass",
        faceStable: true,
        actionReadable: true,
    });
    expect(currentProject.weddingProduction.dynasties[0]?.statuses.video).toBe("ready");
});
