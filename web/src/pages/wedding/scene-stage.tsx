import {
    AlertTriangle,
    Check,
    ImageOff,
    LoaderCircle,
    RefreshCw,
    ScanSearch,
    Sparkles,
    UserRound,
} from "lucide-react";
import { useState } from "react";

import { weddingStageBlockers } from "@/lib/wedding/production-state";
import { weddingSceneJobs } from "@/lib/wedding/scene-runtime";
import type {
    WeddingDynastyState,
    WeddingProductionState,
    WeddingSceneQc,
} from "@/lib/wedding/types";
import type { GenerationTask } from "@/services/api/task-center";
import type { CanvasNodeData } from "@/types/canvas";

export function SceneStage({
    production,
    nodes,
    tasks,
    saving,
    feedback,
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
    saving: boolean;
    feedback: string;
    onGenerateAll: () => Promise<void>;
    onGenerateDynasty: (dynastyId: string) => Promise<void>;
    onRunQc: (dynastyId: string) => Promise<void>;
    onSelectVersion: (dynastyId: string, assetId: string) => Promise<void>;
    onConfirm: (dynastyId: string, assetId: string) => Promise<void>;
    onOverrideQc: (dynastyId: string, reason: string) => Promise<void>;
    onApprove: () => Promise<void>;
}) {
    const jobs = weddingSceneJobs(production);
    const blockers = weddingStageBlockers(production, "scenes");
    const nodeByID = new Map(nodes.map((node) => [node.id, node]));
    const activeTask = jobs.some((job) => isTaskRunning(tasks[job.nodeId]));
    const [overrideReasons, setOverrideReasons] = useState<Record<string, string>>({});

    return (
        <section className="wedding-scene-stage">
            <div className="wedding-scene-overview">
                <div>
                    <small>04 · SCENE FRAMES</small>
                    <strong>男女双人场景与 QC</strong>
                    <span>场景必须引用已确认的男女定妆照，QC 失败时只能通过明确人工覆盖放行。</span>
                </div>
                <div>
                    <span>{production.dynasties.filter((dynasty) => sceneReady(dynasty)).length} / {production.dynasties.length}</span>
                    <small>场景已审核</small>
                </div>
            </div>

            <div className="wedding-scene-dynasties">
                {production.dynasties.map((dynasty) => (
                    <SceneDynastyBoard
                        key={dynasty.id}
                        dynasty={dynasty}
                        node={nodeByID.get(dynasty.sceneNodeId)}
                        task={tasks[dynasty.sceneNodeId]}
                        saving={saving}
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
                ))}
            </div>

            <footer className="wedding-stage-actions wedding-scene-actions">
                <div>
                    <span>{feedback}</span>
                    {blockers.length ? (
                        <small>{blockers.slice(0, 2).join("；")}{blockers.length > 2 ? `；另 ${blockers.length - 2} 项` : ""}</small>
                    ) : (
                        <small>全部朝代场景首帧与 QC 已确认，可以推进视频导演。</small>
                    )}
                </div>
                <div>
                    <button type="button" disabled={saving || activeTask} onClick={() => void onGenerateAll()}>
                        {activeTask ? <LoaderCircle className="wedding-spinner" /> : <Sparkles />}
                        生成缺失场景
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || activeTask || blockers.length > 0}
                        onClick={() => void onApprove()}
                    >
                        <Check />
                        确认场景阶段
                    </button>
                </div>
            </footer>
        </section>
    );
}

function SceneDynastyBoard({
    dynasty,
    node,
    task,
    saving,
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
    const selectedVersion = dynasty.sceneVersions?.find((version) => version.assetId === dynasty.sceneSelection);
    const previewUrl = selectedVersion?.previewUrl || node?.metadata?.content || "";
    const qc = dynasty.qc.scene;
    const qcReady = qc?.verdict === "pass" || Boolean(dynasty.qc.overrides.scene?.reason.trim());
    const reference = (gender: "male" | "female") => dynasty.portraitVersionHistory?.[gender]
        ?.find((version) => version.assetId === dynasty.portraitSelection?.[gender]);

    return (
        <article className="wedding-scene-dynasty">
            <header>
                <div>
                    <span>{String(dynasty.order + 1).padStart(2, "0")}</span>
                    <div>
                        <strong>{dynasty.name}</strong>
                        <small>{sceneReady(dynasty) ? "场景与 QC 已确认" : "等待场景审核"}</small>
                    </div>
                </div>
                <span className={sceneReady(dynasty) ? "is-approved" : ""}>
                    {sceneReady(dynasty) ? <Check /> : null}
                    {sceneReady(dynasty) ? "已审核" : statusLabel(dynasty.statuses.scene)}
                </span>
            </header>

            <div className="wedding-scene-body">
                <section className="wedding-scene-preview">
                    <div className="wedding-scene-references" aria-label={`${dynasty.name}定妆照参考`}>
                        <ReferenceThumb label="新郎参考" version={reference("male")} />
                        <ReferenceThumb label="新娘参考" version={reference("female")} />
                    </div>
                    <div className={`wedding-scene-frame${previewUrl ? " has-image" : ""}`}>
                        {previewUrl ? (
                            <img src={previewUrl} alt={`${dynasty.name}双人婚礼场景`} />
                        ) : (
                            <div>
                                <ImageOff />
                                <strong>尚无场景首帧</strong>
                                <span>使用已确认的男女定妆照生成双人婚礼场景</span>
                            </div>
                        )}
                        {running ? (
                            <span className="wedding-scene-progress">
                                {task?.stage || "正在生成场景"}
                                {typeof progress === "number" ? ` · ${Math.round(progress)}%` : ""}
                            </span>
                        ) : null}
                    </div>
                    {dynasty.sceneVersions?.length ? (
                        <div className="wedding-scene-versions">
                            <span>历史版本</span>
                            <div>
                                {dynasty.sceneVersions.map((version, index) => (
                                    <button
                                        key={`${version.assetId}-${index}`}
                                        type="button"
                                        className={version.assetId === dynasty.sceneSelection ? "is-selected" : ""}
                                        disabled={saving || running}
                                        aria-label={`选择${dynasty.name}场景版本 ${index + 1}`}
                                        onClick={() => void onSelectVersion(version.assetId)}
                                    >
                                        {version.previewUrl ? <img src={version.previewUrl} alt="" /> : <ImageOff />}
                                        <span>V{index + 1}</span>
                                    </button>
                                ))}
                            </div>
                        </div>
                    ) : null}
                </section>

                <section className="wedding-scene-qc-panel">
                    <header>
                        <div>
                            <ScanSearch />
                            <span>
                                <strong>自动 QC</strong>
                                <small>{qc ? `检查于 ${formatTime(qc.checkedAt)}` : "尚未执行视觉检查"}</small>
                            </span>
                        </div>
                        <button type="button" disabled={saving || running || !previewUrl} onClick={() => void onRunQc()}>
                            <ScanSearch />
                            {qc ? "重新 QC" : "运行 QC"}
                        </button>
                    </header>

                    {qc ? <QcResult qc={qc} /> : (
                        <div className="wedding-scene-qc-empty">
                            <AlertTriangle />
                            <span>没有 QC 记录。请运行模型检查，或明确填写覆盖原因。</span>
                        </div>
                    )}

                    {dynasty.qc.overrides.scene ? (
                        <div className="wedding-scene-qc-override-record">
                            <strong>已人工覆盖</strong>
                            <span>{dynasty.qc.overrides.scene.reason}</span>
                        </div>
                    ) : null}

                    <label className="wedding-scene-override">
                        <span>覆盖原因</span>
                        <textarea
                            rows={3}
                            value={overrideReason}
                            placeholder="仅在确认瑕疵不影响交付时填写"
                            aria-label={`${dynasty.name}场景QC覆盖原因`}
                            onChange={(event) => onOverrideReasonChange(event.currentTarget.value)}
                        />
                    </label>
                    <button
                        type="button"
                        className="wedding-scene-override-button"
                        disabled={saving || running || !overrideReason.trim()}
                        onClick={() => void onOverrideQc(overrideReason)}
                    >
                        <AlertTriangle />
                        记录人工覆盖
                    </button>
                </section>
            </div>

            <footer>
                <button type="button" disabled={saving || running || !node} onClick={() => void onGenerate()}>
                    <RefreshCw />
                    {previewUrl ? "重新生成场景" : "生成场景首帧"}
                </button>
                <span>{qcReady ? "QC 条件已满足" : "先完成 QC 或记录覆盖"}</span>
                <button
                    type="button"
                    className="is-primary"
                    disabled={saving || running || !dynasty.sceneSelection || !qcReady}
                    onClick={() => void onConfirm(dynasty.sceneSelection || "")}
                >
                    <Check />
                    确认此场景
                </button>
            </footer>
        </article>
    );
}

function ReferenceThumb({
    label,
    version,
}: {
    label: string;
    version?: { previewUrl?: string };
}) {
    return (
        <div>
            {version?.previewUrl ? <img src={version.previewUrl} alt={label} /> : <UserRound />}
            <span>{label}</span>
        </div>
    );
}

function QcResult({ qc }: { qc: WeddingSceneQc }) {
    const checks = [
        ["双人同时出现", qc.couplePresent, false],
        ["新郎脸部匹配", qc.faceMatchMale, false],
        ["新娘脸部匹配", qc.faceMatchFemale, false],
        ["服饰一致", qc.outfitMatch, false],
        ["场景匹配", qc.sceneMatch, false],
        ["无额外人物", qc.extraPerson, true],
        ["无文字水印", qc.watermarkOrText, true],
    ] as const;
    return (
        <div className={`wedding-scene-qc-result is-${qc.verdict}`}>
            <div>
                <strong>{qc.verdict === "pass" ? "QC 通过" : qc.verdict === "error" ? "QC 执行错误" : "QC 未通过"}</strong>
                <span>{qc.reason}</span>
            </div>
            <div className="wedding-scene-qc-checks">
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
            {qc.suggestion ? <p>{qc.suggestion}</p> : null}
        </div>
    );
}

function isTaskRunning(task?: GenerationTask) {
    return task?.status === "queued" || task?.status === "running";
}

function sceneReady(dynasty: WeddingDynastyState) {
    const qcReady = dynasty.qc.scene?.verdict === "pass" || Boolean(dynasty.qc.overrides.scene?.reason.trim());
    return Boolean(dynasty.sceneSelection) && qcReady;
}

function statusLabel(status: WeddingDynastyState["statuses"]["scene"]) {
    const labels: Record<WeddingDynastyState["statuses"]["scene"], string> = {
        not_started: "未开始",
        ready: "待审核",
        generating: "生成中",
        needs_review: "待审核",
        approved: "已审核",
        failed: "生成失败",
        stale: "需更新",
    };
    return labels[status];
}

function formatTime(value: string) {
    try {
        return new Date(value).toLocaleString("zh-CN", { hour12: false });
    } catch {
        return value;
    }
}
