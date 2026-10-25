import type { PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";
import { createWeddingProductionState } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";

type TemplateNode = {
    id: string;
    metadata?: {
        uploadField?: string;
        pluginData?: Record<string, unknown>;
    };
};

type TemplateVariables = {
    maleName?: unknown;
    femaleName?: unknown;
    customDesc?: unknown;
    dynasties?: unknown;
    ratio?: unknown;
    resolution?: unknown;
    consentConfirmed?: unknown;
};

export function buildWeddingProductionFromTemplate(
    template: Pick<PublicCreationTemplate, "id" | "itemKey" | "name" | "payloadJson">,
    variables: TemplateVariables,
    nodes: TemplateNode[],
    now: string,
): WeddingProductionState {
    return buildWeddingProductionState({
        template: {
            id: template.id,
            key: template.itemKey,
            version: templateVersion(template.payloadJson),
        },
        variables,
        nodes,
        now,
        dynastyNames: dynastyNamesFromPayload(template.payloadJson),
    });
}

export function buildWeddingProductionFromCanvasNodes(
    nodes: TemplateNode[],
    now: string,
): WeddingProductionState | undefined {
    const templateNode = nodes.find((node) => {
        const pluginData = asRecord(node.metadata?.pluginData);
        return text(pluginData?.creationTemplateKey) === "cross-era-wedding-volc";
    });
    if (!templateNode) return undefined;

    const pluginData = asRecord(templateNode.metadata?.pluginData);
    const templateID = text(pluginData?.creationTemplateId) || "cross-era-wedding-volc";
    const variables = asRecord(pluginData?.creationTemplateVariables) || {};
    return buildWeddingProductionState({
        template: {
            id: templateID,
            key: "cross-era-wedding-volc",
            version: text(pluginData?.creationTemplateVersion) || "1.0.0",
        },
        variables,
        nodes,
        now,
        dynastyNames: new Map(),
    });
}

function buildWeddingProductionState(input: {
    template: WeddingProductionState["template"];
    variables: TemplateVariables;
    nodes: TemplateNode[];
    now: string;
    dynastyNames: Map<string, string>;
}): WeddingProductionState {
    const { template, variables, nodes, now, dynastyNames } = input;
    const groomPhoto = nodes.find((node) => node.metadata?.uploadField === "groomPhoto");
    const bridePhoto = nodes.find((node) => node.metadata?.uploadField === "bridePhoto");
    if (!groomPhoto) throw new Error("缺少新郎原始照片节点");
    if (!bridePhoto) throw new Error("缺少新娘原始照片节点");

    const selectedDynasties = Array.isArray(variables.dynasties)
        ? variables.dynasties.map((value) => text(value)).filter(Boolean)
        : [];
    const dynamicNodes = nodes.filter((node) => {
        const pluginData = asRecord(node.metadata?.pluginData);
        return text(pluginData?.creationTemplateDynamicRef) && text(pluginData?.creationTemplateDynastyId);
    });
    const dynasties = selectedDynasties.flatMap((dynastyId, index) => {
        const dynastyNodes = dynamicNodes.filter((node) => {
            const pluginData = asRecord(node.metadata?.pluginData);
            return text(pluginData?.creationTemplateDynastyId) === dynastyId;
        });
        const findNode = (ref: string, field: string) => {
            const node = dynastyNodes.find((candidate) => {
                const pluginData = asRecord(candidate.metadata?.pluginData);
                return text(pluginData?.creationTemplateDynamicRef) === ref;
            });
            if (node) return node;
            const name = dynastyName(dynastyNodes) || dynastyNames.get(dynastyId) || dynastyId;
            throw new Error(`${name}：缺少${field}节点`);
        };
        const portraitMale = findNode("dynastyPortraitMale", "新郎定妆照");
        const portraitFemale = findNode("dynastyPortraitFemale", "新娘定妆照");
        const scene = findNode("dynastyScene", "场景首帧");
        const video = findNode("dynastyVideo", "视频");
        return [{
            id: dynastyId,
            name: dynastyName(dynastyNodes) || dynastyNames.get(dynastyId) || dynastyId,
            order: index,
            portraitMaleNodeId: portraitMale.id,
            portraitFemaleNodeId: portraitFemale.id,
            sceneNodeId: scene.id,
            videoNodeId: video.id,
        }];
    });

    return createWeddingProductionState({
        template,
        source: {
            groomPhotoNodeId: groomPhoto.id,
            bridePhotoNodeId: bridePhoto.id,
            maleName: text(variables.maleName) || "新郎",
            femaleName: text(variables.femaleName) || "新娘",
            story: text(variables.customDesc),
            consentConfirmed: variables.consentConfirmed === true,
            ratio: text(variables.ratio) || "16:9",
            resolution: text(variables.resolution) || "720p",
        },
        dynasties,
        now,
    });
}

function dynastyName(nodes: TemplateNode[]) {
    for (const node of nodes) {
        const pluginData = asRecord(node.metadata?.pluginData);
        const name = text(pluginData?.creationTemplateDynastyName);
        if (name) return name;
    }
    return "";
}

function templateVersion(payloadJson: string) {
    try {
        const payload = asRecord(JSON.parse(payloadJson));
        return text(payload?.version) || "1";
    } catch {
        return "1";
    }
}

function dynastyNamesFromPayload(payloadJson: string) {
    const result = new Map<string, string>();
    try {
        const payload = asRecord(JSON.parse(payloadJson));
        const setup = asRecord(asRecord(payload?.forms)?.setup);
        for (const rawStep of array(setup?.steps)) {
            const fields = array(asRecord(rawStep)?.fields);
            for (const rawField of fields) {
                const field = asRecord(rawField);
                if (text(field?.key) !== "dynasties") continue;
                for (const rawOption of array(field?.options)) {
                    const option = asRecord(rawOption);
                    const value = text(option?.value);
                    const label = text(option?.label);
                    if (!value || !label) continue;
                    result.set(value, label.split("·")[0]?.trim() || label);
                }
            }
        }
    } catch {
        // Invalid payloads are reported by creator validation; name fallback stays deterministic.
    }
    return result;
}

function text(value: unknown) {
    return typeof value === "string" ? value.trim() : "";
}

function asRecord(value: unknown) {
    return value && typeof value === "object" && !Array.isArray(value)
        ? value as Record<string, unknown>
        : null;
}

function array(value: unknown) {
    return Array.isArray(value) ? value : [];
}
