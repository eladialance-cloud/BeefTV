import { http } from "@/services/api/request";
import type { ModelCapability, ModelChannel } from "@/stores/use-config-store";
import type { ModelProtocol } from "@/lib/model-protocols";
import type { ModelCapabilityConfig } from "@/lib/model-capabilities";

/**
 * 托管模式下拉取公开模型目录。
 *
 * 后端只会下发可解释的展示读模型（公开名称、能力、协议、能力参数），
 * 真实 Base URL 与密钥永远不出后台。画布侧把每个系统渠道映射成一条
 * `scope: "system"` 的 `ModelChannel`，`baseUrl` 指向同源系统代理，
 * 生成请求由后端解析到真实上游。
 */

type PublicChannelModel = {
    id: string;
    modelKey: string;
    displayName: string;
    sortOrder: number;
    icon: string;
    capability: ModelCapability;
    protocol?: ModelProtocol;
    capabilityConfig?: ModelCapabilityConfig;
    available: boolean;
};

type PublicChannelCatalog = {
    id: string;
    name: string;
    displayName: string;
    sortOrder: number;
    models: PublicChannelModel[];
};

type PublicLogicalModel = {
    id: string;
    displayName?: string;
    icon?: string;
};

type ModelCatalogResponse = {
    source: "frontend" | "system";
    models: PublicLogicalModel[];
    channels: PublicChannelCatalog[];
};

export function getModelCatalog() {
    return http.get<ModelCatalogResponse>("/model-catalog");
}

/** 拉取并转换公开系统渠道；失败时返回空目录，由调用方决定是否阻断启动。 */
export async function loadSystemChannels(): Promise<ModelChannel[]> {
    const catalog = await getModelCatalog();
    return catalog.source === "system" ? mapSystemChannelCatalog(catalog.channels) : [];
}

export function systemChannelBaseUrl(channelId: string) {
    return `/api/ai/system/${encodeURIComponent(channelId)}`;
}

/**
 * 把公开目录里的系统渠道转换成画布的 `ModelChannel`。
 * 只有可用模型会进入 `models`，同时补齐 `modelProfiles` 让选择器能展示公开名称与图标。
 */
export function mapSystemChannelCatalog(channels: PublicChannelCatalog[] | undefined): ModelChannel[] {
    if (!channels?.length) return [];
    return channels
        .filter((channel) => channel.models.length > 0)
        .sort((a, b) => a.sortOrder - b.sortOrder || a.displayName.localeCompare(b.displayName))
        .map((channel) => {
            const available = channel.models.filter((model) => model.available).sort((a, b) => a.sortOrder - b.sortOrder);
            return {
                id: channel.id,
                name: channel.displayName || channel.name,
                sortOrder: channel.sortOrder,
                baseUrl: systemChannelBaseUrl(channel.id),
                apiKey: "system",
                apiFormat: "openai" as const,
                scope: "system" as const,
                enabled: true,
                models: available.map((model) => model.modelKey),
                modelProfiles: available.map((model) => ({
                    id: model.id,
                    model: model.modelKey,
                    displayName: model.displayName,
                    icon: model.icon,
                    capability: model.capability,
                    protocol: model.protocol,
                    capabilityConfig: model.capabilityConfig,
                })),
            };
        })
        .filter((channel) => channel.models.length > 0);
}

export type { ModelCatalogResponse, PublicChannelCatalog, PublicChannelModel };
