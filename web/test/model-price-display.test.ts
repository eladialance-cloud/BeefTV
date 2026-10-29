import { describe, expect, test } from "bun:test";

import { modelPriceDisplay } from "@/lib/model-price-display";
import type { AiConfig } from "@/stores/use-config-store";

function config(input: { quality?: string; count?: string; videoSeconds?: string; prices?: unknown[] }): AiConfig {
    return {
        channelMode: "remote",
        baseUrl: "",
        apiKey: "",
        apiFormat: "openai",
        channels: [{
            id: "channel-1",
            name: "渠道",
            baseUrl: "",
            apiKey: "",
            apiFormat: "openai",
            models: ["image-model", "video-model"],
            modelProfiles: [
                { id: "model-1", model: "image-model", capability: "image", prices: input.prices as never },
                {
                    id: "model-2",
                    model: "video-model",
                    capability: "video",
                    prices: [{ billingMode: "per_second", unitPriceMicrocredits: 500_000, minimumChargeMicrocredits: 3_000_000 }],
                },
            ],
        }],
        runningHub: { enabled: false } as never,
        model: "channel-1::image-model",
        imageModel: "channel-1::image-model",
        videoModel: "channel-1::video-model",
        textModel: "",
        audioModel: "",
        assistantModel: "",
        audioVoice: "",
        audioFormat: "",
        audioSpeed: "",
        audioPitch: "",
        audioVolume: "",
        audioInstructions: "",
        videoSeconds: input.videoSeconds || "5",
        vquality: "",
        videoGenerateAudio: "",
        videoWatermark: "",
        videoArkPrivateAssetUpload: "",
        systemPrompt: "",
        models: ["channel-1::image-model", "channel-1::video-model"],
        imageModels: ["channel-1::image-model"],
        videoModels: ["channel-1::video-model"],
        textModels: [],
        audioModels: [],
        quality: input.quality || "2k",
        size: "1:1",
        transparentBackground: "",
        count: input.count || "1",
    } as AiConfig;
}

describe("modelPriceDisplay", () => {
    test("uses the matching image quality tier and quantity", () => {
        const display = modelPriceDisplay(config({
            quality: "2k",
            count: "2",
            prices: [
                { billingMode: "per_image", unitPriceMicrocredits: 250_000, minimumChargeMicrocredits: 500_000 },
                { billingMode: "per_image", selector: { quality: "2k" }, unitPriceMicrocredits: 1_000_000 },
            ],
        }), "channel-1::image-model", "image");
        expect(display.configured).toBe(true);
        expect(display.text).toBe("约 2 积分 / 2 张");
    });

    test("applies minimum charge to video seconds", () => {
        const display = modelPriceDisplay(config({ videoSeconds: "5" }), "channel-1::video-model", "video");
        expect(display.text).toBe("约 3 积分 / 5 秒");
    });

    test("reports missing prices instead of displaying zero", () => {
        const display = modelPriceDisplay(config({ prices: [] }), "channel-1::image-model", "image");
        expect(display.configured).toBe(false);
        expect(display.text).toBe("积分未配置");
    });
});

test("canvas model picker is a flat list with price labels", async () => {
    const source = await Bun.file(new URL("../src/components/model-picker.tsx", import.meta.url)).text();
    expect(source).toContain("flatModels");
    expect(source).toContain("modelPriceDisplay");
    expect(source).toContain("useCanvasModelCreditEstimate");
    expect(source).toContain("ModelPickerServerPrice");
    expect(source).not.toContain("canvas-model-picker-brands");
    expect(source).not.toContain("canvas-model-picker-two-pane");
});
