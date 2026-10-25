import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { createWeddingProductionState } from "@/lib/wedding/production-state";
import { SetupStage } from "@/pages/wedding/setup-stage";

test("setup stage renders editable project fields and stage actions", () => {
    const state = createWeddingProductionState({
        template: { id: "template-1", key: "cross-era-wedding-volc", version: "1" },
        source: {
            groomPhotoNodeId: "groom",
            bridePhotoNodeId: "bride",
            maleName: "林屿",
            femaleName: "周棠",
            story: "杭州西湖相识",
            consentConfirmed: false,
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

    const html = renderToStaticMarkup(
        <SetupStage
            production={state}
            saving={false}
            feedback=""
            onSave={async () => undefined}
            onApprove={async () => undefined}
        />,
    );

    expect(html).toContain("新郎称呼");
    expect(html).toContain("爱情故事");
    expect(html).toContain("照片使用授权");
    expect(html).toContain("上传新郎照片");
    expect(html).toContain("上传新娘照片");
    expect(html).toContain('type="file"');
    expect(html).toContain("确认项目设定");
});
