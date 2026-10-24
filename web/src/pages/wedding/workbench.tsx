import { LoaderCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router";

import { openLocalCanvasProjectFromBackend } from "@/services/local-workspace-repository";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { WEDDING_STAGE_IDS, type WeddingItemStatus, type WeddingStageId } from "@/lib/wedding/types";

import "./wedding-workbench.css";

const stageLabels: Record<WeddingStageId, string> = {
    setup: "项目设定",
    dynasties: "朝代规划",
    portraits: "定妆照",
    scenes: "场景首帧",
    videos: "视频导演",
    packaging: "包装制作",
    delivery: "成片交付",
};

const statusLabels: Record<WeddingItemStatus, string> = {
    not_started: "未开始",
    ready: "可执行",
    generating: "生成中",
    needs_review: "待审",
    approved: "已确认",
    failed: "失败",
    stale: "已失效",
};

export default function WeddingWorkbenchPage() {
    const { canvasId = "" } = useParams();
    const project = useCanvasStore((state) => state.projects.find((item) => item.id === canvasId));
    const [loading, setLoading] = useState(!project);
    const [loadError, setLoadError] = useState("");
    const [activeStage, setActiveStage] = useState<WeddingStageId | null>(null);
    const [selectedDynastyId, setSelectedDynastyId] = useState("");

    useEffect(() => {
        if (!canvasId || project) {
            setLoading(false);
            return;
        }
        let cancelled = false;
        setLoading(true);
        setLoadError("");
        void openLocalCanvasProjectFromBackend(canvasId)
            .catch((error) => {
                if (!cancelled) {
                    setLoadError(error instanceof Error ? error.message : "工作台项目读取失败");
                }
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [canvasId, project]);

    const production = project?.weddingProduction;
    const resolvedStage = activeStage || production?.activeStage || "setup";
    const resolvedDynastyId = selectedDynastyId || production?.dynasties[0]?.id || "";
    const resolvedDynasty = useMemo(
        () => production?.dynasties.find((dynasty) => dynasty.id === resolvedDynastyId),
        [production?.dynasties, resolvedDynastyId],
    );

    if (loading) {
        return (
            <main className="wedding-workbench wedding-workbench-centered">
                <LoaderCircle className="wedding-spinner" />
                <span>正在读取婚礼制作项目</span>
            </main>
        );
    }

    if (loadError || !project) {
        return (
            <main className="wedding-workbench wedding-workbench-centered">
                <strong>工作台无法打开</strong>
                <span>{loadError || "画布项目不存在或无权访问"}</span>
                <Link to="/">返回首页</Link>
            </main>
        );
    }

    if (!production) {
        return (
            <main className="wedding-workbench wedding-workbench-centered">
                <strong>该项目没有婚礼制作状态</strong>
                <span>请从“跨时空婚礼电影”工作流重新创建项目。</span>
                <Link to={`/canvas/${encodeURIComponent(project.id)}`}>返回画布</Link>
            </main>
        );
    }

    return (
        <main className="wedding-workbench">
            <aside className="wedding-workbench-rail">
                <div className="wedding-workbench-brand">
                    <span className="wedding-workbench-mark">V</span>
                    <div>
                        <strong>VisiDeep 制作台</strong>
                        <small>跨时空婚礼电影</small>
                    </div>
                </div>
                <div className="wedding-workbench-project">
                    <small>当前项目</small>
                    <strong>{project.title}</strong>
                    <span>{production.source.maleName} × {production.source.femaleName} · {production.dynasties.length} 个朝代</span>
                </div>
                <nav className="wedding-workbench-steps" aria-label="婚礼制作阶段">
                    {WEDDING_STAGE_IDS.map((stage, index) => {
                        const stageState = production.stages[stage];
                        return (
                            <button
                                key={stage}
                                type="button"
                                aria-current={resolvedStage === stage ? "step" : undefined}
                                className={`wedding-workbench-step ${resolvedStage === stage ? "is-active" : ""}`}
                                onClick={() => setActiveStage(stage)}
                            >
                                <span>{String(index + 1).padStart(2, "0")}</span>
                                <div>
                                    <strong>{stageLabels[stage]}</strong>
                                    <small>{statusLabels[stageState.status]}</small>
                                </div>
                            </button>
                        );
                    })}
                </nav>
            </aside>

            <section className="wedding-workbench-workspace">
                <header className="wedding-workbench-topbar">
                    <div>
                        <span>制作方工作台</span>
                        <strong>{stageLabels[resolvedStage]}</strong>
                    </div>
                    <Link to={`/canvas/${encodeURIComponent(project.id)}`}>打开底层画布</Link>
                </header>

                <div className="wedding-workbench-heading">
                    <div>
                        <small>{String(WEDDING_STAGE_IDS.indexOf(resolvedStage) + 1).padStart(2, "0")} / 07</small>
                        <h1>{stageLabels[resolvedStage]}</h1>
                        <p>当前页面使用真实画布生产状态，不创建演示项目或虚拟生成结果。</p>
                    </div>
                    <span className={`wedding-workbench-status is-${production.stages[resolvedStage].status}`}>
                        {statusLabels[production.stages[resolvedStage].status]}
                    </span>
                </div>

                <div className="wedding-workbench-grid">
                    <section className="wedding-workbench-panel">
                        <header>
                            <strong>项目概况</strong>
                            <span>{production.dynasties.length} 个朝代</span>
                        </header>
                        <dl className="wedding-workbench-facts">
                            <div><dt>人物</dt><dd>{production.source.maleName} × {production.source.femaleName}</dd></div>
                            <div><dt>画幅</dt><dd>{production.source.ratio}</dd></div>
                            <div><dt>分辨率</dt><dd>{production.source.resolution}</dd></div>
                            <div><dt>照片授权</dt><dd>{production.source.consentConfirmed ? "已确认" : "待确认"}</dd></div>
                        </dl>
                        <div className="wedding-workbench-dynasties">
                            {production.dynasties.map((dynasty) => (
                                <button
                                    key={dynasty.id}
                                    type="button"
                                    className={dynasty.id === resolvedDynastyId ? "is-active" : ""}
                                    onClick={() => setSelectedDynastyId(dynasty.id)}
                                >
                                    <strong>{dynasty.name}</strong>
                                    <small>{statusLabels[dynasty.statuses.video]}</small>
                                </button>
                            ))}
                        </div>
                    </section>

                    <aside className="wedding-workbench-panel">
                        <header>
                            <strong>{resolvedDynasty ? `${resolvedDynasty.name} · 当前状态` : "当前状态"}</strong>
                            <span>{resolvedDynasty ? "真实项目数据" : "无朝代"}</span>
                        </header>
                        {resolvedDynasty ? (
                            <div className="wedding-workbench-state-list">
                                <div><span>定妆照</span><strong>{statusLabels[resolvedDynasty.statuses.portrait]}</strong></div>
                                <div><span>场景首帧</span><strong>{statusLabels[resolvedDynasty.statuses.scene]}</strong></div>
                                <div><span>视频导演</span><strong>{statusLabels[resolvedDynasty.statuses.video]}</strong></div>
                                <div><span>提示词覆盖</span><strong>{resolvedDynasty.promptOverride ? "已启用" : "使用模板"}</strong></div>
                            </div>
                        ) : <p className="wedding-workbench-empty">当前项目没有可用朝代。</p>}
                    </aside>
                </div>
            </section>
        </main>
    );
}
