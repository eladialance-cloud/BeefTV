import {
    applyGeneratedMediaResultMetadata,
    imageMetadata,
} from "@/lib/canvas/canvas-generation-task-sync";
import {
    buildGenerationConfig,
    generationTaskMetadata,
} from "@/lib/canvas/canvas-project-generation";
import { creationTemplateGenerationMetadata } from "@/lib/canvas/creation-template-workflow";
import type {
    WeddingGenerationAsset,
    WeddingGenerationSummary,
} from "@/lib/wedding/generation-runtime";
import {
    failWeddingSceneGeneration,
    markWeddingSceneGenerating,
    recordWeddingSceneVersion,
    weddingSceneJobs,
    type WeddingSceneJob,
} from "@/lib/wedding/scene-runtime";
import type {
    WeddingDynastyState,
    WeddingPortraitGender,
    WeddingProductionState,
} from "@/lib/wedding/types";
import { persistCanvasDocument } from "@/services/local-workspace-repository";
import { consumeGenerationTaskNode } from "@/services/project-asset-sync";
import { runBackendGenerationTask, type BackendGenerationResult } from "@/services/api/generation-task";
import type { GenerationTask } from "@/services/api/task-center";
import { useAssetStore } from "@/stores/use-asset-store";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";
import type { AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import type { CanvasNodeData } from "@/types/canvas";

type WeddingSceneProjectSnapshot = Pick<CanvasProject, "nodes" | "weddingProduction">;
type WeddingScenePersistPatch = {
    nodes: CanvasNodeData[];
    weddingProduction: WeddingProductionState;
};

export type WeddingSceneGenerationRuntime = {
    projectId: string;
    production: WeddingProductionState;
    config: AiConfig;
    signal?: AbortSignal;
    now?: () => string;
    onTaskUpdate?: (job: WeddingSceneJob, task: GenerationTask) => void;
    onJobComplete?: (job: WeddingSceneJob) => void;
};

export type WeddingSceneGenerationOptions = {
    force?: boolean;
    nodeIds?: string[];
    dependencies?: WeddingSceneGenerationDependencies;
};

export type WeddingSceneGenerationDependencies = {
    getProject?: (projectId: string) => WeddingSceneProjectSnapshot | undefined;
    persist?: (projectId: string, patch: WeddingScenePersistPatch) => Promise<void>;
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

export async function generateWeddingScenes(
    runtime: WeddingSceneGenerationRuntime,
    dynastyId?: string,
    options: WeddingSceneGenerationOptions = {},
): Promise<WeddingGenerationSummary> {
    const dependencies = dependenciesWithDefaults(options.dependencies);
    const project = dependencies.getProject!(runtime.projectId);
    const production = project?.weddingProduction || runtime.production;
    const selectedNodeIds = new Set(options.nodeIds || []);
    const jobs = weddingSceneJobs(production, dynastyId).filter((job) => {
        if (selectedNodeIds.size && !selectedNodeIds.has(job.nodeId)) return false;
        if (options.force) return true;
        return weddingSceneNeedsGeneration(project?.nodes || [], production, job);
    });
    if (!jobs.length) return { attempted: 0, succeeded: 0, failed: 0, errors: [] };

    let commitTail = Promise.resolve();
    const enqueueCommit = (operation: () => Promise<void>) => {
        const running = commitTail.then(operation, operation);
        commitTail = running.catch(() => undefined);
        return running;
    };
    const now = runtime.now || (() => new Date().toISOString());
    const errors: WeddingGenerationSummary["errors"] = [];
    let succeeded = 0;
    let cursor = 0;

    const worker = async () => {
        for (;;) {
            const index = cursor;
            cursor += 1;
            const job = jobs[index];
            if (!job) return;
            try {
                await generateWeddingSceneJob(runtime, job, { dependencies, enqueueCommit, now });
                succeeded += 1;
                runtime.onJobComplete?.(job);
            } catch (error) {
                const message = error instanceof Error ? error.message : "场景首帧生成失败";
                errors.push({ nodeId: job.nodeId, dynastyId: job.dynastyId, message });
                await enqueueCommit(async () => {
                    const latest = dependencies.getProject!(runtime.projectId);
                    if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                    const target = latest.nodes.find((node) => node.id === job.nodeId);
                    const failedProduction = failWeddingSceneGeneration(
                        latest.weddingProduction,
                        job,
                        message,
                        now(),
                    );
                    await dependencies.persist!(runtime.projectId, {
                        nodes: target
                            ? latest.nodes.map((node) => node.id === job.nodeId
                                ? {
                                    ...node,
                                    metadata: {
                                        ...node.metadata,
                                        status: "error" as const,
                                        errorDetails: message,
                                        taskStatus: "failed" as const,
                                    },
                                }
                                : node)
                            : latest.nodes,
                        weddingProduction: failedProduction,
                    });
                });
            }
        }
    };

    await Promise.all(Array.from({ length: Math.min(2, jobs.length) }, () => worker()));
    await commitTail;
    return { attempted: jobs.length, succeeded, failed: errors.length, errors };
}

async function generateWeddingSceneJob(
    runtime: WeddingSceneGenerationRuntime,
    job: WeddingSceneJob,
    context: {
        dependencies: Required<WeddingSceneGenerationDependencies>;
        enqueueCommit: (operation: () => Promise<void>) => Promise<void>;
        now: () => string;
    },
) {
    const { dependencies } = context;
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const dynasty = project.weddingProduction.dynasties.find((item) => item.id === job.dynastyId);
    const target = project.nodes.find((node) => node.id === job.nodeId);
    if (!dynasty) throw new Error("朝代状态不存在");
    if (!target) throw new Error("场景首帧节点不存在");
    if (!dynasty.portraitApproval?.male || !dynasty.portraitApproval?.female) {
        throw new Error("请先确认男女定妆照");
    }
    const referenceImages = [
        portraitReference(dynasty, "male", dependencies.getAsset),
        portraitReference(dynasty, "female", dependencies.getAsset),
    ];
    const taskCommits: Promise<void>[] = [];
    let initialTaskPersisted = false;
    let completedTask: GenerationTask | undefined;
    const config = {
        ...buildGenerationConfig(runtime.config, target, "image"),
        count: "1",
    };

    await dependencies.runTask({
        projectId: runtime.projectId,
        mode: "image",
        prompt: target.metadata?.prompt?.trim() || target.title,
        config,
        referenceImages,
        signal: runtime.signal,
        metadata: {
            source: "wedding-workbench",
            stage: "scenes",
            dynastyId: job.dynastyId,
            ...creationTemplateGenerationMetadata(target.metadata),
        },
        onTaskUpdate: (task) => {
            if (task.status === "succeeded") completedTask = task;
            runtime.onTaskUpdate?.(job, task);
            if (!initialTaskPersisted && task.status === "queued") {
                initialTaskPersisted = true;
                taskCommits.push(context.enqueueCommit(async () => {
                    const latest = dependencies.getProject(runtime.projectId);
                    if (!latest?.weddingProduction) return;
                    const latestTarget = latest.nodes.find((node) => node.id === job.nodeId);
                    if (!latestTarget) return;
                    await dependencies.persist(runtime.projectId, {
                        nodes: latest.nodes.map((node) => node.id === job.nodeId
                            ? {
                                ...node,
                                metadata: {
                                    ...node.metadata,
                                    status: "loading" as const,
                                    errorDetails: undefined,
                                    ...generationTaskMetadata(task),
                                },
                            }
                            : node),
                        weddingProduction: markWeddingSceneGenerating(
                            latest.weddingProduction,
                            job,
                            context.now(),
                        ),
                    });
                }));
            }
        },
    });
    await Promise.all(taskCommits);
    if (!completedTask) throw new Error("生成任务缺少成功终态");

    await dependencies.consumeTask(
        completedTask,
        job.nodeId,
        0,
        async ({ task, output, effectKey }) => {
            const assetId = output.materializedAssetId;
            if (!assetId) throw new Error("生成任务输出尚未物化");
            const asset = dependencies.getAsset(assetId);
            if (!asset || asset.kind !== "image") throw new Error("生成场景素材不可用");
            const previewUrl = asset.data?.dataUrl || asset.data?.url || asset.coverUrl || "";
            const storageKey = asset.data?.storageKey;
            if (!previewUrl) throw new Error("生成场景预览不可用");
            if (!storageKey) throw new Error("生成场景尚未保存为画布资源");
            await context.enqueueCommit(async () => {
                const latest = dependencies.getProject(runtime.projectId);
                if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                const latestTarget = latest.nodes.find((node) => node.id === job.nodeId);
                if (!latestTarget) throw new Error("场景首帧节点不存在");
                const createdAt = context.now();
                const metadata = applyGeneratedMediaResultMetadata(
                    latestTarget,
                    imageMetadata({
                        url: previewUrl,
                        storageKey,
                        width: asset.data?.width || latestTarget.width,
                        height: asset.data?.height || latestTarget.height,
                        bytes: asset.data?.bytes || 0,
                        mimeType: asset.data?.mimeType || "image/png",
                    }),
                    {
                        prompt: task.prompt,
                        ...generationTaskMetadata(task),
                    },
                );
                metadata.assetId = asset.id;
                metadata.generationEffectKeys = Array.from(new Set([
                    ...(metadata.generationEffectKeys || []),
                    effectKey,
                ]));
                const nextProduction = recordWeddingSceneVersion(
                    latest.weddingProduction,
                    job,
                    {
                        assetId: asset.id,
                        storageKey,
                        previewUrl,
                        taskId: task.id,
                        createdAt,
                    },
                    createdAt,
                );
                await dependencies.persist(runtime.projectId, {
                    nodes: latest.nodes.map((node) => node.id === job.nodeId
                        ? { ...node, metadata }
                        : node),
                    weddingProduction: nextProduction,
                });
            });
        },
    );
}

function portraitReference(
    dynasty: WeddingDynastyState,
    gender: WeddingPortraitGender,
    getAsset: (assetId: string) => WeddingGenerationAsset | undefined,
): ReferenceImage {
    const assetId = dynasty.portraitSelection?.[gender];
    const version = dynasty.portraitVersionHistory?.[gender]?.find((item) => item.assetId === assetId);
    const asset = assetId ? getAsset(assetId) : undefined;
    const dataUrl = asset?.data?.dataUrl || asset?.data?.url || asset?.coverUrl || version?.previewUrl || "";
    const storageKey = asset?.data?.storageKey || version?.storageKey;
    if (!assetId || !dataUrl || !storageKey) {
        throw new Error(`请先确认${gender === "male" ? "新郎" : "新娘"}定妆照`);
    }
    return {
        id: assetId,
        name: `${dynasty.name}-${gender}-portrait.png`,
        type: asset?.data?.mimeType || "image/png",
        dataUrl,
        url: /^https?:\/\//i.test(dataUrl) ? dataUrl : undefined,
        storageKey,
    };
}

function weddingSceneNeedsGeneration(
    nodes: CanvasNodeData[],
    production: WeddingProductionState,
    job: WeddingSceneJob,
) {
    const dynasty = production.dynasties.find((item) => item.id === job.dynastyId);
    const node = nodes.find((item) => item.id === job.nodeId);
    if (!dynasty || !node) return true;
    if (["failed", "stale", "not_started"].includes(dynasty.statuses.scene)) return true;
    if (!dynasty.sceneSelection) return true;
    return !node.metadata?.content && !node.metadata?.storageKey;
}

function dependenciesWithDefaults(
    overrides: WeddingSceneGenerationDependencies = {},
): Required<WeddingSceneGenerationDependencies> {
    return {
        getProject: overrides.getProject || ((projectId) => useCanvasStore.getState().projects.find((project) => project.id === projectId)),
        persist: overrides.persist || ((projectId, patch) => persistCanvasDocument(projectId, patch)),
        runTask: overrides.runTask || runBackendGenerationTask,
        consumeTask: overrides.consumeTask || consumeGenerationTaskNode,
        getAsset: overrides.getAsset || ((assetId) => {
            const asset = useAssetStore.getState().assets.find((item) => item.id === assetId);
            if (!asset) return undefined;
            if (asset.kind === "image") {
                return {
                    id: asset.id,
                    kind: asset.kind,
                    coverUrl: asset.coverUrl,
                    data: asset.data,
                };
            }
            return { id: asset.id, kind: asset.kind, coverUrl: asset.coverUrl };
        }),
    };
}
