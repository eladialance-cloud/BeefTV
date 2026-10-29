import { modelOptionName, resolveModelChannel, type AiConfig, type ModelCapability, type ModelChannel } from "@/stores/use-config-store";

type ModelPriceOption = NonNullable<NonNullable<ModelChannel["modelProfiles"]>[number]["prices"]>[number];

export type ModelPriceDisplay = {
    configured: boolean;
    text: string;
    title: string;
};

export type ModelPriceSelection = ModelPriceDisplay & {
    billingMode: string;
    quantity: number;
    totalMicrocredits: number;
};

function positiveNumber(value: unknown, fallback = 1) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? number : fallback;
}

function selectorScore(selector: Record<string, string> | undefined, config: AiConfig) {
    if (!selector || !Object.keys(selector).length) return 0;
    let score = 0;
    for (const [rawKey, rawValue] of Object.entries(selector)) {
        const key = rawKey.trim().toLowerCase();
        const value = String(rawValue || "").trim().toLowerCase();
        if (!value) continue;
        const candidates = key === "quality" || key === "vquality"
            ? [config.quality, config.vquality, config.size]
            : key === "size"
              ? [config.size, config.quality]
              : ["videoseconds", "seconds", "duration"].includes(key)
                ? [config.videoSeconds]
                : [];
        if (!candidates.some((candidate) => String(candidate || "").trim().toLowerCase() === value)) continue;
        score += 1;
    }
    return score;
}

function selectPrice(prices: ModelPriceOption[], config: AiConfig) {
    let selected: ModelPriceOption | undefined;
    let bestScore = -1;
    for (const price of prices) {
        const score = selectorScore(price.selector, config);
        if (score > bestScore) {
            selected = price;
            bestScore = score;
        }
    }
    return selected;
}

function formatCredits(microcredits: number) {
    const value = Number(microcredits) / 1_000_000;
    if (!Number.isFinite(value)) return "0";
    return value.toLocaleString("zh-CN", { maximumFractionDigits: 6 });
}

function priceUnit(mode: string) {
    if (mode === "per_image") return "张";
    if (mode === "per_second") return "秒";
    if (mode === "per_1k_tokens") return "千 Token";
    if (mode === "per_megabyte") return "MB";
    return "次";
}

export function modelPriceDisplay(config: AiConfig, model: string, capability?: ModelCapability): ModelPriceDisplay {
    const selection = modelPriceSelection(config, model, capability);
    return { configured: selection.configured, text: selection.text, title: selection.title };
}

export function modelPriceSelection(config: AiConfig, model: string, capability?: ModelCapability): ModelPriceSelection {
    const profile = resolveModelChannel(config, model).modelProfiles?.find((item) => item.model === modelOptionName(model));
    const prices = (profile?.prices || []).filter((price) => Number.isFinite(price.unitPriceMicrocredits) && price.unitPriceMicrocredits > 0 && (!capability || price.billingMode !== ""));
    const selected = selectPrice(prices, config);
    if (!selected) {
        return {
            configured: false,
            text: "积分未配置",
            title: "后台尚未配置该模型的有效积分价格",
            billingMode: "",
            quantity: 0,
            totalMicrocredits: 0,
        };
    }
    const mode = selected.billingMode || "fixed_request";
    if (mode === "per_1k_tokens" || mode === "per_megabyte") {
        const text = `${formatCredits(selected.unitPriceMicrocredits)} 积分/${priceUnit(mode)}`;
        return {
            configured: true,
            text,
            title: text,
            billingMode: mode,
            quantity: 1,
            totalMicrocredits: selected.unitPriceMicrocredits,
        };
    }
    const quantity = mode === "per_image"
        ? positiveNumber(config.count)
        : mode === "per_second"
          ? positiveNumber(config.videoSeconds)
          : 1;
    const minimum = Math.max(0, Number(selected.minimumChargeMicrocredits || 0));
    const total = Math.max(selected.unitPriceMicrocredits * quantity, minimum);
    const quantityLabel = mode === "per_image"
        ? `${quantity} 张`
        : mode === "per_second"
          ? `${quantity} 秒`
          : "1 次";
    const text = `约 ${formatCredits(total)} 积分 / ${quantityLabel}`;
    const alternatives = prices
        .map((price) => {
            const selector = Object.entries(price.selector || {}).map(([key, value]) => `${key}=${value}`).join(", ");
            return `${selector || "默认"}：${formatCredits(price.unitPriceMicrocredits)} 积分/${priceUnit(price.billingMode)}`;
        })
        .join("\n");
    return {
        configured: true,
        text,
        title: alternatives || text,
        billingMode: mode,
        quantity,
        totalMicrocredits: total,
    };
}
