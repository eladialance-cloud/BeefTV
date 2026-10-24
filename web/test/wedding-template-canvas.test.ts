import { expect, mock, test } from "bun:test";

test("cross-era template creation includes persisted wedding production state", async () => {
    let captured: Record<string, unknown> | undefined;
    mock.module("@/services/workspace-project-repository", () => ({
        createWorkspaceCanvasProject: async (
            _title: string,
            _projectId: string | undefined,
            initialContent: Record<string, unknown>,
        ) => {
            captured = initialContent;
            return { id: "canvas-1" };
        },
    }));
    const { createWorkspaceCanvasFromTemplate } = await import("@/services/workflow-template-canvas");
    const template = {
        id: "template-1",
        itemKey: "cross-era-wedding-volc",
        name: "跨时空婚礼电影",
        payloadJson: JSON.stringify({
            schema: "visideep.workflow-template/v2",
            version: "1.0.0",
            forms: {
                setup: {
                    steps: [{
                        id: "dynasties",
                        title: "朝代",
                        fields: [{
                            key: "dynasties",
                            type: "checkboxGroup",
                            label: "朝代",
                            options: [{ label: "唐 · 大唐盛世", value: "tang" }],
                        }],
                    }],
                },
            },
            canvas: {
                nodes: [
                    { ref: "groomPhoto", type: "image", title: "新郎照片", metadata: { uploadField: "groomPhoto" } },
                    { ref: "bridePhoto", type: "image", title: "新娘照片", metadata: { uploadField: "bridePhoto" } },
                ],
                connections: [],
                dynamicNodes: [
                    { ref: "dynastyPortraitMale", type: "image", titleTemplate: "{{dynasty.name}} · 新郎", metadata: {} },
                    { ref: "dynastyPortraitFemale", type: "image", titleTemplate: "{{dynasty.name}} · 新娘", metadata: {} },
                    { ref: "dynastyScene", type: "image", titleTemplate: "{{dynasty.name}} · 场景", metadata: {} },
                    { ref: "dynastyVideo", type: "video", titleTemplate: "{{dynasty.name}} · 视频", metadata: {} },
                ],
            },
        }),
    };

    await createWorkspaceCanvasFromTemplate(template, {
        dynasties: ["tang"],
        ratio: "16:9",
        resolution: "720p",
        consentConfirmed: true,
    });

    expect(captured?.weddingProduction).toMatchObject({
        template: { id: "template-1", key: "cross-era-wedding-volc", version: "1.0.0" },
        dynasties: [{ id: "tang", name: "唐" }],
    });
});
