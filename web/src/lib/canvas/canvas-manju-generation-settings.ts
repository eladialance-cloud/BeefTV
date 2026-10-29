import type { CanvasBatchRow, CanvasBatchTableData, CanvasGenerationSettings, CanvasManjuAssetKind } from "@/types/canvas";
import type { AiConfig } from "@/stores/use-config-store";

export type ResolvedCanvasGenerationSettings = Required<CanvasGenerationSettings>;
export type CanvasGenerationSettingsScope = {
    rowId?: string;
    kind?: CanvasManjuAssetKind;
};

export function resolveCanvasGenerationSettings(table: CanvasBatchTableData | undefined, row: CanvasBatchRow | undefined, fallback: AiConfig): ResolvedCanvasGenerationSettings {
    const kind = row?.manjuAsset?.kind;
    const layers: Array<CanvasGenerationSettings | undefined> = [table?.manjuGenerationDefaults, kind ? table?.manjuCategoryGeneration?.[kind] : undefined, row?.manjuGeneration];
    let result: CanvasGenerationSettings = {
        model: fallback.imageModel || fallback.model,
        size: fallback.size,
        quality: fallback.quality,
        count: Number(fallback.canvasImageCount || fallback.count || 1),
    };
    for (const layer of layers) {
        if (!layer) continue;
        if (layer.model?.trim()) result.model = layer.model.trim();
        if (layer.size?.trim()) result.size = layer.size.trim();
        if (layer.quality?.trim()) result.quality = layer.quality.trim();
        if (layer.count && layer.count > 0) result.count = Math.floor(layer.count);
    }
    return result as ResolvedCanvasGenerationSettings;
}

export function patchGenerationSettings(current: CanvasGenerationSettings | undefined, patch: Partial<CanvasGenerationSettings>): CanvasGenerationSettings {
    const next = { ...(current || {}), ...patch };
    if (!next.model) delete next.model;
    if (!next.size) delete next.size;
    if (!next.quality) delete next.quality;
    if (!next.count || next.count < 1) delete next.count;
    return next;
}

export function patchManjuTableGenerationSettings(
    table: CanvasBatchTableData,
    scope: CanvasGenerationSettingsScope,
    patch: Partial<CanvasGenerationSettings>,
): CanvasBatchTableData {
    if (scope.rowId) {
        return {
            ...table,
            rows: table.rows.map((row) =>
                row.id === scope.rowId
                    ? { ...row, manjuGeneration: patchGenerationSettings(row.manjuGeneration, patch) }
                    : row,
            ),
        };
    }
    if (scope.kind) {
        return {
            ...table,
            manjuCategoryGeneration: {
                ...table.manjuCategoryGeneration,
                [scope.kind]: patchGenerationSettings(table.manjuCategoryGeneration?.[scope.kind], patch),
            },
        };
    }
    return {
        ...table,
        manjuGenerationDefaults: patchGenerationSettings(table.manjuGenerationDefaults, patch),
    };
}

export function resolveBatchGenerationModels(
    table: CanvasBatchTableData,
    rows: CanvasBatchRow[],
    fallback: AiConfig,
) {
    return Array.from(new Set(
        rows
            .map((row) => resolveCanvasGenerationSettings(table, row, fallback).model)
            .filter((model): model is string => Boolean(model)),
    ));
}

export function generationSettingsLabel(settings: ResolvedCanvasGenerationSettings) {
    return [settings.model || "默认模型", settings.size || "自动比例", (settings.quality || "自动").toUpperCase(), `${settings.count} 张`].filter(Boolean).join(" · ");
}

export function generationSettingsScopeLabel(kind?: CanvasManjuAssetKind, row?: CanvasBatchRow) {
    if (row) return `${row.manjuAsset?.code || "当前行"} 单行设置`;
    if (kind) return `${kind === "character" ? "人物" : kind === "scene" ? "场景" : "道具"}分类设置`;
    return "表格统一设置";
}
