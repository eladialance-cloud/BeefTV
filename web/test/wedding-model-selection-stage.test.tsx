import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { WeddingModelSelectionStage } from "@/pages/wedding/model-selection-stage";
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

test("model selection page renders four project model slots and recommendation table", () => {
    const html = renderToStaticMarkup(
        <WeddingModelSelectionStage
            production={production}
            config={config}
            saving={false}
            feedback=""
            onSave={async () => undefined}
            onBack={() => undefined}
        />,
    );

    expect(html).toContain("图片生成模型");
    expect(html).toContain("视频生成模型");
    expect(html).toContain("视觉质检模型");
    expect(html).toContain("BGM 音频模型");
    expect(html).toContain("推荐模型表");
    expect(html).toContain("Seedream 5.0");
    expect(html).toContain("Seedance 2.5");
    expect(html).toContain("保存模型并返回");
});
