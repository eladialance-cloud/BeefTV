import {
    Check,
    Film,
    Image as ImageIcon,
    LoaderCircle,
    Music2,
    Play,
    Save,
    Sparkles,
    Upload,
} from "lucide-react";
import { useEffect, useState } from "react";

import type { WeddingPackagingPatch } from "@/lib/wedding/packaging-runtime";
import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";
import type { GenerationTask } from "@/services/api/task-center";
import type { CanvasNodeData } from "@/types/canvas";
import type { TimelineClip, TimelineProject } from "@/types/timeline";

export function PackagingStage({
    production,
    nodes,
    timeline,
    saving,
    feedback,
    bgmTask,
    onSave,
    onGenerateTitles,
    onGenerateBgm,
    onUploadBgm,
    onBuildTimeline,
    onApprove,
}: {
    production: WeddingProductionState;
    nodes: CanvasNodeData[];
    timeline?: TimelineProject;
    saving: boolean;
    feedback: string;
    bgmTask?: GenerationTask;
    onSave: (patch: WeddingPackagingPatch) => Promise<void>;
    onGenerateTitles: () => Promise<void>;
    onGenerateBgm: () => Promise<void>;
    onUploadBgm: (file: File) => Promise<void>;
    onBuildTimeline: () => Promise<void>;
    onApprove: () => Promise<void>;
}) {
    const [draft, setDraft] = useState(production.packaging);
    useEffect(() => setDraft(production.packaging), [production.packaging]);
    const blockers = weddingStageBlockers(production, "packaging");
    const nodeByID = new Map(nodes.map((node) => [node.id, node]));
    const intro = nodeByID.get(production.packaging.titleCardNodeIds[0] || "");
    const outro = nodeByID.get(production.packaging.titleCardNodeIds[1] || "");
    const bgm = nodeByID.get(production.packaging.bgmNodeId || "");
    const dirty = JSON.stringify(draft) !== JSON.stringify(production.packaging);
    const visualClips = (timeline?.clips || [])
        .filter((clip) => clip.kind !== "audio")
        .sort((left, right) => left.startMs - right.startMs);
    const [previewIndex, setPreviewIndex] = useState(0);
    const currentClip = visualClips[Math.min(previewIndex, Math.max(0, visualClips.length - 1))];
    const bgmRunning = bgmTask?.status === "queued" || bgmTask?.status === "running";

    const patch = (next: Partial<WeddingProductionState["packaging"]>) => {
        setDraft((current) => ({ ...current, ...next }));
    };

    return (
        <section className="wedding-packaging-stage">
            <div className="wedding-packaging-overview">
                <div>
                    <small>06 · PACKAGING</small>
                    <strong>标题、声音与成片节奏</strong>
                    <span>包装只影响正式成片，不重新生成已确认的朝代视频。</span>
                </div>
                <div>
                    <span>{timeline?.clips.filter((clip) => clip.kind !== "audio").length || 0}</span>
                    <small>时间线视觉片段</small>
                </div>
            </div>

            <div className="wedding-packaging-layout">
                <section className="wedding-packaging-settings">
                    <header>
                        <div><ImageIcon /><span><strong>片头与片尾</strong><small>标题卡会烧录进正式成片</small></span></div>
                        <button type="button" disabled={saving} onClick={() => void onGenerateTitles()}>
                            <Sparkles />
                            生成标题卡
                        </button>
                    </header>
                    <div className="wedding-packaging-copy-fields">
                        <label>
                            <span>片头文案</span>
                            <input
                                value={draft.intro.text}
                                onChange={(event) => patch({ intro: { ...draft.intro, text: event.currentTarget.value } })}
                            />
                        </label>
                        <label>
                            <span>片头时长（秒）</span>
                            <input
                                type="number"
                                min={1}
                                max={15}
                                value={draft.intro.durationSeconds}
                                onChange={(event) => patch({ intro: { ...draft.intro, durationSeconds: Number(event.currentTarget.value) } })}
                            />
                        </label>
                        <label>
                            <span>片尾文案</span>
                            <input
                                value={draft.outro.text}
                                onChange={(event) => patch({ outro: { ...draft.outro, text: event.currentTarget.value } })}
                            />
                        </label>
                        <label>
                            <span>片尾时长（秒）</span>
                            <input
                                type="number"
                                min={1}
                                max={15}
                                value={draft.outro.durationSeconds}
                                onChange={(event) => patch({ outro: { ...draft.outro, durationSeconds: Number(event.currentTarget.value) } })}
                            />
                        </label>
                    </div>
                    <div className="wedding-packaging-title-cards">
                        <TitleCardPreview label="片头标题卡" url={intro?.metadata?.content} />
                        <TitleCardPreview label="片尾标题卡" url={outro?.metadata?.content} />
                    </div>
                    <div className="wedding-packaging-toggles">
                        <label>
                            <span>字幕</span>
                            <input
                                type="checkbox"
                                checked={draft.subtitlesEnabled}
                                onChange={(event) => patch({ subtitlesEnabled: event.currentTarget.checked })}
                            />
                        </label>
                        <label>
                            <span>转场（秒）</span>
                            <input
                                type="number"
                                min={0}
                                max={3}
                                step={0.5}
                                value={draft.transitionSeconds}
                                onChange={(event) => patch({ transitionSeconds: Number(event.currentTarget.value) })}
                            />
                        </label>
                    </div>
                </section>

                <section className="wedding-packaging-audio">
                    <header>
                        <div><Music2 /><span><strong>背景音乐</strong><small>自动生成、上传或关闭</small></span></div>
                        {bgmRunning ? <LoaderCircle className="wedding-spinner" /> : null}
                    </header>
                    <div className="wedding-packaging-bgm-modes">
                        {([
                            ["generated", "自动生成"],
                            ["uploaded", "上传 BGM"],
                            ["none", "关闭"],
                        ] as const).map(([value, label]) => (
                            <button
                                key={value}
                                type="button"
                                className={draft.bgmMode === value ? "is-selected" : ""}
                                onClick={() => patch({ bgmMode: value })}
                            >
                                {label}
                            </button>
                        ))}
                    </div>
                    {bgm?.metadata?.content ? (
                        <audio controls preload="metadata" src={bgm.metadata.content} />
                    ) : (
                        <div className="wedding-packaging-bgm-empty">
                            <Music2 />
                            <span>{draft.bgmMode === "none" ? "本项目不添加 BGM" : "尚无 BGM 资源"}</span>
                        </div>
                    )}
                    <div className="wedding-packaging-audio-actions">
                        <button type="button" disabled={saving || bgmRunning || draft.bgmMode !== "generated"} onClick={() => void onGenerateBgm()}>
                            <Sparkles />
                            生成 BGM
                        </button>
                        <label className={saving || bgmRunning || draft.bgmMode !== "uploaded" ? "is-disabled" : ""}>
                            <Upload />
                            上传 BGM
                            <input
                                type="file"
                                accept="audio/*,.mp3,.wav"
                                disabled={saving || bgmRunning || draft.bgmMode !== "uploaded"}
                                onChange={(event) => {
                                    const file = event.currentTarget.files?.[0];
                                    if (file) void onUploadBgm(file);
                                }}
                            />
                        </label>
                    </div>
                    {bgmTask?.error ? <p className="wedding-packaging-error">{bgmTask.error}</p> : null}
                </section>
            </div>

            <section className="wedding-packaging-preview">
                <header>
                    <div>
                        <Play />
                        <span><strong>时间线预览</strong><small>{timeline ? `${formatDuration(timeline.durationMs)} · ${timeline.clips.length} 个片段` : "尚未构建时间线"}</small></span>
                    </div>
                    <div>
                        <button type="button" disabled={saving || dirty} onClick={() => void onBuildTimeline()}>
                            <Film />
                            构建时间线
                        </button>
                    </div>
                </header>
                <PackagingMediaPreview clip={currentClip} />
                <div className="wedding-packaging-timeline-strip">
                    {visualClips.length ? visualClips.map((clip, index) => (
                        <button
                            key={clip.id}
                            type="button"
                            className={index === previewIndex ? "is-active" : ""}
                            style={{ flexGrow: Math.max(1, clip.durationMs / 1000) }}
                            onClick={() => setPreviewIndex(index)}
                        >
                            <span>{clip.kind === "image" ? <ImageIcon /> : <Film />}</span>
                            <strong>{clip.title || clip.kind}</strong>
                            <small>{formatDuration(clip.durationMs)}</small>
                        </button>
                    )) : <p>生成标题卡并确认至少一个朝代视频后构建时间线。</p>}
                </div>
            </section>

            <footer className="wedding-stage-actions wedding-packaging-actions">
                <div>
                    <span>{feedback}</span>
                    {blockers.length ? <small>{blockers.slice(0, 2).join("；")}</small> : <small>包装素材与时间线预览已就绪。</small>}
                </div>
                <div>
                    <button type="button" disabled={saving || !dirty} onClick={() => void onSave(draft)}>
                        <Save />
                        保存包装设置
                    </button>
                    <button
                        type="button"
                        className="is-primary"
                        disabled={saving || blockers.length > 0}
                        onClick={() => void onApprove()}
                    >
                        <Check />
                        确认包装阶段
                    </button>
                </div>
            </footer>
        </section>
    );
}

function TitleCardPreview({ label, url }: { label: string; url?: string }) {
    return (
        <div className={url ? "has-image" : ""}>
            {url ? <img src={url} alt={label} /> : <ImageIcon />}
            <span>{label}</span>
        </div>
    );
}

function PackagingMediaPreview({ clip }: { clip?: TimelineClip }) {
    if (!clip) {
        return (
            <div className="wedding-packaging-preview-empty">
                <Film />
                <strong>尚未构建时间线</strong>
                <span>片头、朝代视频、片尾和 BGM 将在这里形成完整预览。</span>
            </div>
        );
    }
    const url = clip.directMedia?.url || clip.directMedia?.dataUrl || "";
    if (clip.kind === "video") {
        return <video className="wedding-packaging-media" controls preload="metadata" src={url} />;
    }
    return url
        ? <img className="wedding-packaging-media" src={url} alt={clip.title || "时间线画面"} />
        : <div className="wedding-packaging-preview-empty"><ImageIcon /><strong>画面资源不可用</strong></div>;
}

function formatDuration(durationMs: number) {
    const seconds = Math.max(0, Math.round(durationMs / 1000));
    return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
