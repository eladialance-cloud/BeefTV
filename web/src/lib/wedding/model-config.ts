import type {
    WeddingModelSelection,
    WeddingProductionState,
} from "@/lib/wedding/types";
import {
    modelDisplayName,
    modelOptionName,
    selectableModelsByCapability,
    type AiConfig,
    type ModelCapability,
} from "@/stores/use-config-store";

export function weddingModelSelection(
    production: WeddingProductionState,
    config: AiConfig,
): WeddingModelSelection {
    return {
        imageModel: production.models?.imageModel || config.imageModel || config.model,
        videoModel: production.models?.videoModel || config.videoModel || config.model,
        textModel: production.models?.textModel || config.textModel || config.model,
        audioModel: production.models?.audioModel || config.audioModel || config.model,
    };
}

export function weddingConfigWithModels(
    config: AiConfig,
    production: WeddingProductionState,
): AiConfig {
    const models = weddingModelSelection(production, config);
    return {
        ...config,
        imageModel: models.imageModel,
        videoModel: models.videoModel,
        textModel: models.textModel,
        audioModel: models.audioModel,
    };
}

export function recommendedWeddingModel(
    config: AiConfig,
    capability: ModelCapability,
    patterns: string[],
) {
    const options = selectableModelsByCapability(config, capability);
    for (const pattern of patterns) {
        const normalizedPattern = pattern.toLowerCase();
        const match = options.find((model) => {
            const id = `${model} ${modelOptionName(model)} ${modelDisplayName(config, model)}`.toLowerCase();
            return id.includes(normalizedPattern);
        });
        if (match) return match;
    }
    return "";
}

export function weddingModelReady(
    selection: Pick<WeddingModelSelection, "imageModel" | "videoModel">,
) {
    return Boolean(selection.imageModel && selection.videoModel);
}
