import { Check, Image, ImagePlus, LoaderCircle, RefreshCw, Save } from "lucide-react";
import { useEffect, useId, useState, type ChangeEvent } from "react";

import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";
import type { CanvasNodeData } from "@/types/canvas";

type SourcePatch = Partial<WeddingProductionState["source"]>;

export function SetupStage({
    production,
    saving,
    feedback,
    sourcePhotoNodes = [],
    uploadingPhoto,
    onSave,
    onApprove,
    onUploadPhoto,
}: {
    production: WeddingProductionState;
    saving: boolean;
    feedback: string;
    sourcePhotoNodes?: CanvasNodeData[];
    uploadingPhoto?: "groom" | "bride" | "";
    onSave: (patch: SourcePatch) => Promise<void>;
    onApprove: (patch: SourcePatch) => Promise<void>;
    onUploadPhoto?: (kind: "groom" | "bride", file: File) => Promise<void>;
}) {
    const [draft, setDraft] = useState(production.source);
    useEffect(() => setDraft(production.source), [production.source]);

    const blockers = weddingStageBlockers({
        ...production,
        source: draft,
    }, "setup");
    const groomNode = sourcePhotoNodes.find((node) => uploadField(node) === "groomPhoto");
    const brideNode = sourcePhotoNodes.find((node) => uploadField(node) === "bridePhoto");
    const photosReady = Boolean(groomNode?.metadata?.content && brideNode?.metadata?.content);
    const dirty = JSON.stringify(draft) !== JSON.stringify(production.source);
    const update = <K extends keyof SourcePatch>(key: K, value: SourcePatch[K]) => {
        setDraft((current) => ({ ...current, [key]: value }));
    };

    return (
        <section className="wedding-setup-stage">
            <div className="wedding-stage-panel">
                <header>
                    <div>
                        <small>01 · SOURCE</small>
                        <strong>人物与故事</strong>
                    </div>
                    <span>{photosReady ? "照片已上传" : "缺少照片"}</span>
                </header>
                <div className="wedding-setup-photos">
                    <PhotoUploadCard
                        label="新郎照片"
                        node={groomNode}
                        uploading={uploadingPhoto === "groom"}
                        onUpload={(file) => onUploadPhoto?.("groom", file)}
                    />
                    <PhotoUploadCard
                        label="新娘照片"
                        node={brideNode}
                        uploading={uploadingPhoto === "bride"}
                        onUpload={(file) => onUploadPhoto?.("bride", file)}
                    />
                </div>
                <div className="wedding-setup-form">
                    <label>
                        <span>新郎称呼</span>
                        <input value={draft.maleName} onChange={(event) => update("maleName", event.currentTarget.value)} />
                    </label>
                    <label>
                        <span>新娘称呼</span>
                        <input value={draft.femaleName} onChange={(event) => update("femaleName", event.currentTarget.value)} />
                    </label>
                    <label className="is-wide">
                        <span>爱情故事</span>
                        <textarea rows={6} value={draft.story} onChange={(event) => update("story", event.currentTarget.value)} />
                    </label>
                    <label className="wedding-setup-consent is-wide">
                        <input type="checkbox" checked={draft.consentConfirmed} onChange={(event) => update("consentConfirmed", event.currentTarget.checked)} />
                        <span>照片使用授权已确认，仅用于当前订单</span>
                    </label>
                </div>
            </div>

            <div className="wedding-stage-panel">
                <header>
                    <div>
                        <small>01 · OUTPUT</small>
                        <strong>输出规格</strong>
                    </div>
                    <span>影响全部朝代资产</span>
                </header>
                <div className="wedding-setup-choice">
                    <span>画幅</span>
                    <div>
                        {["16:9", "9:16"].map((value) => (
                            <button
                                key={value}
                                type="button"
                                className={draft.ratio === value ? "is-selected" : ""}
                                onClick={() => update("ratio", value)}
                            >
                                {value}
                            </button>
                        ))}
                    </div>
                </div>
                <div className="wedding-setup-choice">
                    <span>分辨率</span>
                    <div>
                        {["480p", "720p", "1080p"].map((value) => (
                            <button
                                key={value}
                                type="button"
                                className={draft.resolution === value ? "is-selected" : ""}
                                onClick={() => update("resolution", value)}
                            >
                                {value}
                            </button>
                        ))}
                    </div>
                </div>
                <div className="wedding-setup-choice">
                    <span>每朝代时长</span>
                    <div>
                        {[4, 5, 6, 8, 10, 12, 15].map((value) => (
                            <button
                                key={value}
                                type="button"
                                className={(draft.durationSeconds || 8) === value ? "is-selected" : ""}
                                onClick={() => update("durationSeconds", value)}
                            >
                                {value} 秒
                            </button>
                        ))}
                    </div>
                </div>
                {blockers.length ? (
                    <div className="wedding-stage-blockers">
                        {blockers.map((blocker) => <span key={blocker}>{blocker}</span>)}
                    </div>
                ) : (
                    <div className="wedding-stage-ready"><Check /> 当前项目设定可以确认</div>
                )}
            </div>

            <footer className="wedding-stage-actions">
                <span>{feedback}</span>
                <div>
                    <button type="button" disabled={saving || !dirty} onClick={() => void onSave(draft)}>
                        <Save />
                        保存设定
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || blockers.length > 0}
                        onClick={() => void onApprove(draft)}
                    >
                        <Check />
                        确认项目设定
                    </button>
                </div>
            </footer>
        </section>
    );
}

function uploadField(node: CanvasNodeData) {
    const metadata = node.metadata && typeof node.metadata === "object"
        ? node.metadata as Record<string, unknown>
        : undefined;
    return typeof metadata?.uploadField === "string" ? metadata.uploadField : "";
}

function PhotoUploadCard({
    label,
    node,
    uploading,
    onUpload,
}: {
    label: string;
    node?: CanvasNodeData;
    uploading: boolean;
    onUpload: (file: File) => Promise<void> | undefined;
}) {
    const inputId = `wedding-photo-${useId().replace(/:/g, "")}`;
    const url = node?.metadata?.content || "";
    const selectFile = async (event: ChangeEvent<HTMLInputElement>) => {
        const file = event.currentTarget.files?.[0];
        event.currentTarget.value = "";
        if (file) await onUpload(file);
    };
    return (
        <>
            <input
                id={inputId}
                className="wedding-setup-photo-input"
                type="file"
                accept="image/jpeg,image/png,image/webp"
                disabled={uploading}
                aria-label={`上传${label}`}
                onChange={(event) => void selectFile(event)}
            />
            <label htmlFor={inputId} className={url ? "is-ready" : ""}>
            {url ? <img src={url} alt={label} /> : <Image />}
            <span>
                <strong>{label}</strong>
                <small>{uploading ? "正在上传…" : url ? "点击更换照片" : `点击上传${label}`}</small>
            </span>
            {uploading ? <LoaderCircle className="wedding-spinner" /> : url ? <RefreshCw /> : <ImagePlus />}
            </label>
        </>
    );
}
