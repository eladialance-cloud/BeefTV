import { nanoid } from "nanoid";

import { batchReferenceHandleId, createBatchRow } from "@/lib/canvas/canvas-batch-table";
import { createCanvasNode } from "@/lib/canvas/canvas-project-domain";
import { CanvasNodeType, type CanvasConnection, type CanvasBatchTableData, type Position } from "@/types/canvas";

const PRODUCT_BRIEF_PROMPT = ["商品名称：", "核心卖点：", "目标人群：", "品牌调性：", "必须出现：", "禁止出现："].join("\n");

const VISUAL_BRIEF_PROMPT = ["主图方向：干净背景，商品居中，轮廓和材质清晰。", "场景方向：真实使用环境，主体不被背景抢走。", "详情方向：突出材质、工艺和结构细节。", "社媒方向：生活化、可种草，构图适合竖版封面。"].join("\n");

const MAIN_IMAGE_PROMPT = "保持商品主体、颜色、材质和关键细节一致，生成干净背景的商品主图。主体居中，轮廓清晰，柔和均匀布光，商业电商摄影，高清，无文字水印。";
const DETAIL_IMAGE_PROMPT = "基于商品参考图生成细节特写，突出材质纹理、工艺结构、边缘和关键卖点。微距摄影，浅景深，高光柔和，真实材质，高清商业图。";
const SCENE_IMAGE_PROMPT = "保持商品主体和关键细节不变，把商品自然放入真实使用场景。环境与商品调性一致，光影自然，背景有层次但不抢主体，商业广告摄影。";
const SOCIAL_IMAGE_PROMPT = "保持商品主体一致，生成适合社媒种草的商业图片。生活化构图，氛围自然，留出标题空间，画面干净，有真实使用感，无文字水印。";
const VIDEO_PROMPT = "以商品主图为起始画面，生成一条 5 秒商品展示视频。镜头缓慢推进并轻微环绕，突出商品材质、轮廓和核心卖点，运动平稳，商品结构不形变，光影连续。";

export type CanvasEcommercePipeline = {
    nodes: ReturnType<typeof createCanvasNode>[];
    connections: CanvasConnection[];
    productBriefNodeId: string;
    productImageNodeId: string;
    styleReferenceNodeId: string;
    visualBriefNodeId: string;
    mainImageNodeId: string;
    detailImageNodeId: string;
    sceneImageNodeId: string;
    socialImageNodeId: string;
    videoNodeId: string;
    batchTableNodeId: string;
};

export function createEcommercePipeline(center: Position): CanvasEcommercePipeline {
    const productBrief = createCanvasNode(
        CanvasNodeType.Text,
        { x: center.x - 1180, y: center.y - 520 },
        {
            content: PRODUCT_BRIEF_PROMPT,
            status: "success",
            fontSize: 14,
            workflowTitle: "商品资料",
            workflowDescription: "填写商品名称、卖点、人群和禁用项",
        },
    );
    productBrief.title = "商品资料";

    const productImage = createCanvasNode(
        CanvasNodeType.Image,
        { x: center.x - 1180, y: center.y + 40 },
        {
            content: "",
            status: "idle",
            workflowKind: "ecommerce_product",
            workflowTitle: "商品参考图",
            workflowDescription: "上传商品图后作为整条流程的参考",
        },
    );
    productImage.title = "商品参考图 · 待上传";

    const visualBrief = createCanvasNode(
        CanvasNodeType.Text,
        { x: center.x - 1180, y: center.y + 620 },
        {
            content: VISUAL_BRIEF_PROMPT,
            status: "success",
            fontSize: 14,
            workflowTitle: "视觉方向",
            workflowDescription: "主图、场景、细节和社媒方向",
        },
    );
    visualBrief.title = "视觉方向";

    const styleReference = createCanvasNode(
        CanvasNodeType.Image,
        { x: center.x - 1180, y: center.y + 1180 },
        {
            content: "",
            status: "idle",
            workflowKind: "ecommerce_scene",
            workflowTitle: "风格参考图",
            workflowDescription: "可选，用于批量表统一风格",
        },
    );
    styleReference.title = "风格参考图 · 可选";

    const mainImage = createEcommerceImageNode(center.x - 60, center.y - 460, "商品主图", MAIN_IMAGE_PROMPT, "ecommerce_main");
    const detailImage = createEcommerceImageNode(center.x + 820, center.y - 460, "细节图", DETAIL_IMAGE_PROMPT, "ecommerce_detail");
    const sceneImage = createEcommerceImageNode(center.x - 60, center.y + 180, "场景图", SCENE_IMAGE_PROMPT, "ecommerce_scene");
    const socialImage = createEcommerceImageNode(center.x + 820, center.y + 180, "社媒种草图", SOCIAL_IMAGE_PROMPT, "ecommerce_social");

    const videoNode = createCanvasNode(
        CanvasNodeType.Video,
        { x: center.x + 1700, y: center.y - 140 },
        {
            content: "",
            status: "idle",
            prompt: VIDEO_PROMPT,
            composerContent: VIDEO_PROMPT,
            generationMode: "video",
            videoEditOperation: "image_to_video",
            videoStartFrameNodeId: mainImage.id,
            workflowKind: "ecommerce_video",
            workflowTitle: "商品展示视频",
            workflowDescription: "以商品主图为首帧生成 5 秒展示视频",
            seconds: "5",
        },
    );
    videoNode.title = "商品展示视频 · 5 秒";

    const batchTable = createCanvasNode(
        CanvasNodeType.BatchTable,
        { x: center.x + 260, y: center.y + 1120 },
        {
            status: "idle",
            workflowKind: "ecommerce_batch",
            workflowTitle: "电商批量出图",
            workflowDescription: "按商品图和风格参考批量生成商业图片",
            batchTable: createEcommerceBatchTable(productImage.id, styleReference.id),
        },
    );
    batchTable.title = "电商批量出图";

    const prompts = [productBrief, visualBrief];
    const outputs = [mainImage, detailImage, sceneImage, socialImage, videoNode];
    const connections: CanvasConnection[] = outputs.flatMap((output) => [...prompts.map((promptNode) => connection(promptNode.id, output.id)), connection(productImage.id, output.id)]);
    connections.push(connection(productImage.id, batchTable.id, batchReferenceHandleId("product")), connection(styleReference.id, batchTable.id, batchReferenceHandleId("style")), connection(mainImage.id, videoNode.id));

    return {
        nodes: [productBrief, productImage, visualBrief, styleReference, mainImage, detailImage, sceneImage, socialImage, videoNode, batchTable],
        connections,
        productBriefNodeId: productBrief.id,
        productImageNodeId: productImage.id,
        styleReferenceNodeId: styleReference.id,
        visualBriefNodeId: visualBrief.id,
        mainImageNodeId: mainImage.id,
        detailImageNodeId: detailImage.id,
        sceneImageNodeId: sceneImage.id,
        socialImageNodeId: socialImage.id,
        videoNodeId: videoNode.id,
        batchTableNodeId: batchTable.id,
    };
}

function createEcommerceImageNode(x: number, y: number, title: string, prompt: string, workflowKind: "ecommerce_main" | "ecommerce_detail" | "ecommerce_scene" | "ecommerce_social") {
    const node = createCanvasNode(
        CanvasNodeType.Image,
        { x, y },
        {
            content: "",
            status: "idle",
            prompt,
            composerContent: prompt,
            generationMode: "image",
            workflowKind,
            workflowTitle: title,
            workflowDescription: prompt,
        },
    );
    node.title = title;
    return node;
}

function createEcommerceBatchTable(productImageNodeId: string, styleReferenceNodeId: string): CanvasBatchTableData {
    const prompts = [
        "生成干净背景的商品主图，商品居中，保留商品颜色、材质和结构，弱化阴影，适合电商首图。",
        "生成真实使用场景图，商品与场景自然融合，光影合理，背景有层次但不抢主体。",
        "生成材质和工艺细节特写，突出纹理、边缘、结构和关键卖点，微距商业摄影。",
        "生成社媒种草图，生活化构图，氛围自然，适合竖版封面，留出标题空间。",
    ];
    return {
        operation: "creative",
        concurrency: 5,
        referenceColumns: [
            { id: "product", label: "商品参考" },
            { id: "style", label: "风格参考" },
        ],
        rows: prompts.map((prompt, index) => {
            const row = createBatchRow("creative", [productImageNodeId, styleReferenceNodeId]);
            return { ...row, id: `ecommerce-shot-${index + 1}-${nanoid()}`, prompt, enabled: true };
        }),
    };
}

function connection(fromNodeId: string, toNodeId: string, toHandleId?: string): CanvasConnection {
    return { id: nanoid(), fromNodeId, toNodeId, toHandleId };
}
