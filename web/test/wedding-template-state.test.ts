import { expect, test } from "bun:test";

import {
    buildWeddingProductionFromCanvasNodes,
    buildWeddingProductionFromTemplate,
} from "@/lib/wedding/template-state";

test("maps selected dynasty nodes into production state", () => {
    const state = buildWeddingProductionFromTemplate(
        {
            id: "template-1",
            itemKey: "cross-era-wedding-volc",
            name: "跨时空婚礼电影",
            payloadJson: JSON.stringify({
                version: "1.0.0",
                forms: {
                    setup: {
                        steps: [{
                            id: "dynasties",
                            fields: [{
                                key: "dynasties",
                                type: "checkboxGroup",
                                options: [{ label: "唐 · 大唐盛世", value: "tang" }],
                            }],
                        }],
                    },
                },
            }),
        },
        {
            maleName: "林屿",
            femaleName: "周棠",
            customDesc: "杭州西湖相识",
            dynasties: ["tang", "ming"],
            ratio: "16:9",
            resolution: "720p",
            consentConfirmed: true,
        },
        [
            { id: "groom", content: "https://example.com/groom.jpg", storageKey: "resource:groom", metadata: { uploadField: "groomPhoto" } },
            { id: "bride", content: "https://example.com/bride.jpg", storageKey: "resource:bride", metadata: { uploadField: "bridePhoto" } },
            { id: "male-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "female-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitFemale", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "scene-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyScene", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "video-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyVideo", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "male-ming", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale", creationTemplateDynastyId: "ming", creationTemplateDynastyName: "明" } } },
            { id: "female-ming", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitFemale", creationTemplateDynastyId: "ming", creationTemplateDynastyName: "明" } } },
            { id: "scene-ming", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyScene", creationTemplateDynastyId: "ming", creationTemplateDynastyName: "明" } } },
            { id: "video-ming", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyVideo", creationTemplateDynastyId: "ming", creationTemplateDynastyName: "明" } } },
        ],
        "2026-10-25T00:00:00.000Z",
    );

    expect(state.template).toEqual({
        id: "template-1",
        key: "cross-era-wedding-volc",
        version: "1.0.0",
    });
    expect(state.source).toEqual({
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        groomPhotoReady: true,
        bridePhotoReady: true,
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    });
    expect(state.dynasties.map((dynasty) => ({ id: dynasty.id, name: dynasty.name }))).toEqual([
        { id: "tang", name: "唐" },
        { id: "ming", name: "明" },
    ]);
    expect(state.dynasties[0]).toMatchObject({
        portraitMaleNodeId: "male-tang",
        portraitFemaleNodeId: "female-tang",
        sceneNodeId: "scene-tang",
        videoNodeId: "video-tang",
    });
});

test("rejects incomplete dynasty assets with a precise error", () => {
    expect(() => buildWeddingProductionFromTemplate(
        {
            id: "template-1",
            itemKey: "cross-era-wedding-volc",
            name: "跨时空婚礼电影",
            payloadJson: JSON.stringify({
                version: "1.0.0",
                forms: {
                    setup: {
                        steps: [{
                            id: "dynasties",
                            fields: [{
                                key: "dynasties",
                                type: "checkboxGroup",
                                options: [{ label: "唐 · 大唐盛世", value: "tang" }],
                            }],
                        }],
                    },
                },
            }),
        },
        { dynasties: ["tang"], ratio: "16:9", resolution: "720p" },
        [
            { id: "groom", metadata: { uploadField: "groomPhoto" } },
            { id: "bride", metadata: { uploadField: "bridePhoto" } },
        ],
        "2026-10-25T00:00:00.000Z",
    )).toThrow("唐：缺少新郎定妆照节点");
});

test("backfills production state from an existing cross-era canvas", () => {
    const state = buildWeddingProductionFromCanvasNodes(
        [
            {
                id: "groom",
                content: "https://example.com/groom.jpg",
                storageKey: "resource:groom",
                metadata: {
                    uploadField: "groomPhoto",
                    pluginData: {
                        creationTemplateId: "template-1",
                        creationTemplateKey: "cross-era-wedding-volc",
                        creationTemplateVersion: "1.0.0",
                        creationTemplateVariables: {
                            maleName: "林屿",
                            femaleName: "周棠",
                            customDesc: "杭州西湖相识",
                            dynasties: ["tang"],
                            ratio: "16:9",
                            resolution: "720p",
                            consentConfirmed: true,
                        },
                    },
                },
            },
            {
                id: "bride",
                content: "https://example.com/bride.jpg",
                storageKey: "resource:bride",
                metadata: {
                    uploadField: "bridePhoto",
                    pluginData: {
                        creationTemplateKey: "cross-era-wedding-volc",
                    },
                },
            },
            { id: "male-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "female-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyPortraitFemale", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "scene-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyScene", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
            { id: "video-tang", metadata: { pluginData: { creationTemplateDynamicRef: "dynastyVideo", creationTemplateDynastyId: "tang", creationTemplateDynastyName: "唐" } } },
        ],
        "2026-10-25T06:30:00.000Z",
    );

    expect(state).toMatchObject({
        template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
        source: {
            groomPhotoNodeId: "groom",
            bridePhotoNodeId: "bride",
            groomPhotoReady: true,
            bridePhotoReady: true,
            maleName: "林屿",
            femaleName: "周棠",
        },
        dynasties: [{
            id: "tang",
            name: "唐",
            portraitMaleNodeId: "male-tang",
            videoNodeId: "video-tang",
        }],
    });
});

test("does not backfill production state for another workflow", () => {
    expect(buildWeddingProductionFromCanvasNodes(
        [{ id: "other", metadata: { pluginData: { creationTemplateKey: "other-workflow" } } }],
        "2026-10-25T06:30:00.000Z",
    )).toBeUndefined();
});
