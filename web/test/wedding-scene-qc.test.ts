import { expect, test } from "bun:test";

import {
    applyWeddingSceneQc,
    overrideWeddingSceneQc,
    parseWeddingSceneQc,
} from "@/lib/wedding/scene-qc";
import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { runWeddingSceneQc } from "@/services/wedding-scene-generation";
import { defaultConfig } from "@/stores/use-config-store";

const now = "2026-10-25T05:00:00.000Z";
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
production.dynasties[0]!.portraitSelection = { male: "asset-male", female: "asset-female" };
production.dynasties[0]!.portraitApproval = { male: true, female: true };
production.dynasties[0]!.portraitVersionHistory = {
    male: [{
        assetId: "asset-male",
        nodeId: "male-tang",
        storageKey: "resource:male",
        previewUrl: "https://example.com/male.png",
        createdAt: now,
    }],
    female: [{
        assetId: "asset-female",
        nodeId: "female-tang",
        storageKey: "resource:female",
        previewUrl: "https://example.com/female.png",
        createdAt: now,
    }],
};
production.dynasties[0]!.sceneSelection = "asset-scene";
production.dynasties[0]!.sceneVersions = [{
    assetId: "asset-scene",
    nodeId: "scene-tang",
    storageKey: "resource:scene",
    previewUrl: "https://example.com/scene.png",
    createdAt: now,
}];

function qcPayload(patch: Record<string, unknown> = {}) {
    return {
        verdict: "pass",
        couplePresent: true,
        faceMatchMale: true,
        faceMatchFemale: true,
        outfitMatch: true,
        sceneMatch: true,
        extraPerson: false,
        watermarkOrText: false,
        reason: "男女身份、服饰与场景一致。",
        suggestion: "",
        ...patch,
    };
}

test("parses strict QC and derives failure from hard boolean checks", () => {
    const qc = parseWeddingSceneQc(JSON.stringify(qcPayload({
        faceMatchMale: false,
        reason: "新郎脸部漂移",
        suggestion: "提高男女肖像参考权重",
    })), now);

    expect(qc).toMatchObject({
        verdict: "fail",
        faceMatchMale: false,
        reason: "新郎脸部漂移",
        checkedAt: now,
    });
});

test("QC errors and overrides require explicit evidence", () => {
    const failed = applyWeddingSceneQc(production, "tang", parseWeddingSceneQc(
        JSON.stringify(qcPayload({ faceMatchFemale: false })),
        now,
    ), now);
    expect(failed.dynasties[0]?.statuses.scene).toBe("needs_review");
    expect(overrideWeddingSceneQc(failed, "tang", "", now)).toBe(failed);

    const overridden = overrideWeddingSceneQc(failed, "tang", "制作方接受轻微手指瑕疵，不影响主体。", now);
    expect(overridden.dynasties[0]?.qc.overrides.scene).toEqual({
        reason: "制作方接受轻微手指瑕疵，不影响主体。",
        acceptedAt: now,
    });
    expect(overridden.dynasties[0]?.statuses.scene).toBe("ready");
});

test("runs scene QC with portraits and scene references and persists passing checks", async () => {
    const nodes = [{
        id: "scene-tang",
        type: "image",
        title: "唐 · 双人婚礼场景",
        position: { x: 0, y: 0 },
        width: 520,
        height: 320,
        metadata: {
            content: "https://example.com/scene.png",
            storageKey: "resource:scene",
            assetId: "asset-scene",
            pluginData: {
                creationTemplateId: "template-1",
                creationTemplateDynamicRef: "dynastyScene",
                creationTemplateDynastyId: "tang",
            },
        },
    }];
    let currentProject = { id: "canvas-1", nodes, weddingProduction: production };
    let submitted: Parameters<typeof import("@/services/api/generation-task").runBackendGenerationTask>[0] | undefined;
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
    };

    const qc = await runWeddingSceneQc({
        projectId: "canvas-1",
        production,
        config: {
            ...defaultConfig,
            channels: [{
                id: "channel-text",
                name: "视觉检查",
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
            { id: "asset-male", storageKey: "resource:male" },
            { id: "asset-female", storageKey: "resource:female" },
            { id: "asset-scene", storageKey: "resource:scene" },
        ],
        metadata: {
            source: "wedding-workbench",
            stage: "scene-qc",
            dynastyId: "tang",
        },
    });
    expect(currentProject.weddingProduction.dynasties[0]?.qc.scene).toMatchObject({
        verdict: "pass",
        faceMatchMale: true,
        faceMatchFemale: true,
    });
    expect(currentProject.weddingProduction.dynasties[0]?.statuses.scene).toBe("ready");
});
