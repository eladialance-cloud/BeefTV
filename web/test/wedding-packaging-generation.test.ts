import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { uploadWeddingBgm } from "@/services/wedding-packaging-generation";
import { defaultConfig } from "@/stores/use-config-store";
import type { CanvasNodeData } from "@/types/canvas";

test("persists an uploaded BGM node as an asset-library item", async () => {
    const production = createWeddingProductionState({
        template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
        source: {
            groomPhotoNodeId: "groom",
            bridePhotoNodeId: "bride",
            maleName: "林屿",
            femaleName: "周棠",
            story: "",
            consentConfirmed: true,
            ratio: "16:9",
            resolution: "720p",
        },
        dynasties: [{
            id: "tang",
            name: "唐",
            order: 0,
            portraitMaleNodeId: "male",
            portraitFemaleNodeId: "female",
            sceneNodeId: "scene",
            videoNodeId: "video",
        }],
        now: "2026-10-25T00:00:00.000Z",
    });
    let currentProject: { nodes: CanvasNodeData[]; weddingProduction: typeof production } = {
        nodes: [],
        weddingProduction: production,
    };
    let ensuredNode: CanvasNodeData | undefined;

    await uploadWeddingBgm({
        projectId: "canvas-1",
        production,
        config: defaultConfig,
        dependencies: {
            getProject: () => currentProject,
            uploadAudio: async () => ({
                url: "/api/resources/bgm/file",
                storageKey: "resource:bgm",
                bytes: 1024,
                mimeType: "audio/mpeg",
                durationMs: 30_000,
            }),
            ensureAsset: async ({ node }) => {
                ensuredNode = node;
                return {
                    assetId: "asset-bgm",
                    created: true,
                    linkedToProject: true,
                    confirmed: true,
                };
            },
            persist: async (_projectId, patch) => {
                currentProject = {
                    nodes: patch.nodes,
                    weddingProduction: patch.weddingProduction,
                };
            },
        },
    }, new File(["bgm"], "wedding-theme.mp3", { type: "audio/mpeg" }));

    expect(ensuredNode?.type).toBe("audio");
    expect(ensuredNode?.metadata).toMatchObject({
        content: "/api/resources/bgm/file",
        storageKey: "resource:bgm",
    });
    expect(currentProject.nodes[0]?.metadata?.assetId).toBe("asset-bgm");
});
