import {
    buildGenerationConfig,
    generationTaskMetadata,
    nodeReferenceImage,
} from "@/lib/canvas/canvas-project-generation";
import {
    applyGeneratedMediaResultMetadata,
    imageMetadata,
} from "@/lib/canvas/canvas-generation-task-sync";
import { creationTemplateGenerationMetadata } from "@/lib/canvas/creation-template-workflow";
import {
    failWeddingPortraitGeneration,
    markWeddingPortraitGenerating,
    recordWeddingPortraitVersion,
    weddingPortraitJobs,
    type WeddingGenerationJob,
} from "@/lib/wedding/portrait-runtime";
import type {
    WeddingGenerationAsset,
    WeddingGenerationSummary,
} from "@/lib/wedding/generation-runtime";
import type { WeddingProductionState } from "@/lib/wedding/types";
import { persistCanvasDocument } from "@/services/local-workspace-repository";
import { consumeGenerationTaskNode, ensureCanvasNodeAsset } from "@/services/project-asset-sync";
import {
    runBackendGenerationTask,
    type BackendGenerationResult,
} from "@/services/api/generation-task";
import type { GenerationTask } from "@/services/api/task-center";
import { useAssetStore } from "@/stores/use-asset-store";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";
import type { AiConfig } from "@/stores/use-config-store";
import type { CanvasNodeData } from "@/types/canvas";

type WeddingPortraitProjectSnapshot = Pick<CanvasProject, "nodes" | "weddingProduction">;
type WeddingPortraitPersistPatch = {
    nodes: CanvasNodeData[];
    weddingProduction: WeddingProductionState;
};
export type WeddingPortraitGenerationRuntime = {
    projectId: string;
    production: WeddingProductionState;
    config: AiConfig;
    signal?: AbortSignal;
    now?: () => string;
    onTaskUpdate?: (job: WeddingGenerationJob, task: GenerationTask) => void;
    onJobComplete?: (job: WeddingGenerationJob) => void;
};

export type WeddingPortraitGenerationOptions = {
    force?: boolean;
    nodeIds?: string[];
    dependencies?: WeddingPortraitGenerationDependencies;
};

export type WeddingPortraitGenerationDependencies = {
    getProject?: (projectId: string) => WeddingPortraitProjectSnapshot | undefined;
    persist?: (projectId: string, patch: WeddingPortraitPersistPatch) => Promise<void>;
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
    ensureAsset?: typeof ensureCanvasNodeAsset;
    getAsset?: (assetId: string) => WeddingGenerationAsset | undefined;
};

export type WeddingPortraitGenerationSummary = WeddingGenerationSummary;

export async function generateWeddingPortraits(
    runtime: WeddingPortraitGenerationRuntime,
    dynastyId?: string,
    options: WeddingPortraitGenerationOptions = {},
): Promise<WeddingPortraitGenerationSummary> {
    const dependencies = dependenciesWithDefaults(options.dependencies);
    const project = dependencies.getProject!(runtime.projectId);
    const production = project?.weddingProduction || runtime.production;
    const selectedNodeIds = new Set(options.nodeIds || []);
    const jobs = weddingPortraitJobs(production, dynastyId).filter((job) => {
        if (selectedNodeIds.size && !selectedNodeIds.has(job.nodeId)) return false;
        if (options.force) return true;
        return weddingPortraitNeedsGeneration(project?.nodes || [], production, job);
    });
    if (!jobs.length) return { attempted: 0, succeeded: 0, failed: 0, errors: [] };

    let commitTail = Promise.resolve();
    const enqueueCommit = (operation: () => Promise<void>) => {
        const running = commitTail.then(operation, operation);
        commitTail = running.catch(() => undefined);
        return running;
    };
    const now = runtime.now || (() => new Date().toISOString());
    const errors: WeddingPortraitGenerationSummary["errors"] = [];
    let succeeded = 0;
    let cursor = 0;

    const worker = async () => {
        for (;;) {
            const index = cursor;
            cursor += 1;
            const job = jobs[index];
            if (!job) return;
            try {
                await generateWeddingPortraitJob(runtime, job, {
                    dependencies,
                    enqueueCommit,
                    now,
                });
                succeeded += 1;
                runtime.onJobComplete?.(job);
            } catch (error) {
                const message = error instanceof Error ? error.message : "定妆照生成失败";
                errors.push({ nodeId: job.nodeId, dynastyId: job.dynastyId, gender: job.gender, message });
                await enqueueCommit(async () => {
                    const latest = dependencies.getProject!(runtime.projectId);
                    if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                    const target = latest.nodes.find((node) => node.id === job.nodeId);
                    const failedProduction = failWeddingPortraitGeneration(
                        latest.weddingProduction,
                        job,
                        message,
                        now(),
                    );
                    const nodes = target
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
                        : latest.nodes;
                    await dependencies.persist!(runtime.projectId, {
                        nodes,
                        weddingProduction: failedProduction,
                    });
                });
            }
        }
    };

    await Promise.all(Array.from({ length: Math.min(2, jobs.length) }, () => worker()));
    await commitTail;
    return {
        attempted: jobs.length,
        succeeded,
        failed: errors.length,
        errors,
    };
}

async function generateWeddingPortraitJob(
    runtime: WeddingPortraitGenerationRuntime,
    job: WeddingGenerationJob,
    context: {
        dependencies: Required<WeddingPortraitGenerationDependencies>;
        enqueueCommit: (operation: () => Promise<void>) => Promise<void>;
        now: () => string;
    },
) {
    const { dependencies } = context;
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const production = project.weddingProduction;
    const target = project.nodes.find((node) => node.id === job.nodeId);
    if (!target) throw new Error("定妆照节点不存在");
    const source = project.nodes.find((node) => node.id === job.promptSourceNodeId);
    const reference = source ? nodeReferenceImage(source) : null;
    if (!reference) throw new Error("原始照片尚未保存，请先在项目设定中完成上传");

    const taskCommits: Promise<void>[] = [];
    let initialTaskPersisted = false;
    let completedTask: GenerationTask | undefined;
    const config = {
        ...buildGenerationConfig(runtime.config, target, "image"),
        count: "1",
    };
    await (dependencies.runTask)({
        projectId: runtime.projectId,
        mode: "image",
        prompt: target.metadata?.prompt?.trim() || target.title,
        config,
        referenceImages: [reference],
        signal: runtime.signal,
        metadata: {
            source: "wedding-workbench",
            stage: "portraits",
            dynastyId: job.dynastyId,
            portraitGender: job.gender,
            promptSourceNodeId: job.promptSourceNodeId,
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
                        weddingProduction: markWeddingPortraitGenerating(latest.weddingProduction, job, context.now()),
                    });
                }));
            }
        },
    });
    await Promise.all(taskCommits);
    if (!completedTask) throw new Error("生成任务缺少成功终态");

    await (dependencies.consumeTask)(
        completedTask,
        job.nodeId,
        0,
        async ({ task, output, effectKey }) => {
            const assetId = output.materializedAssetId;
            if (!assetId) throw new Error("生成任务输出尚未物化");
            const asset = dependencies.getAsset(assetId);
            if (!asset || asset.kind !== "image") throw new Error("生成图片素材不可用");
            const previewUrl = asset.data?.dataUrl || asset.data?.url || asset.coverUrl || "";
            if (!previewUrl) throw new Error("生成图片预览不可用");
            const storageKey = asset.data?.storageKey;
            if (!storageKey) throw new Error("生成图片尚未保存为画布资源");
            await context.enqueueCommit(async () => {
                const latest = dependencies.getProject(runtime.projectId);
                if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                const latestTarget = latest.nodes.find((node) => node.id === job.nodeId);
                if (!latestTarget) throw new Error("定妆照节点不存在");
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
                const ensured = await dependencies.ensureAsset({
                    canvasId: runtime.projectId,
                    node: { ...latestTarget, metadata },
                    source: "canvas-generation",
                    taskId: task.id,
                });
                metadata.assetId = ensured.assetId;
                const nextProduction = recordWeddingPortraitVersion(
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

function weddingPortraitNeedsGeneration(
    nodes: CanvasNodeData[],
    production: WeddingProductionState,
    job: WeddingGenerationJob,
) {
    const dynasty = production.dynasties.find((item) => item.id === job.dynastyId);
    const node = nodes.find((item) => item.id === job.nodeId);
    if (!dynasty || !node) return true;
    if (["failed", "stale", "not_started"].includes(dynasty.statuses.portrait)) return true;
    if (!dynasty.portraitSelection?.[job.gender]) return true;
    return !node.metadata?.content && !node.metadata?.storageKey;
}

function dependenciesWithDefaults(
    overrides: WeddingPortraitGenerationDependencies = {},
): Required<WeddingPortraitGenerationDependencies> {
    return {
        getProject: overrides.getProject || ((projectId) => useCanvasStore.getState().projects.find((project) => project.id === projectId)),
        persist: overrides.persist || ((projectId, patch) => persistCanvasDocument(projectId, patch)),
        runTask: overrides.runTask || runBackendGenerationTask,
        consumeTask: overrides.consumeTask || consumeGenerationTaskNode,
        ensureAsset: overrides.ensureAsset || ensureCanvasNodeAsset,
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
