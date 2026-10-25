import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import type { CanvasNodeData } from "@/types/canvas";
import { SceneStage } from "@/pages/wedding/scene-stage";

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
production.dynasties[0]!.portraitSelection = { male: "male-asset", female: "female-asset" };
production.dynasties[0]!.portraitApproval = { male: true, female: true };
production.dynasties[0]!.portraitVersionHistory = {
    male: [{
        assetId: "male-asset",
        nodeId: "male-tang",
        previewUrl: "https://example.com/male.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
    female: [{
        assetId: "female-asset",
        nodeId: "female-tang",
        previewUrl: "https://example.com/female.png",
        createdAt: "2026-10-25T01:00:00.000Z",
    }],
};
production.dynasties[0]!.sceneSelection = "scene-asset";
production.dynasties[0]!.sceneVersions = [{
    assetId: "scene-asset",
    nodeId: "scene-tang",
    previewUrl: "https://example.com/scene.png",
    createdAt: "2026-10-25T02:00:00.000Z",
}];
production.dynasties[0]!.qc.scene = {
    verdict: "fail",
    couplePresent: true,
    faceMatchMale: false,
    faceMatchFemale: true,
    outfitMatch: true,
    sceneMatch: true,
    extraPerson: false,
    watermarkOrText: false,
    reason: "新郎脸部漂移",
    suggestion: "提高肖像参考权重",
    checkedAt: "2026-10-25T03:00:00.000Z",
};

const nodes: CanvasNodeData[] = [{
    id: "scene-tang",
    type: "image",
    title: "唐 · 双人婚礼场景",
    position: { x: 0, y: 0 },
    width: 520,
    height: 320,
    metadata: {
        content: "https://example.com/scene.png",
        storageKey: "resource:scene",
        status: "success",
        assetId: "scene-asset",
    },
}];

test("scene stage renders references, QC evidence and real production actions", () => {
    const html = renderToStaticMarkup(
        <SceneStage
            production={production}
            nodes={nodes}
            tasks={{}}
            saving={false}
            feedback=""
            onGenerateAll={async () => undefined}
            onGenerateDynasty={async () => undefined}
            onRunQc={async () => undefined}
            onSelectVersion={async () => undefined}
            onConfirm={async () => undefined}
            onOverrideQc={async () => undefined}
            onApprove={async () => undefined}
        />,
    );

    expect(html).toContain("双人婚礼场景");
    expect(html).toContain("https://example.com/male.png");
    expect(html).toContain("https://example.com/female.png");
    expect(html).toContain("https://example.com/scene.png");
    expect(html).toContain("自动 QC");
    expect(html).toContain("新郎脸部漂移");
    expect(html).toContain("覆盖原因");
    expect(html).toContain("生成缺失场景");
    expect(html).toContain("确认场景阶段");
});
