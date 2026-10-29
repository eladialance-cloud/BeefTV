import { Button, Modal, Spin } from "antd";
import { FolderPlus, LayoutTemplate, Plus } from "lucide-react";

import { parseCreationTemplatePayload, type PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";

type CanvasTemplatePickerProps = {
    open: boolean;
    templates: PublicCreationTemplate[];
    loading?: boolean;
    error?: string;
    creatingKey?: string;
    onCancel: () => void;
    onBlank: () => void;
    onSelect: (template: PublicCreationTemplate) => void;
    onRetry?: () => void;
};

export function CanvasTemplatePicker({ open, templates, loading = false, error = "", creatingKey = "", onCancel, onBlank, onSelect, onRetry }: CanvasTemplatePickerProps) {
    return (
        <Modal open={open} title="选择画布模板" footer={null} width={760} onCancel={creatingKey ? undefined : onCancel} maskClosable={!creatingKey}>
            <p className="mb-4 mt-1 text-sm text-foreground/55">模板来自管理后台。选择后会按流程定义自动创建节点和主流程连线。</p>
            {loading ? (
                <div className="grid min-h-48 place-items-center"><div className="flex items-center gap-3 text-sm text-foreground/55"><Spin size="small" /><span>正在读取模板</span></div></div>
            ) : error ? (
                <div className="rounded-lg border border-red-500/20 bg-red-500/5 px-4 py-5 text-sm">
                    <div className="font-medium text-red-500">模板读取失败</div>
                    <div className="mt-1 text-foreground/60">{error}</div>
                    {onRetry ? <Button className="mt-3" onClick={onRetry}>重新读取</Button> : null}
                </div>
            ) : (
                <div className="grid gap-3 md:grid-cols-2">
                    <button type="button" className="flex min-h-32 items-center gap-4 rounded-lg border border-dashed border-foreground/20 p-5 text-left transition-colors hover:border-foreground/40 hover:bg-foreground/[0.03]" disabled={Boolean(creatingKey)} onClick={onBlank}>
                        <span className="grid size-11 shrink-0 place-items-center rounded-lg bg-foreground/[0.06]"><Plus className="size-5" /></span>
                        <span className="min-w-0">
                            <strong className="block text-sm">空白画布</strong>
                            <span className="mt-1 block text-xs leading-5 text-foreground/50">不套用流程，从零开始添加节点。</span>
                        </span>
                    </button>
                    {templates.map((template) => {
                        const stepCount = templateStepCount(template);
                        const busy = creatingKey === template.itemKey;
                        return (
                            <button key={template.id} type="button" className="flex min-h-32 items-start gap-4 rounded-lg border border-foreground/10 p-5 text-left transition-colors hover:border-foreground/30 hover:bg-foreground/[0.03] disabled:cursor-wait disabled:opacity-60" disabled={Boolean(creatingKey)} onClick={() => onSelect(template)}>
                                <span className="grid size-11 shrink-0 place-items-center rounded-lg bg-foreground/[0.06]">{busy ? <Spin size="small" /> : template.canvasType === "batch" ? <FolderPlus className="size-5" /> : <LayoutTemplate className="size-5" />}</span>
                                <span className="min-w-0 flex-1">
                                    <span className="flex items-center justify-between gap-3">
                                        <strong className="truncate text-sm">{template.name}</strong>
                                        <span className="shrink-0 text-[11px] text-foreground/40">{stepCount ? `${stepCount} 步` : "流程模板"}</span>
                                    </span>
                                    <span className="mt-1 line-clamp-2 block text-xs leading-5 text-foreground/50">{template.description || "按后台配置自动创建流程节点。"}</span>
                                    <span className="mt-3 block truncate font-mono text-[10px] text-foreground/30">{template.itemKey}</span>
                                </span>
                            </button>
                        );
                    })}
                </div>
            )}
        </Modal>
    );
}

function templateStepCount(template: PublicCreationTemplate) {
    try {
        const payload = parseCreationTemplatePayload(template.payloadJson);
        return payload.stepCount || payload.nodes.length;
    } catch {
        return 0;
    }
}
