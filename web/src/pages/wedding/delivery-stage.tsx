import {
    Check,
    Download,
    FileJson,
    Film,
    LoaderCircle,
    Play,
    RefreshCw,
    ShieldCheck,
} from "lucide-react";

import { weddingStageBlockers } from "@/lib/wedding/production-state";
import type { WeddingProductionState } from "@/lib/wedding/types";
import { resourceFileUrl } from "@/services/api/resources";
import type { GenerationTask } from "@/services/api/task-center";

export function DeliveryStage({
    production,
    renderTask,
    saving,
    feedback,
    onRender,
    onVerify,
    onConfirm,
    onExportManifest,
}: {
    production: WeddingProductionState;
    renderTask?: GenerationTask;
    saving: boolean;
    feedback: string;
    onRender: () => Promise<void>;
    onVerify: () => Promise<void>;
    onConfirm: () => Promise<void>;
    onExportManifest: () => void;
}) {
    const delivery = production.delivery;
    const resourceId = delivery?.resourceId;
    const videoUrl = resourceId ? resourceFileUrl(resourceId) : "";
    const rendering = delivery?.status === "rendering" || renderTask?.status === "running" || renderTask?.status === "queued";
    const progress = renderTask?.progress ?? (delivery?.status === "approved" ? 100 : 0);
    const blockers = weddingStageBlockers(production, "delivery");
    const approved = delivery?.status === "approved";

    return (
        <section className="wedding-delivery-stage">
            <div className="wedding-delivery-overview">
                <div>
                    <small>07 · DELIVERY</small>
                    <strong>正式成片与工程交付</strong>
                    <span>服务端 FFmpeg 渲染、可播放验证、MP4 下载和工程清单使用同一份时间线。</span>
                </div>
                <div>
                    <span>{approved ? "DONE" : rendering ? `${Math.round(progress)}%` : delivery?.status === "needs_review" ? "QC" : "READY"}</span>
                    <small>交付状态</small>
                </div>
            </div>

            <div className="wedding-delivery-layout">
                <section className="wedding-delivery-preview">
                    <header>
                        <div>
                            <Film />
                            <span>
                                <strong>正式成片</strong>
                                <small>{delivery?.fileName || "尚未生成 MP4"}</small>
                            </span>
                        </div>
                        <span className={approved ? "is-approved" : ""}>
                            {approved ? <Check /> : null}
                            {statusLabel(delivery?.status || "not_started")}
                        </span>
                    </header>
                    <div className="wedding-delivery-player">
                        {videoUrl ? (
                            <video
                                controls
                                preload="metadata"
                                src={videoUrl}
                                onLoadedMetadata={(event) => {
                                    event.currentTarget.dataset.durationMs = String(Math.round(event.currentTarget.duration * 1000));
                                }}
                            />
                        ) : (
                            <div>
                                <Play />
                                <strong>尚无正式成片</strong>
                                <span>完成包装预览后，提交服务端时间线渲染任务。</span>
                            </div>
                        )}
                        {rendering ? (
                            <div className="wedding-delivery-progress">
                                <LoaderCircle className="wedding-spinner" />
                                <span>{renderTask?.stage || "正在渲染成片"}</span>
                                <strong>{Math.round(progress)}%</strong>
                            </div>
                        ) : null}
                    </div>
                    <div className="wedding-delivery-facts">
                        <div><span>时长</span><strong>{delivery?.durationMs ? formatDuration(delivery.durationMs) : "待渲染"}</strong></div>
                        <div><span>文件大小</span><strong>{delivery?.size ? formatBytes(delivery.size) : "待渲染"}</strong></div>
                        <div><span>渲染任务</span><strong>{delivery?.taskId || "待创建"}</strong></div>
                        <div><span>资源 ID</span><strong>{resourceId || "待生成"}</strong></div>
                    </div>
                    <footer>
                        <button type="button" disabled={saving || rendering} onClick={() => void onRender()}>
                            {rendering ? <LoaderCircle className="wedding-spinner" /> : <RefreshCw />}
                            {delivery?.resourceId ? "重新渲染成片" : "渲染成片"}
                        </button>
                        <button type="button" disabled={saving || rendering || !videoUrl || Boolean(delivery?.playableVerifiedAt)} onClick={() => void onVerify()}>
                            <ShieldCheck />
                            验证可播放
                        </button>
                    </footer>
                </section>

                <section className="wedding-delivery-qc">
                    <header>
                        <div>
                            <ShieldCheck />
                            <span><strong>交付 QC</strong><small>成片与上游资产完整性</small></span>
                        </div>
                    </header>
                    <div className="wedding-delivery-qc-list">
                        <QcRow label="时间线渲染成功" passed={Boolean(resourceId)} value={delivery?.taskId} />
                        <QcRow label="成片可播放" passed={Boolean(delivery?.playableVerifiedAt)} value={delivery?.playableVerifiedAt ? "已验证" : "待验证"} />
                        <QcRow label="全部视频已确认" passed={production.dynasties.every((dynasty) => dynasty.statuses.video === "approved")} value={`${production.dynasties.filter((dynasty) => dynasty.statuses.video === "approved").length}/${production.dynasties.length}`} />
                        <QcRow label="包装已确认" passed={production.packaging.status === "approved"} value={statusLabel(production.packaging.status)} />
                        {delivery?.qc ? (
                            <div className="wedding-delivery-video-qc">
                                <span>{delivery.qc.width} × {delivery.qc.height}</span>
                                <span>{formatDuration(delivery.qc.durationMs)}</span>
                                <span>{delivery.qc.checkedAt}</span>
                            </div>
                        ) : null}
                    </div>

                    <div className="wedding-delivery-exports">
                        {videoUrl ? (
                            <a href={videoUrl} download={delivery?.fileName || "cross-era-wedding.mp4"}>
                                <Download />
                                下载 MP4
                            </a>
                        ) : (
                            <button type="button" disabled>
                                <Download />
                                下载 MP4
                            </button>
                        )}
                        <button type="button" onClick={onExportManifest}>
                            <FileJson />
                            导出工程清单
                        </button>
                    </div>

                    {delivery?.error ? <p className="wedding-delivery-error">{delivery.error}</p> : null}
                    {feedback ? <p className="wedding-delivery-feedback">{feedback}</p> : null}
                </section>
            </div>

            <footer className="wedding-stage-actions wedding-delivery-actions">
                <div>
                    <span>{approved ? "正式成片已交付。" : feedback}</span>
                    {blockers.length ? <small>{blockers.slice(0, 2).join("；")}</small> : <small>成片已通过可播放验证，可以完成最终确认。</small>}
                </div>
                <button
                    type="button"
                    className="is-primary"
                    disabled={saving || rendering || !delivery?.playableVerifiedAt || approved}
                    onClick={() => void onConfirm()}
                >
                    <Check />
                    {approved ? "已完成交付" : "确认最终成片"}
                </button>
            </footer>
        </section>
    );
}

function QcRow({ label, passed, value }: { label: string; passed: boolean; value?: string }) {
    return (
        <div>
            <span className={passed ? "is-pass" : "is-fail"}>{passed ? <Check /> : <Film />}</span>
            <strong>{label}</strong>
            <small>{value || (passed ? "通过" : "待完成")}</small>
        </div>
    );
}

function statusLabel(status: string) {
    const labels: Record<string, string> = {
        not_started: "未开始",
        ready: "可执行",
        generating: "生成中",
        rendering: "渲染中",
        needs_review: "待审核",
        approved: "已确认",
        failed: "失败",
        stale: "已失效",
    };
    return labels[status] || status;
}

function formatDuration(durationMs: number) {
    const seconds = Math.max(0, Math.round(durationMs / 1000));
    return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

function formatBytes(bytes: number) {
    if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
