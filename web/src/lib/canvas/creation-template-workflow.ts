import { nanoid } from "nanoid";

import { mediaGeneratorMetadata } from "@/lib/canvas/canvas-node-semantics";
import { CanvasNodeType, type CanvasConnection, type CanvasNodeData, type CanvasNodeMetadata } from "@/types/canvas";

export type PublicCreationTemplate = {
    id: string;
    itemKey: string;
    name: string;
    description?: string;
    canvasType?: string;
    defaultModel?: string;
    payloadJson: string;
};

export type WorkflowFormOption = {
    label: string;
    value: string | number;
    description?: string;
};

export type WorkflowFormField = {
    key: string;
    type: "segmented" | "select" | "cards" | "tags" | "text" | "textarea" | "checkboxGroup" | "image";
    label: string;
    description?: string;
    placeholder?: string;
    required?: boolean;
    options?: WorkflowFormOption[];
    defaultValue?: unknown;
    minSelections?: number;
    maxSelections?: number;
    uploadKind?: "image";
    accept?: string;
};

export type WorkflowFormStep = {
    id: string;
    title: string;
    description?: string;
    fields: WorkflowFormField[];
};

export type WorkflowSetupForm = {
    enabled: boolean;
    title?: string;
    description?: string;
    steps: WorkflowFormStep[];
};

type TemplateNode = {
    id: string;
    type?: string;
    title: string;
    parentId?: string;
    x?: number;
    y?: number;
    w?: number;
    h?: number;
    data?: Record<string, unknown>;
    metadata?: Record<string, unknown>;
};

type TemplateEdge = {
    id?: string;
    from: string;
    to: string;
    type?: string;
    label?: string;
    fromHandleId?: string;
    toHandleId?: string;
    relation?: CanvasConnection["relation"];
};

type TemplateDynamicNode = {
    ref: string;
    type?: string;
    titleTemplate: string;
    parentId?: string;
    width?: number;
    height?: number;
    layout?: {
        x: number;
        y: number;
        columnWidth?: number;
    };
    dependsOn?: string[];
    connectsTo?: string[];
    metadata?: Record<string, unknown>;
};

type TemplatePayload = {
    schema?: string;
    version?: string | number;
    stepCount?: number;
    setupForm?: WorkflowSetupForm;
    nodes: TemplateNode[];
    edges?: TemplateEdge[];
    dynamicNodes: TemplateDynamicNode[];
    dynastyCatalog: Array<Record<string, unknown>>;
};

type ConversionOptions = {
    idFactory?: (kind: "node" | "edge", ref: string) => string;
    now?: string;
    variables?: Record<string, unknown>;
};

export function creationTemplateGenerationMetadata(metadata?: CanvasNodeMetadata) {
    const pluginData = asRecord(metadata?.pluginData);
    const id = stringValue(pluginData?.creationTemplateId);
    const key = stringValue(pluginData?.creationTemplateKey);
    const nodeRef = stringValue(pluginData?.creationTemplateDynamicRef) || stringValue(pluginData?.creationTemplateNodeId);
    if (!id || !nodeRef) return {};
    return {
        creationTemplate: {
            id,
            ...(key ? { key } : {}),
            nodeRef,
            ...(stringValue(pluginData?.creationTemplateDynastyId)
                ? { dynastyId: stringValue(pluginData?.creationTemplateDynastyId) }
                : {}),
            variables: safeCreationTemplateVariables(pluginData?.creationTemplateVariables),
        },
    };
}

/**
 * Converts the generic workflow payload stored by the admin console into the
 * canvas document format. Unknown step subtypes remain editable text nodes so a
 * new catalog template does not require a frontend release.
 */
export function buildCanvasWorkflowFromTemplate(template: PublicCreationTemplate, options: ConversionOptions = {}) {
    const payload = parseCreationTemplatePayload(template.payloadJson);
    const now = options.now || new Date().toISOString();
    const idFactory = options.idFactory || ((kind: "node" | "edge", ref: string) => `${kind}-${slug(ref)}-${nanoid(8)}`);
    const nodeIds = new Map<string, string>();
    payload.nodes.forEach((node, index) => {
        const templateNodeId = requiredText(node.id, `第 ${index + 1} 个流程节点缺少 id`);
        if (nodeIds.has(templateNodeId)) throw new Error(`流程节点 id 重复：${templateNodeId}`);
        nodeIds.set(templateNodeId, idFactory("node", templateNodeId));
    });
    const nodes: CanvasNodeData[] = payload.nodes.map((node, index) => {
        const templateNodeId = node.id;
        const id = nodeIds.get(templateNodeId)!;
        const title = requiredText(node.title, `流程节点 ${templateNodeId} 缺少标题`);
        const data = resolveTemplateValues(node.metadata || node.data || {}, options.variables || {});
        applyUploadedTemplateAsset(data, options.variables || {});
        const contentTemplate = stringValue(data.contentTemplate);
        if (contentTemplate) data.content = contentTemplate;
        const nodeType = canvasNodeType(node.type);
        const width = positiveNumber(node.w ?? data.width, nodeType === CanvasNodeType.Frame ? 1200 : 340);
        const height = positiveNumber(node.h ?? data.height, nodeType === CanvasNodeType.Frame ? 800 : 220);
        const position = {
            x: finiteNumber(node.x, 80),
            y: finiteNumber(node.y, 80 + index * 260),
        };
        const metadata = normalizeCanvasMetadata(nodeType, title, template, data);
        const pluginData = {
            ...(asRecord(metadata.pluginData) || {}),
            creationTemplateId: template.id,
            creationTemplateKey: template.itemKey,
            creationTemplateName: template.name,
            creationTemplateVersion: String(payload.version || "1"),
            creationTemplateSchema: payload.schema || "visideep.workflow-template/v1",
            creationTemplateNodeId: templateNodeId,
            creationTemplateNodeType: node.type || "workflow-step",
            creationTemplateStepKey: stringValue(data.stepKey),
            creationTemplateStep: numberValue(data.step),
            creationTemplateExecutionMode: stringValue(data.executionMode),
            creationTemplateApproval: data.approval,
            creationTemplateReworkTargets: stringArray(data.reworkTargets),
            creationTemplateVariables: options.variables || {},
        };
        metadata.pluginData = pluginData;
        return {
            id,
            type: nodeType,
            title,
            createdAt: now,
            updatedAt: now,
            position,
            width,
            height,
            ...(node.parentId && nodeIds.has(node.parentId) ? { parentId: nodeIds.get(node.parentId) } : {}),
            metadata,
        } satisfies CanvasNodeData;
    });
    const dynamicTemplates = new Map(payload.dynamicNodes.map((node) => [node.ref, node]));
    const dynastyValues = Array.isArray(options.variables?.dynasties) ? options.variables.dynasties : [];
    const dynastyDisplay = dynastyDisplayFromForm(payload.setupForm, dynastyValues);
    const selectedDynasties = dynastyValues
        .map((id) => dynastyDisplay.get(String(id)))
        .filter((dynasty): dynasty is Record<string, unknown> => Boolean(dynasty));
    const dynamicNodeIds = new Map<string, string>();
    selectedDynasties.forEach((dynasty, dynastyIndex) => {
        for (const templateNode of payload.dynamicNodes) {
            const dynastyId = stringValue(dynasty.id);
            const instanceRef = `${templateNode.ref}-${dynastyId}`;
            const id = idFactory("node", instanceRef);
            dynamicNodeIds.set(`${templateNode.ref}\0${dynastyId}`, id);
            const context = { ...(options.variables || {}), dynasty };
            const data = resolveTemplateValues(templateNode.metadata || {}, context);
            const contentTemplate = stringValue(data.contentTemplate);
            if (contentTemplate) data.content = contentTemplate;
            const title = resolveTemplateString(templateNode.titleTemplate, context);
            const nodeType = canvasNodeType(templateNode.type);
            const normalizedMetadata = normalizeCanvasMetadata(nodeType, title, template, data);
            const metadata = nodeType === CanvasNodeType.Image
                || nodeType === CanvasNodeType.Video
                || nodeType === CanvasNodeType.Audio
                ? mediaGeneratorMetadata(normalizedMetadata)
                : normalizedMetadata;
            metadata.pluginData = {
                ...(asRecord(metadata.pluginData) || {}),
                creationTemplateId: template.id,
                creationTemplateKey: template.itemKey,
                creationTemplateVersion: String(payload.version || "1"),
                creationTemplateNodeId: templateNode.ref,
                creationTemplateDynamicRef: templateNode.ref,
                creationTemplateDynastyId: dynastyId,
                creationTemplateDynastyName: stringValue(dynasty.name),
                creationTemplateVariables: options.variables || {},
            };
            const layout = templateNode.layout || { x: 2200, y: 700 + dynastyIndex * 360 };
            const position = {
                x: layout.x + dynastyIndex * (layout.columnWidth || 560),
                y: layout.y,
            };
            nodes.push({
                id,
                type: nodeType,
                title,
                createdAt: now,
                updatedAt: now,
                position,
                width: positiveNumber(templateNode.width, 420),
                height: positiveNumber(templateNode.height, 280),
                ...(templateNode.parentId && nodeIds.has(templateNode.parentId) ? { parentId: nodeIds.get(templateNode.parentId) } : {}),
                metadata,
            });
        }
    });

    const seenEdges = new Set<string>();
    const connections: CanvasConnection[] = [];
    for (const edge of payload.edges || []) {
        if (edge.type === "rework") continue;
        const fromNodeId = nodeIds.get(edge.from);
        const toNodeId = nodeIds.get(edge.to);
        if (!fromNodeId || !toNodeId || fromNodeId === toNodeId) continue;
        const key = `${fromNodeId}\0${toNodeId}\0${edge.fromHandleId || ""}\0${edge.toHandleId || ""}`;
        if (seenEdges.has(key)) continue;
        seenEdges.add(key);
        const edgeRef = edge.id || `${edge.from}-${edge.to}`;
        connections.push({
            id: idFactory("edge", edgeRef),
            fromNodeId,
            toNodeId,
            ...(edge.fromHandleId ? { fromHandleId: edge.fromHandleId } : {}),
            ...(edge.toHandleId ? { toHandleId: edge.toHandleId } : {}),
            ...(edge.relation ? { relation: edge.relation } : {}),
        });
    }
    selectedDynasties.forEach((dynasty) => {
        const dynastyId = stringValue(dynasty.id);
        for (const dynamicNode of payload.dynamicNodes) {
            const toNodeId = dynamicNodeIds.get(`${dynamicNode.ref}\0${dynastyId}`);
            if (!toNodeId) continue;
            for (const dependency of dynamicNode.dependsOn || []) {
                const fromNodeId = dynamicTemplates.has(dependency)
                    ? dynamicNodeIds.get(`${dependency}\0${dynastyId}`)
                    : nodeIds.get(dependency);
                if (!fromNodeId || fromNodeId === toNodeId) continue;
                const key = `${fromNodeId}\0${toNodeId}\0`;
                if (seenEdges.has(key)) continue;
                seenEdges.add(key);
                connections.push({
                    id: idFactory("edge", `${dependency}-${dynamicNode.ref}-${dynastyId}`),
                    fromNodeId,
                    toNodeId,
                });
            }
            for (const target of dynamicNode.connectsTo || []) {
                const targetNodeId = dynamicTemplates.has(target)
                    ? dynamicNodeIds.get(`${target}\0${dynastyId}`)
                    : nodeIds.get(target);
                if (!targetNodeId || targetNodeId === toNodeId) continue;
                const key = `${toNodeId}\0${targetNodeId}\0`;
                if (seenEdges.has(key)) continue;
                seenEdges.add(key);
                connections.push({
                    id: idFactory("edge", `${dynamicNode.ref}-${dynastyId}-${target}`),
                    fromNodeId: toNodeId,
                    toNodeId: targetNodeId,
                });
            }
        }
    });
    resizeDynamicFrames(nodes);

    return {
        nodes,
        connections,
        template: {
            id: template.id,
            itemKey: template.itemKey,
            name: template.name,
            version: String(payload.version || "1"),
            schema: payload.schema || "visideep.workflow-template/v1",
        },
    };
}

function dynastyDisplayFromForm(setupForm: WorkflowSetupForm | undefined, values: unknown[]) {
    const field = setupForm?.steps
        .flatMap((step) => step.fields)
        .find((item) => item.key === "dynasties");
    const selected = new Set(values.map((value) => String(value)));
    const result = new Map<string, Record<string, unknown>>();
    for (const option of field?.options || []) {
        const id = String(option.value);
        if (!selected.has(id)) continue;
        const name = option.label.split("·")[0]?.trim() || option.label.trim() || id;
        result.set(id, {
            id,
            name,
            label: option.label,
            description: option.description || "",
        });
    }
    return result;
}

function normalizeCanvasMetadata(nodeType: CanvasNodeType, title: string, template: PublicCreationTemplate, data: Record<string, unknown>): CanvasNodeMetadata {
    if (nodeType === CanvasNodeType.Text && (!data.content || !data.status)) {
        return {
            ...(data as CanvasNodeMetadata),
            content: stringValue(data.content) || workflowStepContent(title, data),
            status: "success",
            workflowKind: (data.workflowKind as CanvasNodeMetadata["workflowKind"]) || "free",
            workflowTitle: stringValue(data.workflowTitle) || template.name,
            workflowDescription: stringValue(data.workflowDescription) || template.description,
        };
    }
    const metadata = { ...(data as CanvasNodeMetadata) };
    metadata.workflowTitle ||= template.name;
    metadata.workflowDescription ||= template.description;
    if (nodeType === CanvasNodeType.Frame && !metadata.frame) {
        metadata.frame = { collapsed: false, expandedWidth: 1200, expandedHeight: 800 };
    }
    if (nodeType === CanvasNodeType.Script && !metadata.storyboard) {
        metadata.storyboard = {
            rows: [],
            visibleColumns: ["shotNumber", "durationSeconds", "imageGenerationPrompt", "videoMotionPrompt", "dialogue", "assets"],
            referenceNodeIds: [],
        };
    }
    return metadata;
}

function canvasNodeType(value?: string): CanvasNodeType {
    if (value && Object.values(CanvasNodeType).includes(value as CanvasNodeType)) return value as CanvasNodeType;
    return CanvasNodeType.Text;
}

export function parseCreationTemplatePayload(value: string): TemplatePayload {
    let parsed: unknown;
    try {
        parsed = JSON.parse(value);
    } catch {
        throw new Error("流程定义 JSON 格式错误");
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error("流程定义必须是 JSON 对象");
    const record = parsed as Record<string, unknown>;
    const canvas = asRecord(record.canvas);
    const rawNodes = Array.isArray(canvas?.nodes) ? canvas.nodes : record.nodes;
    const rawEdges = Array.isArray(canvas?.connections) ? canvas.connections : Array.isArray(canvas?.edges) ? canvas.edges : record.edges;
    if (!Array.isArray(rawNodes) || rawNodes.length === 0) throw new Error("流程定义至少需要一个节点");
    if (rawEdges !== undefined && !Array.isArray(rawEdges)) throw new Error("流程定义的 connections 必须是数组");
    return {
        schema: typeof record.schema === "string" ? record.schema : undefined,
        version: typeof record.version === "string" || typeof record.version === "number" ? record.version : undefined,
        stepCount: Array.isArray(record.steps) ? record.steps.length : undefined,
        setupForm: parseSetupForm(asRecord(record.forms)?.setup),
        nodes: rawNodes.map((node, index) => normalizeNode(node, index)),
        edges: Array.isArray(rawEdges) ? rawEdges.map((edge, index) => normalizeEdge(edge, index)) : [],
        dynamicNodes: Array.isArray(canvas?.dynamicNodes) ? canvas.dynamicNodes.map((node, index) => normalizeDynamicNode(node, index)) : [],
        dynastyCatalog: Array.isArray(canvas?.dynastyCatalog)
            ? canvas.dynastyCatalog as Array<Record<string, unknown>>
            : Array.isArray(record.dynastyCatalog) ? record.dynastyCatalog as Array<Record<string, unknown>> : [],
    };
}

function normalizeDynamicNode(value: unknown, index: number): TemplateDynamicNode {
    const record = asRecord(value);
    if (!record) throw new Error(`第 ${index + 1} 个动态节点格式错误`);
    const layout = asRecord(record.layout);
    const metadata = asRecord(record.metadata) || {};
    return {
        ref: requiredText(record.ref, `第 ${index + 1} 个动态节点缺少 ref`),
        type: typeof record.type === "string" ? record.type : undefined,
        titleTemplate: requiredText(record.titleTemplate, `第 ${index + 1} 个动态节点缺少标题模板`),
        parentId: typeof record.parentId === "string" ? record.parentId : undefined,
        width: optionalNumber(record.width),
        height: optionalNumber(record.height),
        layout: layout ? {
            x: finiteNumber(layout.x, 2200),
            y: finiteNumber(layout.y, 700),
            columnWidth: optionalNumber(layout.columnWidth),
        } : undefined,
        dependsOn: stringArray(record.dependsOn),
        connectsTo: stringArray(record.connectsTo),
        metadata,
    };
}

function parseSetupForm(value: unknown): WorkflowSetupForm | undefined {
    const record = asRecord(value);
    if (!record || !Array.isArray(record.steps)) return undefined;
    const steps = record.steps.flatMap((item, index): WorkflowFormStep[] => {
        const step = asRecord(item);
        if (!step || !Array.isArray(step.fields)) return [];
        const fields = step.fields.flatMap((fieldValue): WorkflowFormField[] => {
            const field = asRecord(fieldValue);
            if (!field) return [];
            const key = stringValue(field.key);
            const label = stringValue(field.label);
            const type = stringValue(field.type);
            if (!key || !label || !["segmented", "select", "cards", "tags", "text", "textarea", "checkboxGroup", "image"].includes(type)) return [];
            return [{
                key,
                type: type as WorkflowFormField["type"],
                label,
                description: stringValue(field.description) || undefined,
                placeholder: stringValue(field.placeholder) || undefined,
                required: field.required === true,
                options: parseFormOptions(field.options),
                defaultValue: field.defaultValue,
                minSelections: optionalNumber(field.minSelections),
                maxSelections: optionalNumber(field.maxSelections),
                uploadKind: field.uploadKind === "image" ? "image" : undefined,
                accept: stringValue(field.accept) || undefined,
            }];
        });
        return fields.length ? [{
            id: stringValue(step.id) || `step-${index + 1}`,
            title: stringValue(step.title) || `第 ${index + 1} 步`,
            description: stringValue(step.description) || undefined,
            fields,
        }] : [];
    });
    if (!steps.length) return undefined;
    return {
        enabled: record.enabled !== false,
        title: stringValue(record.title) || undefined,
        description: stringValue(record.description) || undefined,
        steps,
    };
}

function parseFormOptions(value: unknown): WorkflowFormOption[] | undefined {
    if (!Array.isArray(value)) return undefined;
    return value.flatMap((option): WorkflowFormOption[] => {
        if (typeof option === "string" || typeof option === "number") return [{ label: String(option), value: option }];
        const record = asRecord(option);
        if (!record) return [];
        const label = stringValue(record.label);
        const optionValue = record.value;
        if (!label || (typeof optionValue !== "string" && typeof optionValue !== "number")) return [];
        return [{ label, value: optionValue, description: stringValue(record.description) || undefined }];
    });
}

function resolveTemplateValues(value: unknown, variables: Record<string, unknown>): Record<string, unknown> {
    const resolved = resolveTemplateValue(value, variables) as Record<string, unknown>;
    return resolved && typeof resolved === "object" && !Array.isArray(resolved) ? resolved : {};
}

function resolveTemplateValue(value: unknown, variables: Record<string, unknown>): unknown {
    if (typeof value === "string") {
        return value.replace(/\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}/gu, (_, key: string) => templateVariableText(lookupTemplateValue(variables, key)));
    }
    if (Array.isArray(value)) return value.map((item) => resolveTemplateValue(item, variables));
    if (value && typeof value === "object") {
        return Object.fromEntries(Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, resolveTemplateValue(item, variables)]));
    }
    return value;
}

function resolveTemplateString(value: string, variables: Record<string, unknown>) {
    return String(resolveTemplateValue(value, variables));
}

function lookupTemplateValue(values: Record<string, unknown>, path: string) {
    let current: unknown = values;
    for (const segment of path.split(".")) {
        const record = asRecord(current);
        if (!record) return undefined;
        current = record[segment];
    }
    return current;
}

function resizeDynamicFrames(nodes: CanvasNodeData[]) {
    const frames = new Map(nodes.filter((node) => node.type === CanvasNodeType.Frame).map((node) => [node.id, node]));
    for (const node of nodes) {
        if (!node.parentId) continue;
        const frame = frames.get(node.parentId);
        if (!frame) continue;
        frame.width = Math.max(frame.width, node.position.x + node.width - frame.position.x + 120);
        frame.height = Math.max(frame.height, node.position.y + node.height - frame.position.y + 120);
        if (frame.metadata?.frame) {
            frame.metadata.frame.expandedWidth = Math.max(frame.metadata.frame.expandedWidth, frame.width);
            frame.metadata.frame.expandedHeight = Math.max(frame.metadata.frame.expandedHeight, frame.height);
        }
    }
}

function templateVariableText(value: unknown) {
    if (Array.isArray(value)) return value.map((item) => String(item).trim()).filter(Boolean).join("、") || "未填写";
    if (typeof value === "boolean") return value ? "是" : "否";
    if (typeof value === "number" && Number.isFinite(value)) return String(value);
    const text = stringValue(value);
    return text || "未填写";
}

function normalizeNode(value: unknown, index: number): TemplateNode {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`第 ${index + 1} 个流程节点格式错误`);
    const record = value as Record<string, unknown>;
    return {
        id: requiredText(record.id ?? record.ref, `第 ${index + 1} 个流程节点缺少 id`),
        type: typeof record.type === "string" ? record.type : undefined,
        title: requiredText(record.title, `第 ${index + 1} 个流程节点缺少标题`),
        parentId: typeof record.parentId === "string" ? record.parentId : undefined,
        x: optionalNumber(record.x),
        y: optionalNumber(record.y),
        w: optionalNumber(record.w ?? record.width),
        h: optionalNumber(record.h ?? record.height),
        data: record.data && typeof record.data === "object" && !Array.isArray(record.data) ? record.data as Record<string, unknown> : {},
        metadata: record.metadata && typeof record.metadata === "object" && !Array.isArray(record.metadata) ? record.metadata as Record<string, unknown> : undefined,
    };
}

function normalizeEdge(value: unknown, index: number): TemplateEdge {
    if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`第 ${index + 1} 条流程连线格式错误`);
    const record = value as Record<string, unknown>;
    return {
        id: typeof record.id === "string" ? record.id : undefined,
        from: requiredText(record.from ?? record.fromNodeId, `第 ${index + 1} 条流程连线缺少起点`),
        to: requiredText(record.to ?? record.toNodeId, `第 ${index + 1} 条流程连线缺少终点`),
        type: typeof record.type === "string" ? record.type : "next",
        label: typeof record.label === "string" ? record.label : undefined,
        fromHandleId: typeof record.fromHandleId === "string" ? record.fromHandleId : undefined,
        toHandleId: typeof record.toHandleId === "string" ? record.toHandleId : undefined,
        relation: record.relation === "storyboard-output" || record.relation === "storyboard-asset-reference" || record.relation === "batch-output" ? record.relation : undefined,
    };
}

function workflowStepContent(title: string, data: Record<string, unknown>) {
    const sections = [title];
    const description = stringValue(data.description);
    if (description) sections.push(description);
    appendList(sections, "执行方式", [stringValue(data.executionMode)]);
    appendList(sections, "执行角色", stringArray(data.executors));
    appendList(sections, "输入", stringArray(data.inputs));
    appendList(sections, "要做什么", stringArray(data.actions));
    appendList(sections, "产出", stringArray(data.outputs));
    const approval = asRecord(data.approval);
    if (approval) {
        const roles = stringArray(approval.roles).map(approvalRoleLabel);
        const evidence = stringValue(approval.evidence);
        if (roles.length || evidence) sections.push(`确认：${evidence || "等待确认"}${roles.length ? `（角色：${roles.join("、")}）` : ""}`);
    }
    const reworkTargets = stringArray(data.reworkTargets);
    if (reworkTargets.length) sections.push(`返工可回到：${reworkTargets.join("、")}`);
    return sections.filter(Boolean).join("\n\n");
}

function appendList(lines: string[], label: string, values: string[]) {
    const items = values.map((value) => value.trim()).filter(Boolean);
    if (items.length) lines.push(`${label}：${items.join("、")}`);
}

function requiredText(value: unknown, message: string) {
    const text = stringValue(value);
    if (!text) throw new Error(message);
    return text;
}

function stringValue(value: unknown) {
    return typeof value === "string" ? value.trim() : "";
}

function stringArray(value: unknown) {
    return Array.isArray(value) ? value.map(stringValue).filter(Boolean) : [];
}

function approvalRoleLabel(role: string) {
    if (role === "producer") return "制作方";
    if (role === "couple") return "新人";
    return role;
}

function asRecord(value: unknown) {
    return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function finiteNumber(value: unknown, fallback: number) {
    return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function optionalNumber(value: unknown) {
    return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function positiveNumber(value: unknown, fallback: number) {
    return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;
}

function numberValue(value: unknown) {
    return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function applyUploadedTemplateAsset(data: Record<string, unknown>, variables: Record<string, unknown>) {
    const uploadField = stringValue(data.uploadField);
    const uploaded = asRecord(uploadField ? variables[uploadField] : undefined);
    if (!uploaded) return;
    const storageKey = stringValue(uploaded.storageKey);
    const content = stringValue(uploaded.url);
    if (!storageKey && !content) return;
    data.assetId = stringValue(uploaded.assetId) || undefined;
    data.content = content;
    data.storageKey = storageKey || undefined;
    data.mimeType = stringValue(uploaded.mimeType) || undefined;
    data.bytes = numberValue(uploaded.bytes);
    data.naturalWidth = numberValue(uploaded.width);
    data.naturalHeight = numberValue(uploaded.height);
    data.status = "success";
    data.fileUpload = undefined;
}

function safeCreationTemplateVariables(value: unknown): Record<string, unknown> {
    const record = asRecord(value);
    if (!record) return {};
    return Object.fromEntries(
        Object.entries(record)
            .filter(([key]) => !/(prompt|instruction|system|secret|api.?key)/iu.test(key))
            .map(([key, item]) => [key, safeCreationTemplateVariableValue(item)]),
    );
}

function safeCreationTemplateVariableValue(value: unknown): unknown {
    if (Array.isArray(value)) return value.map(safeCreationTemplateVariableValue);
    const record = asRecord(value);
    if (!record) return value;
    return safeCreationTemplateVariables(record);
}

function slug(value: string) {
    return value.toLowerCase().replace(/[^a-z0-9\u4e00-\u9fff]+/gu, "-").replace(/^-+|-+$/gu, "").slice(0, 40) || "item";
}
