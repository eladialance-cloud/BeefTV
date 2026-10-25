import {
    applyGeneratedMediaResultMetadata,
    videoMetadata,
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
    weddingPortraitReference,
    weddingSceneReference,
    weddingVideoReference,
} from "@/lib/wedding/generation-runtime";
import {
    failWeddingVideoGeneration,
    markWeddingVideoGenerating,
    recordWeddingVideoVersion,
    weddingVideoJobs,
    type WeddingVideoJob,
} from "@/lib/wedding/video-runtime";
import type { WeddingProductionState } from "@/lib/wedding/types";
import {
    applyWeddingVideoQc,
    errorWeddingVideoQc,
    parseWeddingVideoQc,
    videoQcPrompt,
} from "@/lib/wedding/video-qc";
import { persistCanvasDocument } from "@/services/local-workspace-repository";
import { consumeGenerationTaskNode, ensureCanvasNodeAsset } from "@/services/project-asset-sync";
import { runBackendGenerationTask, type BackendGenerationResult } from "@/services/api/generation-task";
import type { GenerationTask } from "@/services/api/task-center";
import { useAssetStore } from "@/stores/use-asset-store";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";
import type { AiConfig } from "@/stores/use-config-store";
import type { CanvasNodeData } from "@/types/canvas";

type WeddingVideoProjectSnapshot = Pick<CanvasProject, "nodes" | "weddingProduction">;
type WeddingVideoPersistPatch = {
    nodes: CanvasNodeData[];
    weddingProduction: WeddingProductionState;
};

export type WeddingVideoGenerationRuntime = {
    projectId: string;
    production: WeddingProductionState;
    config: AiConfig;
    signal?: AbortSignal;
    now?: () => string;
    onTaskUpdate?: (job: WeddingVideoJob, task: GenerationTask) => void;
    onJobComplete?: (job: WeddingVideoJob) => void;
};

export type WeddingVideoGenerationOptions = {
    force?: boolean;
    nodeIds?: string[];
    dependencies?: WeddingVideoGenerationDependencies;
};

export type WeddingVideoGenerationDependencies = {
    getProject?: (projectId: string) => WeddingVideoProjectSnapshot | undefined;
    persist?: (projectId: string, patch: WeddingVideoPersistPatch) => Promise<void>;
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

export type WeddingVideoQcRuntime = {
    projectId: string;
    production: WeddingProductionState;
    config: AiConfig;
    signal?: AbortSignal;
    onTaskUpdate?: (dynastyId: string, task: GenerationTask) => void;
};

export type WeddingVideoQcOptions = {
    dependencies?: WeddingVideoQcDependencies;
};

export type WeddingVideoQcDependencies = Pick<
    WeddingVideoGenerationDependencies,
    "getProject" | "persist" | "runTask" | "getAsset"
>;

export async function generateWeddingVideos(
    runtime: WeddingVideoGenerationRuntime,
    dynastyId?: string,
    options: WeddingVideoGenerationOptions = {},
): Promise<WeddingGenerationSummary> {
    const dependencies = dependenciesWithDefaults(options.dependencies);
    const project = dependencies.getProject!(runtime.projectId);
    const production = project?.weddingProduction || runtime.production;
    const selectedNodeIds = new Set(options.nodeIds || []);
    const jobs = weddingVideoJobs(production, dynastyId).filter((job) => {
        if (selectedNodeIds.size && !selectedNodeIds.has(job.nodeId)) return false;
        if (options.force) return true;
        return weddingVideoNeedsGeneration(project?.nodes || [], production, job);
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
                await generateWeddingVideoJob(runtime, job, { dependencies, enqueueCommit, now });
                succeeded += 1;
                runtime.onJobComplete?.(job);
            } catch (error) {
                const message = error instanceof Error ? error.message : "朝代视频生成失败";
                errors.push({ nodeId: job.nodeId, dynastyId: job.dynastyId, message });
                await enqueueCommit(async () => {
                    const latest = dependencies.getProject!(runtime.projectId);
                    if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                    const target = latest.nodes.find((node) => node.id === job.nodeId);
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
                        weddingProduction: failWeddingVideoGeneration(
                            latest.weddingProduction,
                            job,
                            message,
                            now(),
                        ),
                    });
                });
            }
        }
    };

    await Promise.all(Array.from({ length: Math.min(2, jobs.length) }, () => worker()));
    await commitTail;
    return { attempted: jobs.length, succeeded, failed: errors.length, errors };
}

export async function runWeddingVideoQc(
    runtime: WeddingVideoQcRuntime,
    dynastyId: string,
    options: WeddingVideoQcOptions = {},
) {
    const dependencies = videoQcDependenciesWithDefaults(options.dependencies);
    const project = dependencies.getProject(runtime.projectId);
    if (!project?.weddingProduction) throw new Error("婚礼制作状态不存在");
    const dynasty = project.weddingProduction.dynasties.find((item) => item.id === dynastyId);
    const target = dynasty ? project.nodes.find((node) => node.id === dynasty.videoNodeId) : undefined;
    if (!dynasty) throw new Error("朝代状态不存在");
    if (!target) throw new Error("视频节点不存在");
    if (!dynasty.videoSelection) throw new Error("请先确认朝代视频");
    const referenceImages = [
        weddingPortraitReference(dynasty, "male", dependencies.getAsset),
        weddingPortraitReference(dynasty, "female", dependencies.getAsset),
        weddingSceneReference(dynasty, dependencies.getAsset),
    ];
    const referenceVideos = [weddingVideoReference(dynasty, dependencies.getAsset)];
    const checkedAt = new Date().toISOString();
    try {
        const result = await dependencies.runTask({
            projectId: runtime.projectId,
            mode: "text",
            prompt: videoQcPrompt(dynasty.name),
            config: buildGenerationConfig(runtime.config, target, "text"),
            referenceImages,
            referenceVideos,
            signal: runtime.signal,
            metadata: {
                source: "wedding-workbench",
                stage: "video-qc",
                dynastyId,
            },
            onTaskUpdate: (task) => runtime.onTaskUpdate?.(dynastyId, task),
        });
        if (!result.text?.trim()) throw new Error("视频 QC 没有返回文本结果");
        const qc = parseWeddingVideoQc(result.text, checkedAt);
        const latest = dependencies.getProject(runtime.projectId);
        if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
        await dependencies.persist(runtime.projectId, {
            nodes: latest.nodes,
            weddingProduction: applyWeddingVideoQc(latest.weddingProduction, dynastyId, qc, checkedAt),
        });
        return qc;
    } catch (error) {
        const message = error instanceof Error ? error.message : "视频 QC 执行失败";
        const latest = dependencies.getProject(runtime.projectId);
        if (latest?.weddingProduction) {
            await dependencies.persist(runtime.projectId, {
                nodes: latest.nodes,
                weddingProduction: errorWeddingVideoQc(
                    latest.weddingProduction,
                    dynastyId,
                    message,
                    checkedAt,
                ),
            });
        }
        throw error;
    }
}

async function generateWeddingVideoJob(
    runtime: WeddingVideoGenerationRuntime,
    job: WeddingVideoJob,
    context: {
        dependencies: Required<WeddingVideoGenerationDependencies>;
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
    if (!target) throw new Error("视频节点不存在");
    const referenceImages = [
        weddingPortraitReference(dynasty, "male", dependencies.getAsset),
        weddingPortraitReference(dynasty, "female", dependencies.getAsset),
        weddingSceneReference(dynasty, dependencies.getAsset),
    ];
    const creationTemplate = creationTemplateGenerationMetadata(target.metadata).creationTemplate;
    if (!creationTemplate) throw new Error("视频节点缺少工作流模板引用");
    const taskCommits: Promise<void>[] = [];
    let initialTaskPersisted = false;
    let completedTask: GenerationTask | undefined;
    const config = {
        ...buildGenerationConfig(runtime.config, target, "video"),
        videoSeconds: String(project.weddingProduction.source.durationSeconds || 8),
    };

    await dependencies.runTask({
        projectId: runtime.projectId,
        mode: "video",
        prompt: target.metadata?.prompt?.trim() || target.title,
        config,
        referenceImages,
        signal: runtime.signal,
        metadata: {
            source: "wedding-workbench",
            stage: "videos",
            dynastyId: job.dynastyId,
            videoEditOperation: target.metadata?.videoEditOperation || "reference_to_video",
            creationTemplate: {
                ...creationTemplate,
                ...(dynasty.promptOverride ? { promptOverride: dynasty.promptOverride } : {}),
            },
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
                        weddingProduction: markWeddingVideoGenerating(
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
            if (!asset || asset.kind !== "video") throw new Error("生成视频素材不可用");
            const previewUrl = asset.data?.url || asset.data?.dataUrl || "";
            const storageKey = asset.data?.storageKey;
            if (!previewUrl) throw new Error("生成视频预览不可用");
            if (!storageKey) throw new Error("生成视频尚未保存为画布资源");
            const durationMs = asset.data?.durationMs;
            await context.enqueueCommit(async () => {
                const latest = dependencies.getProject(runtime.projectId);
                if (!latest?.weddingProduction) throw new Error("婚礼制作状态不存在");
                const latestTarget = latest.nodes.find((node) => node.id === job.nodeId);
                if (!latestTarget) throw new Error("视频节点不存在");
                const createdAt = context.now();
                const metadata = applyGeneratedMediaResultMetadata(
                    latestTarget,
                    videoMetadata({
                        url: previewUrl,
                        storageKey,
                        width: asset.data?.width || latestTarget.width,
                        height: asset.data?.height || latestTarget.height,
                        durationMs,
                        bytes: asset.data?.bytes || 0,
                        mimeType: asset.data?.mimeType || "video/mp4",
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
                const nextProduction = recordWeddingVideoVersion(
                    latest.weddingProduction,
                    job,
                    {
                        assetId: asset.id,
                        storageKey,
                        previewUrl,
                        posterUrl: asset.coverUrl,
                        taskId: task.id,
                        durationMs,
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

function weddingVideoNeedsGeneration(
    nodes: CanvasNodeData[],
    production: WeddingProductionState,
    job: WeddingVideoJob,
) {
    const dynasty = production.dynasties.find((item) => item.id === job.dynastyId);
    const node = nodes.find((item) => item.id === job.nodeId);
    if (!dynasty || !node) return true;
    if (["failed", "stale", "not_started"].includes(dynasty.statuses.video)) return true;
    if (!dynasty.videoSelection) return true;
    return !node.metadata?.content && !node.metadata?.storageKey;
}

function videoQcDependenciesWithDefaults(
    overrides: WeddingVideoQcDependencies = {},
): Required<WeddingVideoQcDependencies> {
    const defaults = dependenciesWithDefaults();
    return {
        getProject: overrides.getProject || defaults.getProject,
        persist: overrides.persist || defaults.persist,
        runTask: overrides.runTask || defaults.runTask,
        getAsset: overrides.getAsset || defaults.getAsset,
    };
}

function dependenciesWithDefaults(
    overrides: WeddingVideoGenerationDependencies = {},
): Required<WeddingVideoGenerationDependencies> {
    return {
        getProject: overrides.getProject || ((projectId) => useCanvasStore.getState().projects.find((project) => project.id === projectId)),
        persist: overrides.persist || ((projectId, patch) => persistCanvasDocument(projectId, patch)),
        runTask: overrides.runTask || runBackendGenerationTask,
        consumeTask: overrides.consumeTask || consumeGenerationTaskNode,
        ensureAsset: overrides.ensureAsset || ensureCanvasNodeAsset,
        getAsset: overrides.getAsset || ((assetId) => {
            const asset = useAssetStore.getState().assets.find((item) => item.id === assetId);
            if (!asset) return undefined;
            if (asset.kind === "image" || asset.kind === "video") {
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
