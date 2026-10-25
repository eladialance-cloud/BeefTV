import {
    AlertTriangle,
    Check,
    Film,
    ImageOff,
    LoaderCircle,
    RefreshCw,
    ScanSearch,
    Sparkles,
} from "lucide-react";
import { useState, type ReactNode } from "react";

import { weddingStageBlockers } from "@/lib/wedding/production-state";
import { weddingVideoJobs } from "@/lib/wedding/video-runtime";
import type {
    WeddingDynastyState,
    WeddingProductionState,
    WeddingVideoQc,
} from "@/lib/wedding/types";
import type { GenerationTask } from "@/services/api/task-center";
import type { CanvasNodeData } from "@/types/canvas";

export function VideoStage({
    production,
    nodes,
    tasks,
    selectedDynastyId,
    saving,
    feedback,
    promptInspector,
    onSelectDynasty,
    onGenerateAll,
    onGenerateDynasty,
    onRunQc,
    onSelectVersion,
    onConfirm,
    onOverrideQc,
    onApprove,
}: {
    production: WeddingProductionState;
    nodes: CanvasNodeData[];
    tasks: Record<string, GenerationTask>;
    selectedDynastyId: string;
    saving: boolean;
    feedback: string;
    promptInspector: ReactNode;
    onSelectDynasty: (dynastyId: string) => void;
    onGenerateAll: () => Promise<void>;
    onGenerateDynasty: (dynastyId: string) => Promise<void>;
    onRunQc: (dynastyId: string) => Promise<void>;
    onSelectVersion: (dynastyId: string, assetId: string) => Promise<void>;
    onConfirm: (dynastyId: string, assetId: string) => Promise<void>;
    onOverrideQc: (dynastyId: string, reason: string) => Promise<void>;
    onApprove: () => Promise<void>;
}) {
    const jobs = weddingVideoJobs(production);
    const blockers = weddingStageBlockers(production, "videos");
    const activeTask = jobs.some((job) => isTaskRunning(tasks[job.nodeId]));
    const dynasty = production.dynasties.find((item) => item.id === selectedDynastyId) || production.dynasties[0];
    const nodeByID = new Map(nodes.map((node) => [node.id, node]));
    const [overrideReasons, setOverrideReasons] = useState<Record<string, string>>({});

    return (
        <section className="wedding-video-stage">
            <div className="wedding-video-overview">
                <div>
                    <small>05 · VIDEO DIRECTION</small>
                    <strong>逐朝代视频导演与审核</strong>
                    <span>提示词覆盖、视频生成、自动 QC 和版本选择共用同一条任务链。</span>
                </div>
                <div>
                    <span>{production.dynasties.filter((item) => videoReady(item)).length} / {production.dynasties.length}</span>
                    <small>视频已审核</small>
                </div>
            </div>

            <div className="wedding-video-tabs" role="tablist" aria-label="朝代视频">
                {production.dynasties.map((item) => (
                    <button
                        key={item.id}
                        type="button"
                        role="tab"
                        aria-selected={item.id === dynasty?.id}
                        className={item.id === dynasty?.id ? "is-active" : ""}
                        onClick={() => onSelectDynasty(item.id)}
                    >
                        <span>{String(item.order + 1).padStart(2, "0")}</span>
                        <strong>{item.name}</strong>
                        <small>{videoReady(item) ? "已审核" : statusLabel(item.statuses.video)}</small>
                    </button>
                ))}
            </div>

            {dynasty ? (
                <VideoDynastyWorkspace
                    dynasty={dynasty}
                    node={nodeByID.get(dynasty.videoNodeId)}
                    task={tasks[dynasty.videoNodeId]}
                    saving={saving}
                    promptInspector={promptInspector}
                    overrideReason={overrideReasons[dynasty.id] || ""}
                    onOverrideReasonChange={(reason) => setOverrideReasons((current) => ({
                        ...current,
                        [dynasty.id]: reason,
                    }))}
                    onGenerate={() => onGenerateDynasty(dynasty.id)}
                    onRunQc={() => onRunQc(dynasty.id)}
                    onSelectVersion={(assetId) => onSelectVersion(dynasty.id, assetId)}
                    onConfirm={(assetId) => onConfirm(dynasty.id, assetId)}
                    onOverrideQc={(reason) => onOverrideQc(dynasty.id, reason)}
                />
            ) : (
                <div className="wedding-video-empty">当前项目没有可制作朝代。</div>
            )}

            <footer className="wedding-stage-actions wedding-video-actions">
                <div>
                    <span>{feedback}</span>
                    {blockers.length ? (
                        <small>{blockers.slice(0, 2).join("；")}{blockers.length > 2 ? `；另 ${blockers.length - 2} 项` : ""}</small>
                    ) : (
                        <small>全部朝代视频与 QC 已确认，可以进入包装制作。</small>
                    )}
                </div>
                <div>
                    <button type="button" disabled={saving || activeTask} onClick={() => void onGenerateAll()}>
                        {activeTask ? <LoaderCircle className="wedding-spinner" /> : <Sparkles />}
                        生成缺失视频
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || activeTask || blockers.length > 0}
                        onClick={() => void onApprove()}
                    >
                        <Check />
                        确认视频阶段
                    </button>
                </div>
            </footer>
        </section>
    );
}

function VideoDynastyWorkspace({
    dynasty,
    node,
    task,
    saving,
    promptInspector,
    overrideReason,
    onOverrideReasonChange,
    onGenerate,
    onRunQc,
    onSelectVersion,
    onConfirm,
    onOverrideQc,
}: {
    dynasty: WeddingDynastyState;
    node?: CanvasNodeData;
    task?: GenerationTask;
    saving: boolean;
    promptInspector: ReactNode;
    overrideReason: string;
    onOverrideReasonChange: (reason: string) => void;
    onGenerate: () => Promise<void>;
    onRunQc: () => Promise<void>;
    onSelectVersion: (assetId: string) => Promise<void>;
    onConfirm: (assetId: string) => Promise<void>;
    onOverrideQc: (reason: string) => Promise<void>;
}) {
    const running = isTaskRunning(task) || node?.metadata?.status === "loading";
    const progress = task?.progress ?? node?.metadata?.taskProgress;
    const selectedVersion = dynasty.videoVersions?.find((version) => version.assetId === dynasty.videoSelection);
    const previewUrl = selectedVersion?.previewUrl || node?.metadata?.content || "";
    const posterUrl = selectedVersion?.posterUrl || node?.metadata?.videoPreview?.content || "";
    const qc = dynasty.qc.video;
    const qcReady = qc?.verdict === "pass" || Boolean(dynasty.qc.overrides.video?.reason.trim());
    const reference = (gender: "male" | "female") => dynasty.portraitVersionHistory?.[gender]
        ?.find((version) => version.assetId === dynasty.portraitSelection?.[gender]);
    const scene = dynasty.sceneVersions?.find((version) => version.assetId === dynasty.sceneSelection);

    return (
        <article className="wedding-video-workspace">
            <header>
                <div>
                    <span>{String(dynasty.order + 1).padStart(2, "0")}</span>
                    <div>
                        <strong>{dynasty.name} · 视频导演</strong>
                        <small>{videoReady(dynasty) ? "视频与 QC 已确认" : "等待生成、QC 或确认"}</small>
                    </div>
                </div>
                <span className={videoReady(dynasty) ? "is-approved" : ""}>
                    {videoReady(dynasty) ? <Check /> : null}
                    {videoReady(dynasty) ? "已审核" : statusLabel(dynasty.statuses.video)}
                </span>
            </header>

            <div className="wedding-video-body">
                <section className="wedding-video-preview-panel">
                    <div className="wedding-video-references" aria-label={`${dynasty.name}视频参考素材`}>
                        <ReferenceThumb label="新郎" url={reference("male")?.previewUrl} />
                        <ReferenceThumb label="新娘" url={reference("female")?.previewUrl} />
                        <ReferenceThumb label="场景" url={scene?.previewUrl} />
                    </div>
                    <div className="wedding-video-player">
                        {previewUrl ? (
                            <video controls preload="metadata" poster={posterUrl || undefined} src={previewUrl} />
                        ) : (
                            <div>
                                <Film />
                                <strong>尚无朝代视频</strong>
                                <span>确认提示词后，使用男女肖像和场景首帧生成视频</span>
                            </div>
                        )}
                        {running ? (
                            <span className="wedding-video-progress">
                                {task?.stage || "正在生成视频"}
                                {typeof progress === "number" ? ` · ${Math.round(progress)}%` : ""}
                            </span>
                        ) : null}
                    </div>
                    <div className="wedding-video-versions">
                        <span>历史版本</span>
                        <div>
                            {(dynasty.videoVersions || []).map((version, index) => (
                                <button
                                    key={`${version.assetId}-${index}`}
                                    type="button"
                                    className={version.assetId === dynasty.videoSelection ? "is-selected" : ""}
                                    disabled={saving || running}
                                    aria-label={`选择${dynasty.name}视频版本 ${index + 1}`}
                                    onClick={() => void onSelectVersion(version.assetId)}
                                >
                                    {version.posterUrl ? <img src={version.posterUrl} alt="" /> : <Film />}
                                    <span>V{index + 1}</span>
                                    {version.durationMs ? <small>{formatDuration(version.durationMs)}</small> : null}
                                </button>
                            ))}
                            {!dynasty.videoVersions?.length ? <p>生成后会在这里保留版本。</p> : null}
                        </div>
                    </div>
                    <div className="wedding-video-settings">
                        <span>{node?.metadata?.seconds || task?.inputJson ? "8 秒" : "按节点设置"}</span>
                        <span>{node?.metadata?.size || "16:9"}</span>
                        <span>{node?.metadata?.vquality || "720p"}</span>
                    </div>
                    <footer>
                        <button type="button" disabled={saving || running || !node} onClick={() => void onGenerate()}>
                            <RefreshCw />
                            {previewUrl ? "重新生成视频" : "生成朝代视频"}
                        </button>
                        <span>{qcReady ? "QC 条件已满足" : "先完成 QC 或记录覆盖"}</span>
                        <button
                            type="button"
                            className="is-primary"
                            disabled={saving || running || !dynasty.videoSelection || !qcReady}
                            onClick={() => void onConfirm(dynasty.videoSelection || "")}
                        >
                            <Check />
                            确认此视频
                        </button>
                    </footer>
                </section>

                <div className="wedding-video-inspector-column">
                    {promptInspector}
                    <section className="wedding-video-qc-panel">
                        <header>
                            <div>
                                <ScanSearch />
                                <span>
                                    <strong>自动 QC</strong>
                                    <small>{qc ? `检查于 ${formatTime(qc.checkedAt)}` : "尚未执行视频检查"}</small>
                                </span>
                            </div>
                            <button type="button" disabled={saving || running || !previewUrl} onClick={() => void onRunQc()}>
                                <ScanSearch />
                                {qc && qc.verdict !== "pending" ? "重新 QC" : "运行 QC"}
                            </button>
                        </header>
                        {qc && qc.verdict !== "pending" ? <VideoQcResult qc={qc} /> : (
                            <div className="wedding-video-qc-empty">
                                <AlertTriangle />
                                <span>{qc?.reason || "没有 QC 记录。请运行自动检查，或填写覆盖原因。"}</span>
                            </div>
                        )}
                        {dynasty.qc.overrides.video ? (
                            <div className="wedding-video-override-record">
                                <strong>已人工覆盖</strong>
                                <span>{dynasty.qc.overrides.video.reason}</span>
                            </div>
                        ) : null}
                        <label className="wedding-video-override">
                            <span>覆盖原因</span>
                            <textarea
                                rows={3}
                                value={overrideReason}
                                placeholder="仅在确认瑕疵不影响交付时填写"
                                aria-label={`${dynasty.name}视频QC覆盖原因`}
                                onChange={(event) => onOverrideReasonChange(event.currentTarget.value)}
                            />
                        </label>
                        <button
                            type="button"
                            className="wedding-video-override-button"
                            disabled={saving || running || !overrideReason.trim()}
                            onClick={() => void onOverrideQc(overrideReason)}
                        >
                            <AlertTriangle />
                            记录人工覆盖
                        </button>
                    </section>
                </div>
            </div>
        </article>
    );
}

function ReferenceThumb({ label, url }: { label: string; url?: string }) {
    return (
        <div>
            {url ? <img src={url} alt={`${label}参考`} /> : <ImageOff />}
            <span>{label}</span>
        </div>
    );
}

function VideoQcResult({ qc }: { qc: WeddingVideoQc }) {
    const checks = [
        ["脸部稳定", qc.faceStable, false],
        ["服饰连续", qc.outfitStable, false],
        ["动作可读", qc.actionReadable, false],
        ["无额外人物", qc.extraPerson, true],
        ["无文字水印", qc.watermarkOrText, true],
    ] as const;
    return (
        <div className={`wedding-video-qc-result is-${qc.verdict}`}>
            <strong>{qc.verdict === "pass" ? "QC 通过" : qc.verdict === "error" ? "QC 执行错误" : "QC 未通过"}</strong>
            <span>{qc.reason}</span>
            <div>
                {checks.map(([label, value, inverted]) => {
                    const passed = inverted ? !value : value;
                    return (
                        <span key={label} className={passed ? "is-pass" : "is-fail"}>
                            {passed ? <Check /> : <AlertTriangle />}
                            {label}
                        </span>
                    );
                })}
            </div>
        </div>
    );
}

function isTaskRunning(task?: GenerationTask) {
    return task?.status === "queued" || task?.status === "running";
}

function videoReady(dynasty: WeddingDynastyState) {
    const qcReady = dynasty.qc.video?.verdict === "pass" || Boolean(dynasty.qc.overrides.video?.reason.trim());
    return Boolean(dynasty.videoSelection) && qcReady;
}

function statusLabel(status: WeddingDynastyState["statuses"]["video"]) {
    const labels: Record<WeddingDynastyState["statuses"]["video"], string> = {
        not_started: "未开始",
        ready: "待生成",
        generating: "生成中",
        needs_review: "待审核",
        approved: "已审核",
        failed: "生成失败",
        stale: "需更新",
    };
    return labels[status];
}

function formatDuration(durationMs: number) {
    return `${Math.max(1, Math.round(durationMs / 1000))}s`;
}

function formatTime(value: string) {
    try {
        return new Date(value).toLocaleString("zh-CN", { hour12: false });
    } catch {
        return value;
    }
}
