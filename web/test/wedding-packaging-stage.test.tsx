import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { PackagingStage } from "@/pages/wedding/packaging-stage";
import type { CanvasNodeData } from "@/types/canvas";
import type { TimelineProject } from "@/types/timeline";

const production = createWeddingProductionState({
    template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
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
production.packaging.titleCardNodeIds = ["intro-node", "outro-node"];
production.packaging.bgmNodeId = "bgm-node";
production.packaging.timelineRevision = 1;
production.packaging.previewReadyAt = "2026-10-25T08:00:00.000Z";

const nodes: CanvasNodeData[] = [
    {
        id: "intro-node",
        type: "image",
        title: "片头标题卡",
        position: { x: 0, y: 0 },
        width: 1280,
        height: 720,
        metadata: { content: "https://example.com/intro.png", storageKey: "resource:intro" },
    },
    {
        id: "outro-node",
        type: "image",
        title: "片尾标题卡",
        position: { x: 0, y: 0 },
        width: 1280,
        height: 720,
        metadata: { content: "https://example.com/outro.png", storageKey: "resource:outro" },
    },
    {
        id: "bgm-node",
        type: "audio",
        title: "主题 BGM",
        position: { x: 0, y: 0 },
        width: 340,
        height: 220,
        metadata: { content: "https://example.com/bgm.mp3", storageKey: "resource:bgm", durationMs: 27000 },
    },
];

const timeline: TimelineProject = {
    version: 2,
    tracks: [
        { id: "video-1", kind: "video", label: "成片", order: 0 },
        { id: "audio-1", kind: "audio", label: "BGM", order: 1 },
        { id: "subtitle-1", kind: "subtitle", label: "字幕", order: 2 },
    ],
    clips: [
        {
            id: "clip-1",
            kind: "image",
            nodeId: "intro-node",
            trackId: "video-1",
            startMs: 0,
            durationMs: 4000,
            title: "片头",
            directMedia: { id: "intro-node", kind: "image", title: "片头", url: "https://example.com/intro.png", durationMs: 4000 },
        },
    ],
    durationMs: 4000,
};

test("packaging stage renders title cards, BGM, timeline preview and actions", () => {
    const html = renderToStaticMarkup(
        <PackagingStage
            production={production}
            nodes={nodes}
            timeline={timeline}
            saving={false}
            feedback=""
            bgmTask={undefined}
            onSave={async () => undefined}
            onGenerateTitles={async () => undefined}
            onGenerateBgm={async () => undefined}
            onUploadBgm={async () => undefined}
            onBuildTimeline={async () => undefined}
            onApprove={async () => undefined}
        />,
    );

    expect(html).toContain("片头文案");
    expect(html).toContain("片尾文案");
    expect(html).toContain("https://example.com/intro.png");
    expect(html).toContain("https://example.com/bgm.mp3");
    expect(html).toContain("时间线预览");
    expect(html).toContain("生成标题卡");
    expect(html).toContain("构建时间线");
    expect(html).toContain("确认包装阶段");
});
