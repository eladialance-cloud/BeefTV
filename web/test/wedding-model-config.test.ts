import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import {
    recommendedWeddingModel,
    weddingConfigWithModels,
    weddingModelSelection,
} from "@/lib/wedding/model-config";
import type { AiConfig } from "@/stores/use-config-store";

const config = {
    channelMode: "remote",
    baseUrl: "https://example.com",
    apiKey: "",
    apiFormat: "openai",
    channels: [{
        id: "beefapi",
        name: "VisiDeep AI",
        baseUrl: "/api/ai/system/beefapi",
        apiKey: "system",
        apiFormat: "openai",
        scope: "system",
        enabled: true,
        models: ["doubao-seedream-5-0", "seedance-2.5", "gemini-3.8-flash", "music-generation"],
        modelProfiles: [
            { model: "doubao-seedream-5-0", displayName: "Seedream 5.0", capability: "image", protocol: "volcengine-ark-image" },
            { model: "seedance-2.5", displayName: "Seedance 2.5", capability: "video", protocol: "volcengine-ark-video" },
            { model: "gemini-3.8-flash", displayName: "Gemini 3.8 Flash", capability: "text", protocol: "chat-completion" },
            { model: "music-generation", displayName: "Music Generation", capability: "audio", protocol: "async-audio" },
        ],
    }],
    runningHub: { enabled: false, baseUrl: "", apiKey: "", walletApiKey: "", uploadApiKey: "", useWallet: false, capability: "image", selectedKind: "workflow", workflowId: "", workflows: [] },
    model: "beefapi::gemini-3.8-flash",
    imageModel: "beefapi::doubao-seedream-5-0",
    videoModel: "beefapi::seedance-2.5",
    textModel: "beefapi::gemini-3.8-flash",
    audioModel: "beefapi::music-generation",
    assistantModel: "",
    audioVoice: "",
    audioFormat: "mp3",
    audioSpeed: "1",
    audioPitch: "0",
    audioVolume: "1",
    audioInstructions: "",
    videoSeconds: "8",
    vquality: "720",
    videoGenerateAudio: "true",
    videoWatermark: "false",
    videoArkPrivateAssetUpload: "true",
    systemPrompt: "",
    models: [],
    imageModels: [],
    videoModels: [],
    textModels: [],
    audioModels: [],
    quality: "auto",
    size: "16:9",
    transparentBackground: "false",
    count: "1",
    canvasImageCount: "1",
} satisfies AiConfig;

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

test("project model selections override global generation defaults", () => {
    const selected = {
        ...production,
        models: {
            imageModel: "beefapi::doubao-seedream-5-0",
            videoModel: "beefapi::seedance-2.5",
            textModel: "beefapi::gemini-3.8-flash",
            audioModel: "beefapi::music-generation",
        },
    };
    expect(weddingModelSelection(selected, config)).toEqual(selected.models);
    expect(weddingConfigWithModels(config, selected)).toMatchObject({
        imageModel: "beefapi::doubao-seedream-5-0",
        videoModel: "beefapi::seedance-2.5",
        textModel: "beefapi::gemini-3.8-flash",
        audioModel: "beefapi::music-generation",
    });
});

test("recommendations resolve to models that exist in the current catalog", () => {
    expect(recommendedWeddingModel(config, "image", ["seedream-5", "gpt-image-2"])).toBe("beefapi::doubao-seedream-5-0");
    expect(recommendedWeddingModel(config, "video", ["seedance-2.5", "seedance-2.0"])).toBe("beefapi::seedance-2.5");
    expect(recommendedWeddingModel(config, "text", ["gemini-3.8-flash", "gpt-5"])).toBe("beefapi::gemini-3.8-flash");
    expect(recommendedWeddingModel(config, "audio", ["music-generation", "gpt-4o-mini-tts"])).toBe("beefapi::music-generation");
});
