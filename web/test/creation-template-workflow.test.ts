import { describe, expect, test } from "bun:test";

import {
    buildCanvasWorkflowFromTemplate,
    creationTemplateGenerationMetadata,
    parseCreationTemplatePayload,
    type PublicCreationTemplate,
} from "../src/lib/canvas/creation-template-workflow";
import { canOpenCanvasNodePromptPanel, canvasMediaNodeRole } from "../src/lib/canvas/canvas-node-semantics";
import { CanvasNodeType } from "../src/types/canvas";

const template: PublicCreationTemplate = {
    id: "template-1",
    itemKey: "wedding-workflow",
    name: "婚礼视频流程",
    description: "真实故事采集到成片交付",
    canvasType: "node",
    payloadJson: JSON.stringify({
        schema: "visideep.workflow-template/v1",
        version: "1.2.0",
        nodes: [
            {
                id: "W01",
                type: "workflow-step",
                title: "01 故事采集",
                x: 120,
                y: 80,
                w: 360,
                data: {
                    step: 1,
                    stepKey: "story_intake",
                    description: "收回新人故事和照片",
                    executionMode: "guided_human",
                    inputs: ["故事卡", "人物照片"],
                    actions: ["整理事实", "核实缺项"],
                    outputs: ["FACTS.md"],
                    approval: { roles: ["producer"], evidence: "制作方确认资料" },
                    reworkTargets: [],
                },
            },
            {
                id: "W02",
                type: "workflow-step",
                title: "02 文案初稿",
                x: 520,
                y: 80,
                w: 360,
                data: {
                    step: 2,
                    stepKey: "writing",
                    description: "生成完整写作包",
                    executors: ["assistant"],
                    inputs: ["FACTS.md"],
                    actions: ["生成写作包"],
                    outputs: ["SCRIPT.txt"],
                    approval: { roles: ["producer"], evidence: "制作方确认脚本" },
                    reworkTargets: ["W01"],
                },
            },
        ],
        edges: [
            { id: "E01", from: "W01", to: "W02", type: "next", label: "资料确认" },
            { id: "R02", from: "W02", to: "W01", type: "rework", label: "补充事实" },
        ],
    }),
};

describe("creation template workflow conversion", () => {
    test("turns backend workflow nodes into editable canvas text nodes and keeps the main path", () => {
        const converted = buildCanvasWorkflowFromTemplate(template, {
            idFactory: (kind, ref) => `${kind}-${ref}`,
            now: "2026-10-21T12:00:00.000Z",
        });

        expect(converted.nodes).toHaveLength(2);
        expect(converted.connections).toHaveLength(1);
        expect(converted.nodes[0]).toMatchObject({
            id: "node-W01",
            type: CanvasNodeType.Text,
            title: "01 故事采集",
            position: { x: 120, y: 80 },
            width: 360,
            createdAt: "2026-10-21T12:00:00.000Z",
        });
        expect(converted.nodes[0].metadata?.workflowKind).toBe("free");
        expect(converted.nodes[0].metadata?.pluginData).toMatchObject({
            creationTemplateId: "template-1",
            creationTemplateKey: "wedding-workflow",
            creationTemplateVersion: "1.2.0",
            creationTemplateNodeId: "W01",
            creationTemplateStepKey: "story_intake",
        });
        expect(converted.nodes[0].metadata?.content).toContain("输入：故事卡、人物照片");
        expect(converted.nodes[0].metadata?.content).toContain("确认：制作方确认资料");
        expect(converted.connections[0]).toMatchObject({
            id: "edge-E01",
            fromNodeId: "node-W01",
            toNodeId: "node-W02",
        });
    });

    test("rejects a template with broken JSON instead of creating an empty canvas", () => {
        expect(() => buildCanvasWorkflowFromTemplate({ ...template, payloadJson: "{" })).toThrow("流程定义 JSON 格式错误");
    });

    test("keeps real canvas node types from a v2 template graph", () => {
        const v2: PublicCreationTemplate = {
            ...template,
            itemKey: "wedding-v2",
            payloadJson: JSON.stringify({
                schema: "visideep.workflow-template/v2",
                version: "2.0.0",
                steps: Array.from({ length: 14 }, (_, index) => ({ id: `S${index + 1}`, title: `步骤 ${index + 1}` })),
                canvas: {
                    nodes: [
                        {
                            ref: "frame-1",
                            type: "frame",
                            title: "资料区",
                            x: 0,
                            y: 0,
                            width: 1200,
                            height: 800,
                            metadata: {
                                frame: { collapsed: false, expandedWidth: 1200, expandedHeight: 800 },
                            },
                        },
                        {
                            ref: "story-card",
                            type: "text",
                            title: "五幕故事卡",
                            parentId: "frame-1",
                            x: 80,
                            y: 80,
                            width: 420,
                            height: 600,
                            metadata: {
                                content: "第一幕·相识",
                                workflowKind: "story_input",
                                status: "success",
                            },
                        },
                        {
                            ref: "storyboard",
                            type: "script",
                            title: "婚礼分镜",
                            parentId: "frame-1",
                            x: 560,
                            y: 80,
                            width: 980,
                            height: 520,
                            metadata: {
                                workflowKind: "storyboard",
                                storyboard: {
                                    rows: [],
                                    visibleColumns: ["shotNumber", "durationSeconds", "imageGenerationPrompt", "videoMotionPrompt"],
                                    referenceNodeIds: [],
                                },
                            },
                        },
                    ],
                    connections: [
                        { id: "E01", from: "story-card", to: "storyboard", type: "next" },
                    ],
                },
            }),
        };

        const converted = buildCanvasWorkflowFromTemplate(v2, {
            idFactory: (kind, ref) => `${kind}-${ref}`,
            now: "2026-10-21T12:00:00.000Z",
        });

        expect(converted.nodes.map((node) => node.type)).toEqual([
            CanvasNodeType.Frame,
            CanvasNodeType.Text,
            CanvasNodeType.Script,
        ]);
        expect(converted.nodes[1].parentId).toBe("node-frame-1");
        expect(converted.nodes[2].metadata?.storyboard?.rows).toEqual([]);
        expect(converted.connections).toEqual([
            expect.objectContaining({ id: "edge-E01", fromNodeId: "node-story-card", toNodeId: "node-storyboard" }),
        ]);
        expect(parseCreationTemplatePayload(v2.payloadJson).stepCount).toBe(14);
    });

    test("fills template placeholders from the guided setup form", () => {
        const configured: PublicCreationTemplate = {
            ...template,
            itemKey: "configured-v2",
            payloadJson: JSON.stringify({
                schema: "visideep.workflow-template/v2",
                version: "2.1.0",
                forms: {
                    setup: {
                        steps: [
                            {
                                id: "spec",
                                title: "制作规格",
                                fields: [
                                    { key: "aspectRatio", type: "segmented", label: "画幅", options: ["16:9", "9:16"] },
                                ],
                            },
                        ],
                    },
                },
                canvas: {
                    nodes: [
                        {
                            ref: "spec",
                            type: "text",
                            title: "制作规格",
                            metadata: {
                                contentTemplate: "画幅：{{aspectRatio}}\n必须出现：{{mustInclude}}\n画风：{{visualStyle}}",
                                workflowKind: "styleboard",
                            },
                        },
                    ],
                    connections: [],
                },
            }),
        };

        const converted = buildCanvasWorkflowFromTemplate(configured, {
            variables: {
                aspectRatio: "9:16",
                mustInclude: ["宠物", "第一次见面的咖啡馆"],
                visualStyle: "写实电影感",
            },
        });

        expect(converted.nodes[0].metadata?.content).toBe("画幅：9:16\n必须出现：宠物、第一次见面的咖啡馆\n画风：写实电影感");
        expect(converted.nodes[0].metadata?.pluginData?.creationTemplateVariables).toEqual({
            aspectRatio: "9:16",
            mustInclude: ["宠物", "第一次见面的咖啡馆"],
            visualStyle: "写实电影感",
        });
        expect(parseCreationTemplatePayload(configured.payloadJson).setupForm?.steps[0].fields[0].key).toBe("aspectRatio");
    });

    test("expands dynamic dynasty nodes from the selected values", () => {
        const dynamic: PublicCreationTemplate = {
            ...template,
            itemKey: "cross-era",
            payloadJson: JSON.stringify({
                schema: "visideep.workflow-template/v2",
                version: "1.0.0",
                forms: {
                    setup: {
                        steps: [{
                            id: "dynasty",
                            title: "选择朝代",
                            fields: [{
                                key: "dynasties",
                                type: "checkboxGroup",
                                label: "朝代",
                                minSelections: 2,
                                maxSelections: 5,
                                options: [
                                    { label: "唐 · 大唐盛世", value: "tang" },
                                    { label: "明 · 凤冠霞帔", value: "ming" },
                                ],
                            }],
                        }],
                    },
                },
                canvas: {
                    nodes: [
                        { ref: "groom", type: "image", title: "新郎照片", x: 0, y: 0, width: 400, height: 300, metadata: { status: "idle" } },
                        { ref: "era-frame", type: "frame", title: "朝代资产", x: 600, y: 0, width: 1800, height: 1200, metadata: { frame: { collapsed: false, expandedWidth: 1800, expandedHeight: 1200 } } },
                    ],
                    dynamicNodes: [
                        {
                            ref: "dynastyPortraitMale",
                            type: "image",
                            per: "dynasty",
                            titleTemplate: "{{dynasty.name}} · 新郎肖像",
                            parentId: "era-frame",
                            width: 420,
                            height: 280,
                            layout: { x: 700, y: 180, columnWidth: 460 },
                            dependsOn: ["groom"],
                            metadata: {
                                generationMode: "image",
                            },
                        },
                    ],
                    connections: [],
                },
            }),
        };

        const converted = buildCanvasWorkflowFromTemplate(dynamic, {
            variables: { dynasties: ["tang", "ming"] },
            idFactory: (kind, ref) => `${kind}-${ref}`,
        });

        const portraits = converted.nodes.filter((node) => node.title.endsWith("新郎肖像"));
        expect(portraits).toHaveLength(2);
        expect(portraits.map((node) => node.title)).toEqual(["唐 · 新郎肖像", "明 · 新郎肖像"]);
        expect(portraits[0].parentId).toBe("node-era-frame");
        expect(portraits[0].metadata?.prompt).toBeUndefined();
        expect(portraits[0].metadata?.pluginData).toMatchObject({
            creationTemplateNodeId: "dynastyPortraitMale",
            creationTemplateDynastyId: "tang",
        });
        expect(portraits[0].metadata?.nodeRole).toBe("generator");
        expect(portraits[0].metadata?.resultOrigin).toBeUndefined();
        expect(canOpenCanvasNodePromptPanel(portraits[0])).toBe(true);
        expect(portraits[1].position.x).toBe(1160);
        expect(converted.connections).toHaveLength(2);
        expect(converted.connections[0]).toMatchObject({
            fromNodeId: "node-groom",
            toNodeId: "node-dynastyPortraitMale-tang",
        });
        expect(parseCreationTemplatePayload(dynamic.payloadJson).setupForm?.steps[0].fields[0]).toMatchObject({
            minSelections: 2,
            maxSelections: 5,
        });
    });

    test("submits only private prompt references instead of prompt text", () => {
        const metadata = creationTemplateGenerationMetadata({
            pluginData: {
                creationTemplateId: "template-1",
                creationTemplateKey: "cross-era",
                creationTemplateNodeId: "dynastyPortraitMale",
                creationTemplateDynamicRef: "dynastyPortraitMale",
                creationTemplateDynastyId: "tang",
                creationTemplateVariables: {
                    dynasties: ["tang"],
                    duration: 8,
                    ratio: "16:9",
                    customDesc: "杭州西湖相识",
                },
                promptSourceKeys: ["must-not-leak"],
            },
            prompt: "must-not-leak",
        });

        expect(metadata).toEqual({
            creationTemplate: {
                id: "template-1",
                key: "cross-era",
                nodeRef: "dynastyPortraitMale",
                dynastyId: "tang",
                variables: {
                    dynasties: ["tang"],
                    duration: 8,
                    ratio: "16:9",
                    customDesc: "杭州西湖相识",
                },
            },
        });
        expect(JSON.stringify(metadata)).not.toContain("must-not-leak");
    });

    test("recovers legacy empty workflow media nodes as generators", () => {
        expect(canvasMediaNodeRole({
            id: "legacy-dynamic-image",
            type: CanvasNodeType.Image,
            title: "唐 · 新郎肖像",
            position: { x: 0, y: 0 },
            width: 420,
            height: 280,
            metadata: {
                nodeRole: "result",
                resultOrigin: "generated",
                pluginData: { creationTemplateDynamicRef: "dynastyPortraitMale" },
            },
        })).toBe("generator");
    });

    test("writes uploaded setup images into their matching source nodes", () => {
        const uploadTemplate: PublicCreationTemplate = {
            ...template,
            itemKey: "cross-era-upload",
            payloadJson: JSON.stringify({
                schema: "visideep.workflow-template/v2",
                version: "1.0.0",
                forms: {
                    setup: {
                        steps: [{
                            id: "source",
                            title: "人物",
                            fields: [{
                                key: "malePhoto",
                                type: "image",
                                label: "新郎照片",
                                required: true,
                                uploadKind: "image",
                                accept: "image/*",
                            }],
                        }],
                    },
                },
                canvas: {
                    nodes: [{
                        ref: "groomPhoto",
                        type: "image",
                        title: "新郎原始照片",
                        metadata: { uploadField: "malePhoto", status: "idle" },
                    }],
                    connections: [],
                },
            }),
        };
        const converted = buildCanvasWorkflowFromTemplate(uploadTemplate, {
            variables: {
                malePhoto: {
                    assetId: "asset-1",
                    resourceId: "resource-1",
                    storageKey: "resource:resource-1",
                    url: "/api/resources/resource-1/file",
                    mimeType: "image/jpeg",
                    width: 1200,
                    height: 1600,
                },
            },
            idFactory: (kind, ref) => `${kind}-${ref}`,
        });

        expect(converted.nodes[0].metadata).toMatchObject({
            content: "/api/resources/resource-1/file",
            assetId: "asset-1",
            storageKey: "resource:resource-1",
            mimeType: "image/jpeg",
            naturalWidth: 1200,
            naturalHeight: 1600,
            status: "success",
        });
        expect(parseCreationTemplatePayload(uploadTemplate.payloadJson).setupForm?.steps[0].fields[0]).toMatchObject({
            type: "image",
            uploadKind: "image",
            accept: "image/*",
        });
    });
});
