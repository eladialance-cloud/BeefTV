import { App, Button, Checkbox, Form, Input, Modal, Segmented, Select } from "antd";
import { ArrowLeft, ArrowRight, Check, ImagePlus, LoaderCircle, RefreshCw } from "lucide-react";
import { useEffect, useId, useMemo, useState, type ChangeEvent } from "react";

import {
    parseCreationTemplatePayload,
    type PublicCreationTemplate,
    type WorkflowFormField,
    type WorkflowFormStep,
} from "@/lib/canvas/creation-template-workflow";
import { readImageMeta } from "@/lib/image-utils";
import { assertUserScope, captureUserScope } from "@/lib/user-scope-guard";
import { resourceFileUrl, resourceStorageKey, uploadResourceFile } from "@/services/api/resources";
import { persistWorkspaceAssetLink } from "@/services/workspace-asset-repository";
import { useAssetStore } from "@/stores/use-asset-store";

type CanvasWorkflowSetupModalProps = {
    open: boolean;
    template: PublicCreationTemplate | null;
    submitting?: boolean;
    onCancel: () => void;
    onSubmit: (variables: Record<string, unknown>) => void;
};

export function CanvasWorkflowSetupModal({ open, template, submitting = false, onCancel, onSubmit }: CanvasWorkflowSetupModalProps) {
    const [form] = Form.useForm<Record<string, unknown>>();
    const [stepIndex, setStepIndex] = useState(0);
    const [draftValues, setDraftValues] = useState<Record<string, unknown>>({});
    const setup = useMemo(() => {
        if (!template) return undefined;
        try {
            return parseCreationTemplatePayload(template.payloadJson).setupForm;
        } catch {
            return undefined;
        }
    }, [template]);
    const steps = setup?.steps || [];
    const currentStep = steps[stepIndex];

    useEffect(() => {
        if (!open || !template) return;
        setStepIndex(0);
        const defaults = defaultFormValues(steps);
        setDraftValues(defaults);
        form.resetFields();
        form.setFieldsValue(defaults);
    }, [form, open, steps, template]);
    useEffect(() => {
        if (open) form.setFieldsValue(draftValues);
    }, [draftValues, form, open, stepIndex]);

    const submit = async () => {
        try {
            await form.validateFields();
            onSubmit({ ...draftValues, ...form.getFieldsValue(true) });
        } catch {
            // Ant Design renders field errors in place.
        }
    };
    const next = async () => {
        if (!currentStep) return;
        try {
            await form.validateFields(currentStep.fields.filter((field) => field.required).map((field) => field.key));
            setDraftValues((current) => ({ ...current, ...form.getFieldsValue(true) }));
            setStepIndex((index) => Math.min(index + 1, steps.length - 1));
        } catch {
            // Ant Design renders field errors in place.
        }
    };

    return (
        <Modal
            open={open}
            title={setup?.title || "配置工作流"}
            width={760}
            footer={null}
            maskClosable={!submitting}
            onCancel={submitting ? undefined : onCancel}
        >
            {setup?.description ? <p className="mb-5 mt-1 text-sm leading-6 text-foreground/55">{setup.description}</p> : null}
            {steps.length > 1 ? (
                <div className="mb-5 flex items-center gap-2">
                    {steps.map((step, index) => (
                        <button
                            key={step.id}
                            type="button"
                            className={`flex min-w-0 flex-1 items-center gap-2 rounded-md border px-3 py-2 text-left text-xs transition-colors ${index === stepIndex ? "border-foreground/30 bg-foreground/[0.06] text-foreground" : index < stepIndex ? "border-emerald-500/25 text-emerald-600" : "border-foreground/10 text-foreground/40"}`}
                            onClick={() => {
                                if (index <= stepIndex) {
                                    setDraftValues((current) => ({ ...current, ...form.getFieldsValue(true) }));
                                    setStepIndex(index);
                                }
                            }}
                        >
                            <span className="grid size-5 shrink-0 place-items-center rounded-full border border-current text-[10px]">{index < stepIndex ? <Check className="size-3" /> : index + 1}</span>
                            <span className="truncate">{step.title}</span>
                        </button>
                    ))}
                </div>
            ) : null}
            {currentStep ? (
                <>
                    <div className="mb-4">
                        <h3 className="text-base font-semibold">{currentStep.title}</h3>
                        {currentStep.description ? <p className="mt-1 text-xs leading-5 text-foreground/50">{currentStep.description}</p> : null}
                    </div>
                    <Form form={form} layout="vertical" requiredMark={false}>
                        <div className="grid gap-x-4 md:grid-cols-2">
                            {currentStep.fields.map((field) => <FormField key={field.key} field={field} />)}
                        </div>
                    </Form>
                </>
            ) : (
                <div className="rounded-md border border-amber-500/25 bg-amber-500/5 px-4 py-3 text-sm text-amber-700">该模板没有配置表单，可以直接创建画布。</div>
            )}
            <div className="mt-6 flex items-center justify-between gap-3">
                <Button disabled={submitting} onClick={onCancel}>取消</Button>
                <div className="flex items-center gap-2">
                    {stepIndex > 0 ? <Button icon={<ArrowLeft className="size-4" />} disabled={submitting} onClick={() => {
                        setDraftValues((current) => ({ ...current, ...form.getFieldsValue(true) }));
                        setStepIndex((index) => Math.max(0, index - 1));
                    }}>上一步</Button> : null}
                    {stepIndex < steps.length - 1 ? (
                        <Button type="primary" icon={<ArrowRight className="size-4" />} iconPosition="end" onClick={() => void next()}>下一步</Button>
                    ) : (
                        <Button type="primary" loading={submitting} onClick={() => void submit()}>创建流程画布</Button>
                    )}
                </div>
            </div>
        </Modal>
    );
}

function FormField({ field }: { field: WorkflowFormField }) {
    const rules: Array<Record<string, unknown>> = [];
    if (field.required) rules.push({ required: true, message: `请选择${field.label}` });
    if (field.type === "checkboxGroup" && (field.minSelections || field.maxSelections)) {
        rules.push({
            validator: (_: unknown, value: unknown) => {
                const count = Array.isArray(value) ? value.length : 0;
                if (field.minSelections && count < field.minSelections) {
                    return Promise.reject(new Error(`至少选择 ${field.minSelections} 项`));
                }
                if (field.maxSelections && count > field.maxSelections) {
                    return Promise.reject(new Error(`最多选择 ${field.maxSelections} 项`));
                }
                return Promise.resolve();
            },
        });
    }
    return (
        <Form.Item
            className={field.type === "textarea" || field.type === "tags" || field.type === "cards" ? "md:col-span-2" : undefined}
            name={field.key}
            label={field.label}
            rules={rules.length ? rules : undefined}
            extra={field.description}
        >
            {field.type === "segmented" ? (
                <Segmented block options={(field.options || []).map(({ label, value }) => ({ label, value }))} />
            ) : field.type === "select" ? (
                <Select options={field.options} placeholder={field.placeholder || `请选择${field.label}`} />
            ) : field.type === "cards" ? (
                <CardOptions field={field} />
            ) : field.type === "tags" ? (
                <Select mode="tags" tokenSeparators={[",", "，", "、"]} placeholder={field.placeholder || "输入后按回车添加"} options={field.options} />
            ) : field.type === "textarea" ? (
                <Input.TextArea rows={4} placeholder={field.placeholder || `请输入${field.label}`} />
            ) : field.type === "checkboxGroup" ? (
                <Checkbox.Group className="grid gap-2" options={field.options} />
            ) : field.type === "image" ? (
                <WorkflowImageField field={field} />
            ) : (
                <Input placeholder={field.placeholder || `请输入${field.label}`} />
            )}
        </Form.Item>
    );
}

type WorkflowUploadedImage = {
    assetId: string;
    resourceId: string;
    storageKey: string;
    url: string;
    mimeType: string;
    bytes?: number;
    width?: number;
    height?: number;
};

function WorkflowImageField({ field, value, onChange }: {
    field: WorkflowFormField;
    value?: unknown;
    onChange?: (value: WorkflowUploadedImage) => void;
}) {
    const { message } = App.useApp();
    const inputId = `workflow-image-${useId().replace(/:/g, "")}`;
    const [uploading, setUploading] = useState(false);
    const [uploadError, setUploadError] = useState("");
    const uploaded = uploadedImageValue(value);
    const rejectUpload = (text: string) => {
        setUploadError(text);
        message.error(text);
    };
    const chooseFile = async (file: File) => {
        if (!isSupportedWorkflowImage(file)) {
            rejectUpload(`文件未通过格式检查：${file.name || "未知文件"}，浏览器识别类型为 ${file.type || "空"}。请选择 JPG、PNG 或 WebP。`);
            return;
        }
        if (file.size > 50 * 1024 * 1024) {
            rejectUpload(`文件过大：${file.name}，当前大小 ${formatUploadBytes(file.size)}，上限 50MB。`);
            return;
        }
        setUploadError("");
        setUploading(true);
        let stage = "读取图片信息";
        try {
            const expectedScope = captureUserScope();
            const imageMeta = await readWorkflowImageMeta(file);
            stage = "上传资源";
            const resource = await uploadResourceFile(file, "image", {
                fileName: file.name,
                width: imageMeta.width,
                height: imageMeta.height,
                expectedScope,
            });
            assertUserScope(expectedScope);
            const storageKey = resourceStorageKey(resource.id);
            const url = resourceFileUrl(resource.id);
            stage = "登记素材";
            const assetId = useAssetStore.getState().addAsset({
                kind: "image",
                title: file.name.replace(/\.[^.]+$/, "") || field.label,
                coverUrl: url,
                tags: ["工作流上传"],
                category: workflowUploadCategory(field),
                status: "confirmed",
                source: "工作流上传",
                metadata: { source: "workflow-setup", uploadField: field.key },
                data: {
                    dataUrl: url,
                    storageKey,
                    width: resource.width || imageMeta.width,
                    height: resource.height || imageMeta.height,
                    bytes: resource.size || file.size,
                    mimeType: resource.mimeType || workflowImageMimeType(file),
                },
            });
            const asset = useAssetStore.getState().assets.find((item) => item.id === assetId);
            if (!asset) throw new Error("上传照片登记到素材库失败");
            stage = "保存素材";
            await persistWorkspaceAssetLink({ asset, source: "uploaded", forceRemote: true, expectedScope });
            assertUserScope(expectedScope);
            onChange?.({
                assetId,
                resourceId: resource.id,
                storageKey,
                url,
                mimeType: resource.mimeType || workflowImageMimeType(file),
                bytes: resource.size,
                width: resource.width || imageMeta.width,
                height: resource.height || imageMeta.height,
            });
        } catch (error) {
            rejectUpload(`${stage}失败：${error instanceof Error ? error.message : "未知错误"}`);
        } finally {
            setUploading(false);
        }
    };
    const selectFile = async (event: ChangeEvent<HTMLInputElement>) => {
        const file = event.currentTarget.files?.[0];
        event.currentTarget.value = "";
        if (file) await chooseFile(file);
    };
    return (
        <div>
            <input
                id={inputId}
                className="sr-only"
                type="file"
                accept={field.accept || "image/jpeg,image/png,image/webp"}
                disabled={uploading}
                onChange={(event) => void selectFile(event)}
            />
            <label
                htmlFor={inputId}
                className={`grid min-h-28 w-full place-items-center overflow-hidden rounded-md border border-dashed px-4 py-3 text-center transition-colors ${uploaded ? "border-emerald-500/35 bg-emerald-500/5" : "border-foreground/20 bg-foreground/[0.025] hover:border-foreground/35 hover:bg-foreground/[0.045]"}`}
                aria-disabled={uploading}
            >
                {uploading ? (
                    <span className="flex items-center gap-2 text-sm text-foreground/60"><LoaderCircle className="size-4 animate-spin" />照片上传中</span>
                ) : uploaded ? (
                    <span className="flex w-full items-center gap-3 text-left">
                        <img src={uploaded.url} alt={field.label} className="size-20 shrink-0 rounded-md object-cover" />
                        <span className="min-w-0 flex-1">
                            <strong className="block truncate text-sm">{field.label}已上传</strong>
                            <small className="mt-1 flex items-center gap-1 text-xs text-foreground/45"><RefreshCw className="size-3" />点击更换照片</small>
                        </span>
                    </span>
                ) : (
                    <span>
                        <ImagePlus className="mx-auto size-6 text-foreground/45" />
                        <strong className="mt-2 block text-sm">{field.label}</strong>
                        <small className="mt-1 block text-xs text-foreground/45">点击选择清晰正脸照片</small>
                    </span>
                )}
            </label>
            {uploadError ? <p role="alert" className="mt-2 text-xs leading-5 text-red-500">{uploadError}</p> : null}
        </div>
    );
}

function uploadedImageValue(value: unknown): WorkflowUploadedImage | null {
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const record = value as Record<string, unknown>;
    const url = typeof record.url === "string" ? record.url : "";
    const storageKey = typeof record.storageKey === "string" ? record.storageKey : "";
    if (!url || !storageKey) return null;
    return {
        assetId: typeof record.assetId === "string" ? record.assetId : "",
        resourceId: typeof record.resourceId === "string" ? record.resourceId : "",
        storageKey,
        url,
        mimeType: typeof record.mimeType === "string" ? record.mimeType : "",
        bytes: typeof record.bytes === "number" ? record.bytes : undefined,
        width: typeof record.width === "number" ? record.width : undefined,
        height: typeof record.height === "number" ? record.height : undefined,
    };
}

function workflowUploadCategory(field: WorkflowFormField) {
    return /photo|portrait|character|person/i.test(field.key) ? "character" as const : "material" as const;
}

function isSupportedWorkflowImage(file: File) {
    return file.type.startsWith("image/") || /\.(?:jpe?g|png|webp)$/i.test(file.name);
}

function workflowImageMimeType(file: File) {
    if (file.type.startsWith("image/")) return file.type;
    if (/\.png$/i.test(file.name)) return "image/png";
    if (/\.webp$/i.test(file.name)) return "image/webp";
    return "image/jpeg";
}

async function readWorkflowImageMeta(file: File) {
    const objectUrl = URL.createObjectURL(file);
    try {
        return await readImageMeta(objectUrl);
    } finally {
        URL.revokeObjectURL(objectUrl);
    }
}

function formatUploadBytes(bytes: number) {
    if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
    return `${Math.max(1, Math.round(bytes / 1024))}KB`;
}

function CardOptions({ field }: { field: WorkflowFormField }) {
    return (
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {(field.options || []).map((option) => (
                <Form.Item key={String(option.value)} noStyle shouldUpdate>
                    {({ getFieldValue, setFieldValue }) => {
                        const active = getFieldValue(field.key) === option.value;
                        return (
                            <button
                                type="button"
                                className={`min-h-16 rounded-md border px-3 py-2.5 text-left transition-colors ${active ? "border-foreground/35 bg-foreground/[0.07]" : "border-foreground/10 hover:border-foreground/25 hover:bg-foreground/[0.03]"}`}
                                onClick={() => setFieldValue(field.key, option.value)}
                            >
                                <span className="block text-sm font-medium">{option.label}</span>
                                {option.description ? <span className="mt-1 block text-xs leading-5 text-foreground/50">{option.description}</span> : null}
                            </button>
                        );
                    }}
                </Form.Item>
            ))}
        </div>
    );
}

function defaultFormValues(steps: WorkflowFormStep[]) {
    return steps.reduce<Record<string, unknown>>((values, step) => {
        step.fields.forEach((field) => {
            if (field.defaultValue !== undefined) values[field.key] = field.defaultValue;
            else if (field.type === "tags" || field.type === "checkboxGroup") values[field.key] = [];
        });
        return values;
    }, {});
}
