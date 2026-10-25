import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import type { CanvasNodeData } from "@/types/canvas";
import { PortraitStage } from "@/pages/wedding/portrait-stage";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
    source: {
        groomPhotoNodeId: "groom",
        bridePhotoNodeId: "bride",
        maleName: "林屿",
        femaleName: "周棠",
        story: "杭州西湖相识",
        consentConfirmed: true,
        ratio: "16:9",
        resolution: "720p",
    },
    dynasties: [{
        id: "tang",
        name: "唐",
        order: 0,
        portraitMaleNodeId: "male-tang",
        portraitFemaleNodeId: "female-tang",
        sceneNodeId: "scene-tang",
        videoNodeId: "video-tang",
    }],
    now: "2026-10-25T00:00:00.000Z",
});

const nodes: CanvasNodeData[] = [
    {
        id: "male-tang",
        type: "image",
        title: "唐 · 新郎肖像",
        position: { x: 0, y: 0 },
        width: 420,
        height: 280,
        metadata: {
            content: "https://example.com/male-tang.png",
            storageKey: "image:male-tang",
            status: "success",
            assetId: "asset-male-tang",
        },
    },
    {
        id: "female-tang",
        type: "image",
        title: "唐 · 新娘肖像",
        position: { x: 480, y: 0 },
        width: 420,
        height: 280,
        metadata: { status: "idle" },
    },
];

test("portrait stage renders real male and female production slots", () => {
    const html = renderToStaticMarkup(
        <PortraitStage
            production={production}
            nodes={nodes}
            tasks={{}}
            saving={false}
            feedback=""
            onGenerateAll={async () => undefined}
            onGenerateJob={async () => undefined}
            onSelectVersion={async () => undefined}
            onConfirmPortrait={async () => undefined}
            onApprove={async () => undefined}
        />,
    );

    expect(html).toContain("唐");
    expect(html).toContain("新郎定妆照");
    expect(html).toContain("新娘定妆照");
    expect(html).toContain("https://example.com/male-tang.png");
    expect(html).toContain("生成缺失定妆照");
    expect(html).toContain("确认定妆照阶段");
});
