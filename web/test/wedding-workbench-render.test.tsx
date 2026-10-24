import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { WorkbenchStageRail } from "@/pages/wedding/workbench-stage-rail";

const state = createWeddingProductionState({
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

test("renders all seven producer stages", () => {
    const html = renderToStaticMarkup(
        <WorkbenchStageRail state={state} activeStage="videos" onSelect={() => undefined} />,
    );

    expect(html.indexOf("项目设定")).toBeLessThan(html.indexOf("朝代规划"));
    expect(html.indexOf("朝代规划")).toBeLessThan(html.indexOf("定妆照"));
    expect(html.indexOf("定妆照")).toBeLessThan(html.indexOf("场景首帧"));
    expect(html.indexOf("场景首帧")).toBeLessThan(html.indexOf("视频导演"));
    expect(html.indexOf("视频导演")).toBeLessThan(html.indexOf("包装制作"));
    expect(html.indexOf("包装制作")).toBeLessThan(html.indexOf("成片交付"));
    expect(html).toContain('aria-current="step"');
});
