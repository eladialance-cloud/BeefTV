import { describe, expect, test } from "bun:test";

import {
    canvasVideoCreditFallback,
    canvasModelPricingRequest,
    canvasVideoPricingRequest,
    canvasVideoPricingSignature,
    createLatestQuoteGate,
    formatServerCreditEstimate,
    missingCreditEstimate,
} from "@/lib/canvas/canvas-credit-estimate";
import { defaultModelCapabilityConfig } from "@/lib/model-capabilities";
import { mapSystemChannelCatalog } from "@/services/api/model-catalog";
import type { AiConfig } from "@/stores/use-config-store";

function config(input: { prices?: unknown[]; vquality?: string; videoSeconds?: string; quality?: string; count?: string } = {}): AiConfig {
    return {
        channelMode: "remote",
        baseUrl: "",
        apiKey: "",
        apiFormat: "openai",
        channels: [{
            id: "channel-1",
            name: "系统渠道",
            baseUrl: "/api/ai/system/channel-1",
            apiKey: "system",
            apiFormat: "openai",
            scope: "system",
            models: ["video-model"],
            modelProfiles: [{
                id: "channel-model-1",
                model: "video-model",
                capability: "video",
                logicalModelId: "logical-video",
                prices: input.prices as never,
            }],
        }],
        runningHub: { enabled: false } as never,
        model: "channel-1::video-model",
        imageModel: "",
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
        videoSeconds: input.videoSeconds || "8",
        vquality: input.vquality || "1080",
        videoGenerateAudio: "",
        videoWatermark: "",
        videoArkPrivateAssetUpload: "",
        systemPrompt: "",
        models: ["channel-1::video-model"],
        imageModels: [],
        videoModels: ["channel-1::video-model"],
        textModels: [],
        audioModels: [],
        quality: "",
        size: "9:16",
        transparentBackground: "",
        count: "1",
        canvasImageCount: "1",
    } as AiConfig;
}

function configWithImageModel(base: AiConfig, quality: string, count: string): AiConfig {
    const channel = base.channels[0]!;
    const capabilityConfig = defaultModelCapabilityConfig(undefined, "image-model");
    capabilityConfig.image = {
        ...capabilityConfig.image!,
        quality: { supported: true, values: [quality], default: quality },
    };
    return {
        ...base,
        channels: [{
            ...channel,
            models: [...channel.models, "image-model"],
            modelProfiles: [
                ...(channel.modelProfiles || []),
                {
                    id: "channel-model-image",
                    model: "image-model",
                    capability: "image",
                    capabilityConfig,
                },
            ],
        }],
        imageModel: "channel-1::image-model",
        quality,
        count,
    };
}

describe("canvas video credit estimate", () => {
    test("quotes the current resolution and duration through the selected server channel model", () => {
        const request = canvasVideoPricingRequest(config({
            prices: [{ billingMode: "per_second", selector: { quality: "1080p" }, unitPriceMicrocredits: 500_000 }],
        }));
        expect(request).toEqual({
            logicalModelId: "logical-video",
            channelId: "channel-1",
            channelModelId: "channel-model-1",
            modelKey: "video-model",
            capability: "video",
            billingMode: "per_second",
            selector: { quality: "1080p" },
            quantity: 8,
        });
    });

    test("sends the model key when an old Canvas profile has no channel-model id", () => {
        const oldConfig = config();
        oldConfig.channels[0]!.modelProfiles![0]!.id = undefined;
        const request = canvasVideoPricingRequest(oldConfig);
        expect(request?.channelId).toBe("channel-1");
        expect(request?.channelModelId).toBeUndefined();
        expect(request?.modelKey).toBe("video-model");
    });

    test("support-model pricing requests support non-video Canvas model rows", () => {
        const imageConfig = configWithImageModel(config(), "2k", "2");
        expect(canvasModelPricingRequest(imageConfig, "image", "channel-1::image-model")).toMatchObject({
            channelId: "channel-1",
            channelModelId: "channel-model-image",
            modelKey: "image-model",
            capability: "image",
            billingMode: "auto",
            selector: { quality: "2k" },
            quantity: 2,
        });
    });

    test("uses server auto billing when no local price rows were published", () => {
        const request = canvasVideoPricingRequest(config({ vquality: "720", videoSeconds: "12" }));
        expect(request?.billingMode).toBe("auto");
        expect(request?.selector).toEqual({ quality: "720p" });
        expect(request?.quantity).toBe(12);
    });

    test("keeps an explicit local fixed-request price mode", () => {
        const request = canvasVideoPricingRequest(config({
            prices: [{ billingMode: "fixed_request", selector: { quality: "1080p" }, unitPriceMicrocredits: 2_000_000 }],
        }));
        expect(request?.billingMode).toBe("fixed_request");
        expect(request?.quantity).toBe(1);
    });

    test("uses the model duration default when Canvas selects automatic duration", () => {
        const request = canvasVideoPricingRequest(config({ videoSeconds: "-1" }));
        expect(request?.quantity).toBeGreaterThan(1);
    });

    test("uses the model resolution default when Canvas selects automatic resolution", () => {
        const request = canvasVideoPricingRequest(config({ vquality: "auto" }));
        expect(request?.selector).toEqual({ quality: "720p" });
    });

    test("formats the authoritative fixed-point quote without rounding a fractional charge", () => {
        const estimate = formatServerCreditEstimate({
            ruleId: "video-1080",
            ruleKind: "channel_tier",
            billingMode: "per_second",
            quantity: 8,
            unitPriceMicrocredits: 287_500,
            minimumChargeCredits: 0,
            estimatedCredits: 2_300_000,
            effectiveFrom: "2026-10-01T00:00:00Z",
        });
        expect(estimate.configured).toBe(true);
        expect(estimate.text).toBe("预计 2.3 积分");
    });

    test("uses a configured local price only as an explicitly marked reference fallback", () => {
        const fallback = canvasVideoCreditFallback(config({
            prices: [{ billingMode: "per_second", selector: { quality: "1080p" }, unitPriceMicrocredits: 500_000 }],
        }));
        expect(fallback.configured).toBe(true);
        expect(fallback.text).toBe("参考 约 4 积分 / 8 秒");
        expect(fallback.source).toBe("local");
    });

    test("reports missing pricing instead of showing zero", () => {
        expect(canvasVideoCreditFallback(config()).text).toBe("积分未配置");
        expect(missingCreditEstimate().text).toBe("积分未配置");
    });

    test("drops a late quote after a newer parameter set starts", () => {
        const gate = createLatestQuoteGate();
        const oldRequest = gate.begin();
        const currentRequest = gate.begin();
        expect(gate.isCurrent(oldRequest)).toBe(false);
        expect(gate.isCurrent(currentRequest)).toBe(true);
        gate.invalidate();
        expect(gate.isCurrent(currentRequest)).toBe(false);
    });

    test("keeps the quote signature stable when Canvas recreates the config object", () => {
        const first = config({ prices: [{ billingMode: "per_second", selector: { quality: "1080p" }, unitPriceMicrocredits: 500_000 }] });
        const second = {
            ...first,
            channels: first.channels.map((channel) => ({ ...channel, modelProfiles: channel.modelProfiles?.map((profile) => ({ ...profile })) })),
        };
        expect(canvasVideoPricingSignature(second)).toBe(canvasVideoPricingSignature(first));
    });

    test("keeps the public channel-model id so system quotes can resolve channel tiers", () => {
        const channels = mapSystemChannelCatalog([{
            id: "channel-1",
            name: "系统渠道",
            displayName: "系统渠道",
            sortOrder: 1,
            models: [{
                id: "channel-model-1",
                modelKey: "video-model",
                displayName: "视频模型",
                sortOrder: 1,
                icon: "",
                capability: "video",
                available: true,
            }],
        }]);
        expect(channels[0]?.modelProfiles?.[0]?.id).toBe("channel-model-1");
    });
});
