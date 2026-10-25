import {
    Check,
    ImageOff,
    LoaderCircle,
    RefreshCw,
    Sparkles,
    UserRound,
} from "lucide-react";

import {
    weddingPortraitJobs,
} from "@/lib/wedding/portrait-runtime";
import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type {
    WeddingDynastyState,
    WeddingPortraitGender,
    WeddingPortraitVersion,
    WeddingProductionState,
} from "@/lib/wedding/types";
import type { GenerationTask } from "@/services/api/task-center";
import type { CanvasNodeData } from "@/types/canvas";

export function PortraitStage({
    production,
    nodes,
    tasks,
    saving,
    feedback,
    onGenerateAll,
    onGenerateJob,
    onSelectVersion,
    onConfirmPortrait,
    onApprove,
}: {
    production: WeddingProductionState;
    nodes: CanvasNodeData[];
    tasks: Record<string, GenerationTask>;
    saving: boolean;
    feedback: string;
    onGenerateAll: () => Promise<void>;
    onGenerateJob: (nodeId: string) => Promise<void>;
    onSelectVersion: (
        dynastyId: string,
        gender: WeddingPortraitGender,
        assetId: string,
    ) => Promise<void>;
    onConfirmPortrait: (
        dynastyId: string,
        gender: WeddingPortraitGender,
        assetId: string,
    ) => Promise<void>;
    onApprove: () => Promise<void>;
}) {
    const jobs = weddingPortraitJobs(production);
    const blockers = weddingStageBlockers(production, "portraits");
    const nodeByID = new Map(nodes.map((node) => [node.id, node]));
    const activeTask = jobs.some((job) => isTaskRunning(tasks[job.nodeId]));

    return (
        <section className="wedding-portrait-stage">
            <div className="wedding-portrait-overview">
                <div>
                    <small>03 · PORTRAITS</small>
                    <strong>逐朝代确认人物身份与服饰</strong>
                    <span>男女两张分别确认后，当前朝代才算审核完成。</span>
                </div>
                <div>
                    <span>{approvedDynastyCount(production)} / {production.dynasties.length}</span>
                    <small>朝代已审核</small>
                </div>
            </div>

            <div className="wedding-portrait-dynasties">
                {production.dynasties.map((dynasty) => (
                    <DynastyPortraitBoard
                        key={dynasty.id}
                        dynasty={dynasty}
                        nodeByID={nodeByID}
                        tasks={tasks}
                        saving={saving}
                        onGenerateJob={onGenerateJob}
                        onSelectVersion={onSelectVersion}
                        onConfirmPortrait={onConfirmPortrait}
                    />
                ))}
            </div>

            <footer className="wedding-stage-actions wedding-portrait-actions">
                <div>
                    <span>{feedback}</span>
                    {blockers.length ? (
                        <small>{blockers.slice(0, 2).join("；")}{blockers.length > 2 ? `；另 ${blockers.length - 2} 项` : ""}</small>
                    ) : (
                        <small>全部朝代男女定妆照已确认，可以推进到场景首帧。</small>
                    )}
                </div>
                <div>
                    <button type="button" disabled={saving || activeTask} onClick={() => void onGenerateAll()}>
                        {activeTask ? <LoaderCircle className="wedding-spinner" /> : <Sparkles />}
                        生成缺失定妆照
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || activeTask || blockers.length > 0}
                        onClick={() => void onApprove()}
                    >
                        <Check />
                        确认定妆照阶段
                    </button>
                </div>
            </footer>
        </section>
    );
}

function DynastyPortraitBoard({
    dynasty,
    nodeByID,
    tasks,
    saving,
    onGenerateJob,
    onSelectVersion,
    onConfirmPortrait,
}: {
    dynasty: WeddingDynastyState;
    nodeByID: Map<string, CanvasNodeData>;
    tasks: Record<string, GenerationTask>;
    saving: boolean;
    onGenerateJob: (nodeId: string) => Promise<void>;
    onSelectVersion: (
        dynastyId: string,
        gender: WeddingPortraitGender,
        assetId: string,
    ) => Promise<void>;
    onConfirmPortrait: (
        dynastyId: string,
        gender: WeddingPortraitGender,
        assetId: string,
    ) => Promise<void>;
}) {
    const approved = dynasty.portraitApproval?.male && dynasty.portraitApproval?.female;
    return (
        <article className="wedding-portrait-dynasty">
            <header>
                <div>
                    <span>{String(dynasty.order + 1).padStart(2, "0")}</span>
                    <div>
                        <strong>{dynasty.name}</strong>
                        <small>{approved ? "男女定妆照均已确认" : "等待逐人审核"}</small>
                    </div>
                </div>
                <span className={approved ? "is-approved" : ""}>
                    {approved ? <Check /> : null}
                    {approved ? "已审核" : statusLabel(dynasty.statuses.portrait)}
                </span>
            </header>
            <div className="wedding-portrait-pair">
                <PortraitSlot
                    dynasty={dynasty}
                    gender="male"
                    node={nodeByID.get(dynasty.portraitMaleNodeId)}
                    task={tasks[dynasty.portraitMaleNodeId]}
                    saving={saving}
                    onGenerate={() => onGenerateJob(dynasty.portraitMaleNodeId)}
                    onSelect={(assetId) => onSelectVersion(dynasty.id, "male", assetId)}
                    onConfirm={(assetId) => onConfirmPortrait(dynasty.id, "male", assetId)}
                />
                <PortraitSlot
                    dynasty={dynasty}
                    gender="female"
                    node={nodeByID.get(dynasty.portraitFemaleNodeId)}
                    task={tasks[dynasty.portraitFemaleNodeId]}
                    saving={saving}
                    onGenerate={() => onGenerateJob(dynasty.portraitFemaleNodeId)}
                    onSelect={(assetId) => onSelectVersion(dynasty.id, "female", assetId)}
                    onConfirm={(assetId) => onConfirmPortrait(dynasty.id, "female", assetId)}
                />
            </div>
        </article>
    );
}

function PortraitSlot({
    dynasty,
    gender,
    node,
    task,
    saving,
    onGenerate,
    onSelect,
    onConfirm,
}: {
    dynasty: WeddingDynastyState;
    gender: WeddingPortraitGender;
    node?: CanvasNodeData;
    task?: GenerationTask;
    saving: boolean;
    onGenerate: () => Promise<void>;
    onSelect: (assetId: string) => Promise<void>;
    onConfirm: (assetId: string) => Promise<void>;
}) {
    const label = gender === "male" ? "新郎定妆照" : "新娘定妆照";
    const selectedAssetId = dynasty.portraitSelection?.[gender] || "";
    const versions = dynasty.portraitVersionHistory?.[gender] || [];
    const selectedVersion = versions.find((version) => version.assetId === selectedAssetId);
    const previewUrl = selectedVersion?.previewUrl
        || (selectedAssetId && node?.metadata?.assetId === selectedAssetId ? node.metadata?.content : "")
        || (selectedAssetId ? versions.find((version) => version.assetId === selectedAssetId)?.previewUrl : "")
        || node?.metadata?.content
        || "";
    const running = isTaskRunning(task) || node?.metadata?.status === "loading";
    const progress = task?.progress ?? node?.metadata?.taskProgress;
    const approved = dynasty.portraitApproval?.[gender] === true;
    const error = task?.error || node?.metadata?.errorDetails;

    return (
        <section className="wedding-portrait-slot">
            <header>
                <div>
                    <UserRound />
                    <span>
                        <strong>{label}</strong>
                        <small>{node?.title || "画布节点缺失"}</small>
                    </span>
                </div>
                <span className={approved ? "is-approved" : running ? "is-running" : ""}>
                    {running ? <LoaderCircle className="wedding-spinner" /> : approved ? <Check /> : null}
                    {running ? "生成中" : approved ? "已确认" : selectedAssetId ? "待确认" : "未准备"}
                </span>
            </header>

            <div className={`wedding-portrait-preview${previewUrl ? " has-image" : ""}`}>
                {previewUrl ? (
                    <img src={previewUrl} alt={`${dynasty.name}${label}`} />
                ) : (
                    <div>
                        <ImageOff />
                        <strong>尚无定妆照</strong>
                        <span>{node ? "使用原始照片生成该朝代的男女定妆照" : "请先补齐画布肖像节点"}</span>
                    </div>
                )}
                {running ? (
                    <span className="wedding-portrait-progress">
                        {task?.stage || "正在生成"}
                        {typeof progress === "number" ? ` · ${Math.round(progress)}%` : ""}
                    </span>
                ) : null}
            </div>

            {error ? <p className="wedding-portrait-error">{error}</p> : null}

            {versions.length ? (
                <div className="wedding-portrait-versions">
                    <span>历史版本</span>
                    <div>
                        {versions.map((version, index) => (
                            <VersionButton
                                key={`${version.assetId}-${index}`}
                                version={version}
                                index={index}
                                selected={selectedAssetId === version.assetId}
                                disabled={saving || running}
                                onSelect={() => void onSelect(version.assetId)}
                            />
                        ))}
                    </div>
                </div>
            ) : null}

            <footer>
                <button type="button" disabled={saving || running || !node} onClick={() => void onGenerate()}>
                    <RefreshCw />
                    {previewUrl ? "重新生成" : "生成定妆照"}
                </button>
                <button
                    type="button"
                    className="is-primary"
                    disabled={saving || running || !selectedAssetId || approved}
                    onClick={() => void onConfirm(selectedAssetId)}
                >
                    <Check />
                    {approved ? "已确认使用" : "确认此定妆照"}
                </button>
            </footer>
        </section>
    );
}

function VersionButton({
    version,
    index,
    selected,
    disabled,
    onSelect,
}: {
    version: WeddingPortraitVersion;
    index: number;
    selected: boolean;
    disabled: boolean;
    onSelect: () => void;
}) {
    return (
        <button
            type="button"
            className={selected ? "is-selected" : ""}
            disabled={disabled}
            title={`选择版本 ${index + 1}`}
            aria-label={`选择版本 ${index + 1}`}
            onClick={onSelect}
        >
            {version.previewUrl ? <img src={version.previewUrl} alt="" /> : <ImageOff />}
            <span>V{index + 1}</span>
        </button>
    );
}

function isTaskRunning(task?: GenerationTask) {
    return task?.status === "queued" || task?.status === "running";
}

function approvedDynastyCount(production: WeddingProductionState) {
    return production.dynasties.filter((dynasty) => dynasty.portraitApproval?.male && dynasty.portraitApproval?.female).length;
}

function statusLabel(status: WeddingDynastyState["statuses"]["portrait"]) {
    const labels: Record<WeddingDynastyState["statuses"]["portrait"], string> = {
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
