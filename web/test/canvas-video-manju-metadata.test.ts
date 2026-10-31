import { beforeEach, expect, mock, test } from "bun:test";

import { CanvasNodeType } from "@/types/canvas";
import { useCanvasStore, type CanvasProject } from "@/stores/canvas/use-canvas-store";

const submittedInputs: Array<Record<string, unknown>> = [];
const originalGenerationModule = await import("@/lib/canvas/canvas-project-generation");

mock.module("@/lib/canvas/canvas-project-generation", () => ({
    ...originalGenerationModule,
    runCanvasGenerationTaskToConsumer: async (input: Record<string, unknown>) => {
        submittedInputs.push(input);
    },
}));

const { executeVideoGeneration } = await import("@/pages/canvas/canvas-media-generation-executors");

const sourceNode = {
    id: "video-manju-segment-1",
    type: CanvasNodeType.Video,
    title: "段1 · 分镜视频",
    position: { x: 100, y: 200 },
    width: 420,
    height: 747,
    metadata: {
        status: "idle" as const,
        prompt: "段1 视频提示词",
        composerContent: "段1 视频提示词",
        manjuProjectId: "manju-project-1",
        manjuStage: "p4_videos" as const,
        manjuTaskId: "p4-segment-1",
        manjuTaskCode: "段1",
        manjuGlobalStylePrompt: "全片风格锁",
    },
};

const project: CanvasProject = {
    id: "canvas-video-manju",
    title: "漫剧画布",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-01",
    revision: 1,
    nodes: [sourceNode],
    connections: [],
    chatSessions: [],
    activeChatId: null,
    backgroundMode: "grid",
    showImageInfo: false,
    viewport: { x: 0, y: 0, k: 1 },
    directorScenes: [],
};

beforeEach(() => {
    submittedInputs.length = 0;
    useCanvasStore.setState({ projects: [structuredClone(project)] });
});

test("video generation submits the Manju task identity used by recovery", async () => {
    await executeVideoGeneration({
        resolveReferenceLinks: async () => undefined,
        nodeId: sourceNode.id,
        sourceNode,
        canvasNodes: [sourceNode],
        nodesRef: { current: [sourceNode] },
        prompt: sourceNode.metadata.prompt,
        effectivePrompt: sourceNode.metadata.prompt,
        generationConfig: {
            model: "seedance-2",
            size: "9:16",
            videoSeconds: "5",
            videoGenerateAudio: "false",
            videoWatermark: "false",
            count: "1",
        } as never,
        generationContext: { referenceImages: [], referenceVideos: [], referenceAudios: [] },
        controller: new AbortController(),
        projectId: project.id,
        canvasConnections: [],
        setNodes: () => undefined,
        setConnections: () => undefined,
        startGenerationRequest: () => undefined,
        finishGenerationRequest: () => undefined,
        bindGenerationTask: () => undefined,
        applyGenerationTaskResult: async () => undefined,
        registerPendingNodeIds: () => undefined,
        styleMetadata: {},
        skillMetadata: {},
        taskContext: {},
        clientOperationId: "test-video-manju-metadata",
    } as never);

    expect(submittedInputs).toHaveLength(1);
    expect(submittedInputs[0]?.metadata).toMatchObject({
        manjuProjectId: "manju-project-1",
        manjuStage: "p4_videos",
        manjuTaskId: "p4-segment-1",
        manjuTaskCode: "段1",
    });
});
