import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { DeliveryStage } from "@/pages/wedding/delivery-stage";
import type { GenerationTask } from "@/services/api/task-center";

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
production.delivery = {
    status: "needs_review",
    taskId: "render-task",
    resourceId: "resource:film",
    fileName: "cross-era-wedding.mp4",
    durationMs: 27000,
    size: 123456,
};
const renderTask: GenerationTask = {
    id: "render-task",
    type: "timeline_render",
    status: "succeeded",
    progress: 100,
    prompt: "timeline",
    attempts: 1,
    createdAt: "2026-10-25T09:00:00.000Z",
    updatedAt: "2026-10-25T09:10:00.000Z",
};

test("delivery stage renders final video, verification, download and manifest actions", () => {
    const html = renderToStaticMarkup(
        <DeliveryStage
            production={production}
            renderTask={renderTask}
            saving={false}
            feedback=""
            onRender={async () => undefined}
            onVerify={async () => undefined}
            onConfirm={async () => undefined}
            onExportManifest={() => undefined}
        />,
    );

    expect(html).toContain("正式成片");
    expect(html).toContain("渲染成片");
    expect(html).toContain("验证可播放");
    expect(html).toContain("下载 MP4");
    expect(html).toContain("导出工程清单");
    expect(html).toContain("cross-era-wedding.mp4");
    expect(html).toContain("resource:film");
});
