import { nanoid } from "nanoid";

import {
    applyGeneratedMediaResultMetadata,
    audioMetadata,
    imageMetadata,
} from "@/lib/canvas/canvas-generation-task-sync";
import {
    buildGenerationConfig,
    generationTaskMetadata,
} from "@/lib/canvas/canvas-project-generation";
import type { WeddingGenerationAsset } from "@/lib/wedding/generation-runtime";
import type { WeddingProductionState } from "@/lib/wedding/types";
import { consumeGenerationTaskNode } from "@/services/project-asset-sync";
import { runBackendGenerationTask, type BackendGenerationResult } from "@/services/api/generation-task";
import type { GenerationTask } from "@/services/api/task-center";
import { uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import { uploadImage, type UploadedImage } from "@/services/image-storage";
import { persistCanvasDocument } from "@/services/local-workspace-repository";
import { useAssetStore } from "@/stores/use-asset-store";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";
import type { AiConfig } from "@/stores/use-config-store";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

export type WeddingTitleCardKind = "intro" | "outro";
export type WeddingTitleCardDocument = {
    kind: WeddingTitleCardKind;
    text: string;
    subtitle: string;
    width: number;
    height: number;
};

type WeddingPackagingProjectSnapshot = Pick<CanvasProject, "nodes" | "weddingProduction">;
type WeddingPackagingPersistPatch = {
    nodes: CanvasNodeData[];
    weddingProduction: WeddingProductionState;
};

export type WeddingPackagingGenerationRuntime = {
    projectId: string;
    production: WeddingProductionState;
    config: AiConfig;
    signal?: AbortSignal;
    onTaskUpdate?: (kind: "bgm", task: GenerationTask) => void;
    dependencies?: WeddingPackagingGenerationDependencies;
};

export type WeddingPackagingGenerationDependencies = {
    getProject?: (projectId: string) => WeddingPackagingProjectSnapshot | undefined;
    persist?: (projectId: string, patch: WeddingPackagingPersistPatch) => Promise<void>;
    uploadImage?: (dataUrl: string) => Promise<UploadedImage>;
    uploadAudio?: (file: File) => Promise<UploadedFile>;
    runTask?: (options: Parameters<typeof runBackendGenerationTask>[0]) => Promise<BackendGenerationResult>;
    consumeTask?: (
        task: GenerationTask,
        nodeId: string,
        outputIndex: number,
        consumer: (input: {
            task: GenerationTask;
            output: NonNullable<GenerationTask["outputs"]>[number];
            effectKey: string;
        }) => Promise<void>,
    ) => Promise<GenerationTask>;
    getAsset?: (assetId: string) => WeddingGenerationAsset | undefined;
};

export function buildWeddingTitleCardDocument(
    production: WeddingProductionState,
    kind: WeddingTitleCardKind,
): WeddingTitleCardDocument {
    const vertical = production.source.ratio === "9:16";
    return {
        kind,
        text: kind === "intro" ? production.packaging.intro.text : production.packaging.outro.text,
        subtitle: `${production.source.maleName} × ${production.source.femaleName}`,
        width: vertical ? 720 : 1280,
        height: vertical ? 1280 : 720,
    };
}

export function weddingBgmPrompt(production: WeddingProductionState) {
    const dynastyNames = production.dynasties.map((dynasty) => dynasty.name).join("、");
    return `Create an original instrumental BGM for a Chinese cross-time wedding film.
Project: 跨时空婚礼.
Theme: ${production.packaging.intro.text} to ${production.packaging.outro.text}.
Dynasty emotional arc: ${dynastyNames}.
Style: cinematic Chinese strings, guzheng, xiao, soft choir, warm modern piano, romantic and timeless.
No vocals, no lyrics, no existing melody, no copyrighted song reference.
Target duration: ${production.dynasties.length * 8 + production.packaging.intro.durationSeconds + production.packaging.outro.durationSeconds} seconds.
Visual format context: ${production.source.ratio}, ${production.source.resolution}.`;
}

export async function generateWeddingTitleCards(runtime: WeddingPackagingGenerationRuntime) {
    const dependencies = dependenciesWithDefaults(runtime.dependencies);
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const production = project.weddingProduction;
    const documents = [
        buildWeddingTitleCardDocument(production, "intro"),
        buildWeddingTitleCardDocument(production, "outro"),
    ];
    const uploaded = await Promise.all(documents.map((document) => dependencies.uploadImage(renderTitleCard(document))));
    const maxX = Math.max(0, ...project.nodes.map((node) => node.position.x + node.width));
    const now = new Date().toISOString();
    const nodes = documents.map((document, index): CanvasNodeData => ({
        id: `wedding-title-${document.kind}-${nanoid(8)}`,
        type: CanvasNodeType.Image,
        title: document.kind === "intro" ? "跨时空婚礼 · 片头标题卡" : "跨时空婚礼 · 片尾标题卡",
        position: { x: maxX + 80, y: index * 260 },
        width: document.width,
        height: document.height,
        createdAt: now,
        updatedAt: now,
        metadata: {
            ...imageMetadata(uploaded[index]!),
            nodeRole: "result",
            resultOrigin: "generated",
            workflowKind: "final",
        },
    }));
    const nextProduction = structuredClone(production);
    nextProduction.packaging.titleCardNodeIds = nodes.map((node) => node.id);
    nextProduction.packaging.status = "ready";
    nextProduction.packaging.approvedAt = undefined;
    nextProduction.stages.packaging = { status: "ready", updatedAt: now };
    nextProduction.stages.delivery = { ...nextProduction.stages.delivery, status: "stale" };
    await dependencies.persist(runtime.projectId, {
        nodes: [...project.nodes, ...nodes],
        weddingProduction: nextProduction,
    });
    return {
        introNodeId: nodes[0]!.id,
        outroNodeId: nodes[1]!.id,
    };
}

export async function generateWeddingBgm(runtime: WeddingPackagingGenerationRuntime) {
    const dependencies = dependenciesWithDefaults(runtime.dependencies);
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const nodeId = `wedding-bgm-${nanoid(8)}`;
    let completedTask: GenerationTask | undefined;
    await dependencies.runTask({
        projectId: runtime.projectId,
        mode: "audio",
        prompt: weddingBgmPrompt(project.weddingProduction),
        config: buildGenerationConfig(runtime.config, undefined, "audio"),
        signal: runtime.signal,
        metadata: {
            source: "wedding-workbench",
            stage: "packaging-bgm",
        },
        onTaskUpdate: (task) => {
            if (task.status === "succeeded") completedTask = task;
            runtime.onTaskUpdate?.("bgm", task);
        },
    });
    if (!completedTask) throw new Error("BGM 任务缺少成功终态");
    let persisted: { nodeId: string; assetId: string } | undefined;
    await dependencies.consumeTask(completedTask, nodeId, 0, async ({ task, output, effectKey }) => {
        const assetId = output.materializedAssetId;
        if (!assetId) throw new Error("BGM 输出尚未物化");
        const asset = dependencies.getAsset(assetId);
        const audioUrl = asset?.data?.url || asset?.data?.dataUrl || "";
        const storageKey = asset?.data?.storageKey;
        if (!asset || asset.kind !== "audio" || !audioUrl || !storageKey) throw new Error("BGM 音频素材不可用");
        const latest = dependencies.getProject(runtime.projectId);
        if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
        const now = new Date().toISOString();
        const node: CanvasNodeData = {
            id: nodeId,
            type: CanvasNodeType.Audio,
            title: "跨时空婚礼 · 主题 BGM",
            position: { x: 80, y: 760 },
            width: 340,
            height: 220,
            createdAt: now,
            updatedAt: now,
            metadata: applyGeneratedMediaResultMetadata(
                {
                    id: nodeId,
                    type: CanvasNodeType.Audio,
                    title: "跨时空婚礼 · 主题 BGM",
                    position: { x: 80, y: 760 },
                    width: 340,
                    height: 220,
                    metadata: {},
                },
                audioMetadata({
                    url: audioUrl,
                    storageKey,
                    durationMs: asset.data?.durationMs,
                    bytes: asset.data?.bytes || 0,
                    mimeType: asset.data?.mimeType || "audio/mpeg",
                }),
                {
                    prompt: task.prompt,
                    ...generationTaskMetadata(task),
                },
            ),
        };
        node.metadata = {
            ...node.metadata,
            assetId,
            generationEffectKeys: [effectKey],
        };
        const nextProduction = structuredClone(latest.weddingProduction);
        nextProduction.packaging.bgmNodeId = nodeId;
        nextProduction.packaging.bgmMode = "generated";
        nextProduction.packaging.status = "ready";
        nextProduction.packaging.approvedAt = undefined;
        nextProduction.stages.packaging = { status: "ready", updatedAt: now };
        nextProduction.stages.delivery = { ...nextProduction.stages.delivery, status: "stale" };
        await dependencies.persist(runtime.projectId, {
            nodes: [...latest.nodes, node],
            weddingProduction: nextProduction,
        });
        persisted = { nodeId, assetId };
    });
    if (!persisted) throw new Error("BGM 结果没有写入画布");
    return persisted;
}

export async function uploadWeddingBgm(runtime: WeddingPackagingGenerationRuntime, file: File) {
    const dependencies = dependenciesWithDefaults(runtime.dependencies);
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const uploaded = await dependencies.uploadAudio(file);
    const nodeId = `wedding-bgm-upload-${nanoid(8)}`;
    const now = new Date().toISOString();
    const node: CanvasNodeData = {
        id: nodeId,
        type: CanvasNodeType.Audio,
        title: file.name || "跨时空婚礼 · 上传 BGM",
        position: { x: 80, y: 760 },
        width: 340,
        height: 220,
        createdAt: now,
        updatedAt: now,
        metadata: audioMetadata(uploaded),
    };
    const nextProduction = structuredClone(project.weddingProduction);
    nextProduction.packaging.bgmNodeId = nodeId;
    nextProduction.packaging.bgmMode = "uploaded";
    nextProduction.packaging.status = "ready";
    nextProduction.packaging.approvedAt = undefined;
    nextProduction.stages.packaging = { status: "ready", updatedAt: now };
    nextProduction.stages.delivery = { ...nextProduction.stages.delivery, status: "stale" };
    await dependencies.persist(runtime.projectId, {
        nodes: [...project.nodes, node],
        weddingProduction: nextProduction,
    });
    return { nodeId, assetId: uploaded.storageKey };
}

function renderTitleCard(card: WeddingTitleCardDocument) {
    if (typeof globalThis.document === "undefined") throw new Error("当前环境不支持标题卡绘制");
    const canvas = globalThis.document.createElement("canvas");
    canvas.width = card.width;
    canvas.height = card.height;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("浏览器无法创建标题卡画布");
    const gradient = context.createLinearGradient(0, 0, card.width, card.height);
    gradient.addColorStop(0, "#080b0d");
    gradient.addColorStop(0.55, "#172326");
    gradient.addColorStop(1, "#2c2419");
    context.fillStyle = gradient;
    context.fillRect(0, 0, card.width, card.height);
    context.strokeStyle = "rgba(216,170,97,.45)";
    context.lineWidth = Math.max(2, Math.round(card.width / 640));
    context.strokeRect(36, 36, card.width - 72, card.height - 72);
    context.textAlign = "center";
    context.fillStyle = "#f1e6d3";
    context.font = `600 ${Math.round(card.width / 14)}px "Microsoft YaHei", "PingFang SC", sans-serif`;
    context.fillText(card.text, card.width / 2, card.height / 2, card.width - 140);
    context.fillStyle = "rgba(216,170,97,.9)";
    context.font = `400 ${Math.round(card.width / 38)}px "Microsoft YaHei", "PingFang SC", sans-serif`;
    context.fillText(card.subtitle, card.width / 2, card.height / 2 + card.height / 8, card.width - 160);
    return canvas.toDataURL("image/png");
}

function dependenciesWithDefaults(
    overrides: WeddingPackagingGenerationDependencies = {},
): Required<WeddingPackagingGenerationDependencies> {
    return {
        getProject: overrides.getProject || ((projectId) => useCanvasStore.getState().projects.find((project) => project.id === projectId)),
        persist: overrides.persist || ((projectId, patch) => persistCanvasDocument(projectId, patch)),
        uploadImage: overrides.uploadImage || ((dataUrl) => uploadImage(dataUrl)),
        uploadAudio: overrides.uploadAudio || ((file) => uploadMediaFile(file, "audio")),
        runTask: overrides.runTask || runBackendGenerationTask,
        consumeTask: overrides.consumeTask || consumeGenerationTaskNode,
        getAsset: overrides.getAsset || ((assetId) => {
            const asset = useAssetStore.getState().assets.find((item) => item.id === assetId);
            if (!asset) return undefined;
            if (asset.kind === "image" || asset.kind === "audio") {
                return { id: asset.id, kind: asset.kind, coverUrl: asset.coverUrl, data: asset.data };
            }
            return { id: asset.id, kind: asset.kind, coverUrl: asset.coverUrl };
        }),
    };
}
