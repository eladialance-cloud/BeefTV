import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import type { CanvasNodeData } from "@/types/canvas";
import { VideoStage } from "@/pages/wedding/video-stage";

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
const tang = production.dynasties[0]!;
tang.portraitSelection = { male: "male-asset", female: "female-asset" };
tang.portraitApproval = { male: true, female: true };
tang.portraitVersionHistory = {
    male: [{ assetId: "male-asset", nodeId: "male-tang", previewUrl: "https://example.com/male.png", createdAt: "2026-10-25T01:00:00.000Z" }],
    female: [{ assetId: "female-asset", nodeId: "female-tang", previewUrl: "https://example.com/female.png", createdAt: "2026-10-25T01:00:00.000Z" }],
};
tang.sceneSelection = "scene-asset";
tang.sceneVersions = [{
    assetId: "scene-asset",
    nodeId: "scene-tang",
    previewUrl: "https://example.com/scene.png",
    createdAt: "2026-10-25T02:00:00.000Z",
}];
tang.videoSelection = "video-asset";
tang.videoVersions = [{
    assetId: "video-asset",
    nodeId: "video-tang",
    previewUrl: "https://example.com/video.mp4",
    posterUrl: "https://example.com/poster.jpg",
    durationMs: 8000,
    createdAt: "2026-10-25T03:00:00.000Z",
}];
tang.qc.video = {
    verdict: "fail",
    faceStable: false,
    outfitStable: true,
    actionReadable: true,
    extraPerson: false,
    watermarkOrText: false,
    reason: "中段脸部漂移",
    checkedAt: "2026-10-25T04:00:00.000Z",
};

const nodes: CanvasNodeData[] = [{
    id: "video-tang",
    type: "video",
    title: "唐 · 朝代视频",
    position: { x: 0, y: 0 },
    width: 520,
    height: 360,
    metadata: {
        content: "https://example.com/video.mp4",
        storageKey: "resource:video",
        status: "success",
        assetId: "video-asset",
        durationMs: 8000,
    },
}];

test("video stage renders prompt, playback, QC, versions and production actions", () => {
    const html = renderToStaticMarkup(
        <VideoStage
            production={production}
            nodes={nodes}
            tasks={{}}
            selectedDynastyId="tang"
            saving={false}
            feedback=""
            promptInspector={<div>最终英文视频提示词</div>}
            onSelectDynasty={() => undefined}
            onGenerateAll={async () => undefined}
            onGenerateDynasty={async () => undefined}
            onRunQc={async () => undefined}
            onSelectVersion={async () => undefined}
            onConfirm={async () => undefined}
            onOverrideQc={async () => undefined}
            onApprove={async () => undefined}
        />,
    );

    expect(html).toContain("最终英文视频提示词");
    expect(html).toContain("https://example.com/video.mp4");
    expect(html).toContain("https://example.com/male.png");
    expect(html).toContain("https://example.com/female.png");
    expect(html).toContain("https://example.com/scene.png");
    expect(html).toContain("自动 QC");
    expect(html).toContain("中段脸部漂移");
    expect(html).toContain("历史版本");
    expect(html).toContain("生成缺失视频");
    expect(html).toContain("确认视频阶段");
});
