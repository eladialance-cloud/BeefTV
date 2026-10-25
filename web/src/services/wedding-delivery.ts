import {
    beginLocalExecutorSession,
    runOwnedTimelineRender,
} from "@/lib/plugins/builtin/editor/local-executor-session";
import {
    failWeddingDeliveryRendering,
    markWeddingDeliveryRendering,
    recordWeddingDeliveryResult,
} from "@/lib/wedding/delivery-runtime";
import type {
    WeddingDeliveryResult,
    WeddingProductionState,
} from "@/lib/wedding/types";
import type { GenerationTask } from "@/services/api/task-center";
import type { TimelineProject } from "@/types/timeline";

export type WeddingDeliveryRenderRuntime = {
    projectId: string;
    production: WeddingProductionState;
    timeline: TimelineProject;
    clientOperationId: string;
    signal?: AbortSignal;
    onTaskUpdate?: (task: GenerationTask) => void;
};

export type WeddingDeliveryDependencies = {
    runRender?: (input: {
        projectId: string;
        timeline: TimelineProject;
        clientOperationId: string;
        onCreated?: (task: GenerationTask) => void;
        onTaskUpdate?: (task: GenerationTask) => void;
    }) => Promise<{ task: GenerationTask; result: WeddingDeliveryResult }>;
    persist?: (projectId: string, production: WeddingProductionState) => Promise<void>;
};

export async function renderWeddingDelivery(
    runtime: WeddingDeliveryRenderRuntime,
    dependencies: WeddingDeliveryDependencies = {},
) {
    const persist = dependencies.persist || (async (projectId, production) => {
        const { persistCanvasDocument } = await import("@/services/local-workspace-repository");
        await persistCanvasDocument(projectId, { weddingProduction: production });
    });
    const queuedWrites: Promise<void>[] = [];
    let latestTask: GenerationTask | undefined;
    const initial = markWeddingDeliveryRendering(runtime.production, "", new Date().toISOString());
    await persist(runtime.projectId, initial);
    try {
        const executed = await (dependencies.runRender || defaultRunRender)({
            projectId: runtime.projectId,
            timeline: runtime.timeline,
            clientOperationId: runtime.clientOperationId,
            onCreated: (task) => {
                latestTask = task;
                queuedWrites.push(persist(
                    runtime.projectId,
                    markWeddingDeliveryRendering(initial, task.id, new Date().toISOString()),
                ));
            },
            onTaskUpdate: (task) => {
                latestTask = task;
                runtime.onTaskUpdate?.(task);
            },
        });
        await Promise.all(queuedWrites);
        const withTask = latestTask
            ? markWeddingDeliveryRendering(initial, latestTask.id, new Date().toISOString())
            : initial;
        const completed = recordWeddingDeliveryResult(
            withTask,
            executed.result,
            new Date().toISOString(),
        );
        completed.delivery = { ...completed.delivery!, taskId: executed.task.id };
        await persist(runtime.projectId, completed);
        return executed.result;
    } catch (error) {
        const message = error instanceof Error ? error.message : "成片渲染失败";
        const failed = failWeddingDeliveryRendering(
            latestTask ? markWeddingDeliveryRendering(initial, latestTask.id, new Date().toISOString()) : initial,
            message,
            new Date().toISOString(),
        );
        await persist(runtime.projectId, failed);
        throw error;
    }
}

export function weddingDeliveryManifest(production: WeddingProductionState) {
    return JSON.stringify({
        schema: "visideep.cross-era-wedding.delivery/v1",
        exportedAt: new Date().toISOString(),
        template: production.template,
        source: production.source,
        dynasties: production.dynasties.map((dynasty) => ({
            id: dynasty.id,
            name: dynasty.name,
            promptOverride: dynasty.promptOverride || "",
            portraitSelection: dynasty.portraitSelection,
            portraitVersionHistory: dynasty.portraitVersionHistory || {},
            sceneSelection: dynasty.sceneSelection,
            sceneVersions: dynasty.sceneVersions || [],
            sceneQc: dynasty.qc.scene || null,
            videoSelection: dynasty.videoSelection,
            videoVersions: dynasty.videoVersions || [],
            videoQc: dynasty.qc.video || null,
            qcOverrides: dynasty.qc.overrides,
        })),
        packaging: production.packaging,
        delivery: production.delivery || null,
        activity: production.activity,
    }, null, 2);
}

async function defaultRunRender(input: {
    projectId: string;
    timeline: TimelineProject;
    clientOperationId: string;
    onCreated?: (task: GenerationTask) => void;
    onTaskUpdate?: (task: GenerationTask) => void;
}) {
    const controller = new AbortController();
    const session = beginLocalExecutorSession(input.projectId, {
        controller,
        getLiveProjectId: () => input.projectId,
    });
    return runOwnedTimelineRender({
        session,
        projectId: input.projectId,
        timeline: input.timeline,
        clientOperationId: input.clientOperationId,
        onCreated: input.onCreated,
        onTaskUpdate: input.onTaskUpdate,
    });
}
