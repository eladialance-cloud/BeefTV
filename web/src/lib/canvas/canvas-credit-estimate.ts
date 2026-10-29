import { modelOptionName, resolveModelChannel, type AiConfig, type ModelCapability } from "@/stores/use-config-store";
import { modelCapabilityConfigFor, normalizeImageValue, resolveVideoResolutionValue } from "@/lib/model-capabilities";
import { modelPriceDisplay, modelPriceSelection } from "@/lib/model-price-display";
import type { PricingQuote, PricingQuoteRequest } from "@/services/api/pricing";

export type CanvasCreditEstimate = {
    configured: boolean;
    text: string;
    title: string;
    source: "server" | "local" | "none";
};

export function canvasVideoPricingSignature(config: AiConfig) {
    return canvasModelPricingSignature(config, "video", config.model || config.videoModel);
}

export function canvasVideoPricingRequest(config: AiConfig): PricingQuoteRequest | null {
    return canvasModelPricingRequest(config, "video", config.model || config.videoModel);
}

export function canvasModelPricingSignature(config: AiConfig, capability: ModelCapability, modelValue?: string) {
    return JSON.stringify(canvasModelPricingRequest(config, capability, modelValue));
}

export function canvasModelPricingRequest(config: AiConfig, capability: ModelCapability, modelValue?: string): PricingQuoteRequest | null {
    const selectedModel = modelValue || capabilityModelValue(config, capability);
    if (!selectedModel) return null;
    const channel = resolveModelChannel(config, selectedModel);
    const model = modelOptionName(selectedModel);
    const profile = channel.modelProfiles?.find((item) => item.model === model);
    if (!channel.id || !model) return null;

    const local = modelPriceSelection(config, selectedModel, capability);
    const billingMode = local.configured ? local.billingMode : "auto";
    const quantity = pricingQuantity(config, capability, selectedModel, billingMode, local.quantity);
    const selector = pricingSelector(config, capability, selectedModel);

    return {
        ...(profile?.logicalModelId?.trim() ? { logicalModelId: profile.logicalModelId.trim() } : {}),
        channelId: channel.id,
        ...(profile?.id?.trim() ? { channelModelId: profile.id.trim() } : {}),
        modelKey: model,
        capability,
        billingMode,
        ...(Object.keys(selector).length ? { selector } : {}),
        quantity,
    };
}

function capabilityModelValue(config: AiConfig, capability: ModelCapability) {
    if (capability === "image") return config.imageModel || config.model;
    if (capability === "video") return config.model || config.videoModel;
    if (capability === "text") return config.textModel || config.model;
    return config.audioModel || config.model;
}

function pricingQuantity(config: AiConfig, capability: ModelCapability, modelValue: string, billingMode: string, localQuantity: number) {
    if (billingMode === "per_second") {
        return capability === "video" ? videoSecondsForQuote(config, modelValue) : positiveQuantity(1);
    }
    if (billingMode === "per_image") {
        return positiveQuantity(config.count || localQuantity);
    }
    if (billingMode === "auto") {
        if (capability === "video") return videoSecondsForQuote(config, modelValue);
        if (capability === "image") return positiveQuantity(config.count);
    }
    return positiveQuantity(localQuantity);
}

function pricingSelector(config: AiConfig, capability: ModelCapability, modelValue: string): Record<string, string> {
    if (capability === "image") {
        const profile = modelCapabilityConfigFor(config, modelValue).image;
        const quality = profile
            ? normalizeImageValue(profile, {
                size: config.size,
                quality: config.quality,
                count: config.count,
                transparentBackground: config.transparentBackground,
            }).quality
            : config.quality;
        const normalized = normalizeVideoQuality(quality);
        return normalized ? { quality: normalized } : {};
    }
    if (capability === "video") {
        const videoProfile = modelCapabilityConfigFor(config, modelValue).video;
        const quality = normalizeVideoQuality(videoProfile ? resolveVideoResolutionValue(videoProfile, config.vquality) : config.vquality);
        return quality ? { quality } : {};
    }
    return {};
}

export function canvasModelCreditFallback(config: AiConfig, capability: ModelCapability, modelValue?: string): CanvasCreditEstimate {
    const selectedModel = modelValue || capabilityModelValue(config, capability);
    const display = modelPriceDisplay(config, selectedModel, capability);
    if (!display.configured) {
        return {
            configured: false,
            text: display.text,
            title: display.title,
            source: "none",
        };
    }
    return {
        configured: true,
        text: `参考 ${display.text}`,
        title: `${display.title}\n本地配置仅供参考，提交任务时以后端价格快照为准`,
        source: "local",
    };
}

export function canvasVideoCreditFallback(config: AiConfig): CanvasCreditEstimate {
    return canvasModelCreditFallback(config, "video", config.model || config.videoModel);
}

export function formatServerCreditEstimate(quote: PricingQuote): CanvasCreditEstimate {
    const credits = Number(quote.estimatedCredits);
    if (!Number.isFinite(credits) || credits <= 0) return missingCreditEstimate();
    const text = `预计 ${formatCredits(credits)} 积分`;
    return {
        configured: true,
        text,
        title: `${text}\n${quote.ruleName || quote.ruleKind} · ${quote.billingMode} · ${quote.quantity}`,
        source: "server",
    };
}

export function missingCreditEstimate(): CanvasCreditEstimate {
    return {
        configured: false,
        text: "积分未配置",
        title: "后台尚未配置当前模型、分辨率和时长对应的有效积分价格",
        source: "none",
    };
}

export function createLatestQuoteGate() {
    let revision = 0;
    return {
        begin() {
            revision += 1;
            return revision;
        },
        isCurrent(requestRevision: number) {
            return requestRevision === revision;
        },
        invalidate() {
            revision += 1;
        },
    };
}

function normalizeVideoQuality(value: string) {
    const normalized = String(value || "").trim().toLowerCase();
    if (!normalized || normalized === "auto" || normalized === "standard") return "";
    return /^\d+$/.test(normalized) ? `${normalized}p` : normalized;
}

function positiveQuantity(value: unknown) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? number : 1;
}

function videoSecondsForQuote(config: AiConfig, modelValue: string) {
    const configured = Number(config.videoSeconds);
    if (Number.isFinite(configured) && configured > 0) return configured;
    const durationDefault = modelCapabilityConfigFor(config, modelValue).video?.duration.default;
    return positiveQuantity(durationDefault);
}

function formatCredits(microcredits: number) {
    return (microcredits / 1_000_000).toLocaleString("zh-CN", { maximumFractionDigits: 6 });
}
