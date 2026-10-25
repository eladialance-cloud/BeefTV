import { Check, Image, Save } from "lucide-react";
import { useEffect, useState } from "react";

import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";

type SourcePatch = Partial<WeddingProductionState["source"]>;

export function SetupStage({
    production,
    saving,
    feedback,
    onSave,
    onApprove,
}: {
    production: WeddingProductionState;
    saving: boolean;
    feedback: string;
    onSave: (patch: SourcePatch) => Promise<void>;
    onApprove: (patch: SourcePatch) => Promise<void>;
}) {
    const [draft, setDraft] = useState(production.source);
    useEffect(() => setDraft(production.source), [production.source]);

    const blockers = weddingStageBlockers(production, "setup");
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
                    <span>{production.source.groomPhotoNodeId && production.source.bridePhotoNodeId ? "照片已登记" : "缺少照片"}</span>
                </header>
                <div className="wedding-setup-photos">
                    <PhotoSummary label="新郎照片" nodeId={production.source.groomPhotoNodeId} />
                    <PhotoSummary label="新娘照片" nodeId={production.source.bridePhotoNodeId} />
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

function PhotoSummary({ label, nodeId }: { label: string; nodeId: string }) {
    return (
        <div className={nodeId ? "is-ready" : ""}>
            <Image />
            <span>
                <strong>{label}</strong>
                <small>{nodeId ? `已登记 · ${nodeId}` : "尚未上传"}</small>
            </span>
        </div>
    );
}
