import { expect, test } from "bun:test";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { syncWeddingVideoDurationNodes } from "@/lib/wedding/duration-state";
import type { CanvasNodeData } from "@/types/canvas";

test("syncs the selected duration into every dynasty video node", () => {
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
        dynasties: [
            {
                id: "tang",
                name: "唐",
                order: 0,
                portraitMaleNodeId: "male-tang",
                portraitFemaleNodeId: "female-tang",
                sceneNodeId: "scene-tang",
                videoNodeId: "video-tang",
            },
            {
                id: "ming",
                name: "明",
                order: 1,
                portraitMaleNodeId: "male-ming",
                portraitFemaleNodeId: "female-ming",
                sceneNodeId: "scene-ming",
                videoNodeId: "video-ming",
            },
        ],
        now: "2026-10-25T00:00:00.000Z",
    });
    const nodes = [
        videoNode("video-tang", "8"),
        videoNode("video-ming", "8"),
        videoNode("scene-tang", "8"),
    ];

    const synchronized = syncWeddingVideoDurationNodes(nodes, production, 12);

    expect(synchronized.find((node) => node.id === "video-tang")?.metadata?.seconds).toBe("12");
    expect(synchronized.find((node) => node.id === "video-tang")?.metadata?.pluginData?.creationTemplateVariables)
        .toMatchObject({ duration: 12 });
    expect(synchronized.find((node) => node.id === "video-ming")?.metadata?.seconds).toBe("12");
    expect(synchronized.find((node) => node.id === "scene-tang")).toBe(nodes[2]);
});

function videoNode(id: string, seconds: string): CanvasNodeData {
    return {
        id,
        type: "video",
        title: id,
        position: { x: 0, y: 0 },
        width: 320,
        height: 240,
        metadata: {
            seconds,
            pluginData: {
                creationTemplateVariables: {
                    duration: Number(seconds),
                },
            },
        },
    };
}
