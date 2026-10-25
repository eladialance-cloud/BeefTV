import { LoaderCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Link, Navigate, useParams } from "react-router";

import { openLocalCanvasProjectFromBackend, persistCanvasDocument } from "@/services/local-workspace-repository";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { WEDDING_STAGE_IDS, type WeddingStageId } from "@/lib/wedding/types";
import { clearWeddingPromptOverride, setWeddingPromptOverride } from "@/lib/wedding/prompt";
import { legacyWeddingWorkbenchPath } from "@/lib/wedding/routing";
import { VideoPromptInspector } from "@/pages/wedding/video-prompt-inspector";
import { WorkbenchStageRail, weddingStageLabels, weddingStatusLabels } from "@/pages/wedding/workbench-stage-rail";
import { WorkbenchTopBar } from "@/pages/wedding/workbench-top-bar";

import "./wedding-workbench.css";

export function WeddingWorkbenchSurface({
    canvasId,
    embedded = false,
}: {
    canvasId: string;
    embedded?: boolean;
}) {
    const project = useCanvasStore((state) => state.projects.find((item) => item.id === canvasId));
    const [loading, setLoading] = useState(!project);
    const [loadError, setLoadError] = useState("");
    const [activeStage, setActiveStage] = useState<WeddingStageId | null>(null);
    const [selectedDynastyId, setSelectedDynastyId] = useState("");
    const [promptSaving, setPromptSaving] = useState(false);
    const [promptFeedback, setPromptFeedback] = useState("");

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
    const promptVariables = useMemo(() => production ? {
        maleName: production.source.maleName,
        femaleName: production.source.femaleName,
        customDesc: production.source.story,
        dynasties: production.dynasties.map((dynasty) => dynasty.id),
        ratio: production.source.ratio,
        resolution: production.source.resolution,
        consentConfirmed: production.source.consentConfirmed,
    } : {}, [production]);

    if (loading) {
        return (
            <main className={`wedding-workbench wedding-workbench-centered ${embedded ? "wedding-workbench-embedded" : ""}`}>
                <LoaderCircle className="wedding-spinner" />
                <span>正在读取婚礼制作项目</span>
            </main>
        );
    }

    if (loadError || !project) {
        return (
            <main className={`wedding-workbench wedding-workbench-centered ${embedded ? "wedding-workbench-embedded" : ""}`}>
                <strong>工作台无法打开</strong>
                <span>{loadError || "画布项目不存在或无权访问"}</span>
                <Link to="/">返回首页</Link>
            </main>
        );
    }

    if (!production) {
        return (
            <main className={`wedding-workbench wedding-workbench-centered ${embedded ? "wedding-workbench-embedded" : ""}`}>
                <strong>该项目没有婚礼制作状态</strong>
                <span>请从“跨时空婚礼电影”工作流重新创建项目。</span>
                <Link to={`/canvas/${encodeURIComponent(project.id)}`}>返回画布</Link>
            </main>
        );
    }

    const savePrompt = async (prompt: string) => {
        if (!production || !resolvedDynasty) return;
        setPromptSaving(true);
        setPromptFeedback("");
        try {
            const next = setWeddingPromptOverride(
                production,
                resolvedDynasty.id,
                prompt,
                new Date().toISOString(),
            );
            await persistCanvasDocument(project.id, { weddingProduction: next });
            setPromptFeedback("提示词已保存，当前视频和最终成片已标记为失效。");
        } catch (error) {
            setPromptFeedback(error instanceof Error ? error.message : "提示词保存失败");
        } finally {
            setPromptSaving(false);
        }
    };

    const restorePrompt = async () => {
        if (!production || !resolvedDynasty) return;
        setPromptSaving(true);
        setPromptFeedback("");
        try {
            const next = clearWeddingPromptOverride(
                production,
                resolvedDynasty.id,
                new Date().toISOString(),
            );
            await persistCanvasDocument(project.id, { weddingProduction: next });
            setPromptFeedback("已恢复朝代默认提示词。");
        } catch (error) {
            setPromptFeedback(error instanceof Error ? error.message : "恢复默认提示词失败");
        } finally {
            setPromptSaving(false);
        }
    };

    return (
        <main className={`wedding-workbench ${embedded ? "wedding-workbench-embedded" : ""}`}>
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
                <WorkbenchStageRail
                    state={production}
                    activeStage={resolvedStage}
                    onSelect={setActiveStage}
                />
            </aside>

            <section className="wedding-workbench-workspace">
                {embedded ? null : <WorkbenchTopBar canvasId={project.id} stageLabel={weddingStageLabels[resolvedStage]} />}

                <div className="wedding-workbench-heading">
                    <div>
                        <small>{String(WEDDING_STAGE_IDS.indexOf(resolvedStage) + 1).padStart(2, "0")} / 07</small>
                        <h1>{weddingStageLabels[resolvedStage]}</h1>
                        <p>当前页面使用真实画布生产状态，不创建演示项目或虚拟生成结果。</p>
                    </div>
                    <span className={`wedding-workbench-status is-${production.stages[resolvedStage].status}`}>
                        {weddingStatusLabels[production.stages[resolvedStage].status]}
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
                                    <small>{weddingStatusLabels[dynasty.statuses.video]}</small>
                                </button>
                            ))}
                        </div>
                    </section>

                    {resolvedStage === "videos" && resolvedDynasty ? (
                        <VideoPromptInspector
                            templateId={production.template.id}
                            dynastyId={resolvedDynasty.id}
                            dynastyName={resolvedDynasty.name}
                            variables={promptVariables}
                            production={production}
                            saving={promptSaving}
                            feedback={promptFeedback}
                            onSave={savePrompt}
                            onRestore={restorePrompt}
                        />
                    ) : (
                        <aside className="wedding-workbench-panel">
                            <header>
                                <strong>{resolvedDynasty ? `${resolvedDynasty.name} · 当前状态` : "当前状态"}</strong>
                                <span>{resolvedDynasty ? "真实项目数据" : "无朝代"}</span>
                            </header>
                            {resolvedDynasty ? (
                                <div className="wedding-workbench-state-list">
                                    <div><span>定妆照</span><strong>{weddingStatusLabels[resolvedDynasty.statuses.portrait]}</strong></div>
                                    <div><span>场景首帧</span><strong>{weddingStatusLabels[resolvedDynasty.statuses.scene]}</strong></div>
                                    <div><span>视频导演</span><strong>{weddingStatusLabels[resolvedDynasty.statuses.video]}</strong></div>
                                    <div><span>提示词覆盖</span><strong>{resolvedDynasty.promptOverride ? "已启用" : "使用模板"}</strong></div>
                                </div>
                            ) : <p className="wedding-workbench-empty">当前项目没有可用朝代。</p>}
                        </aside>
                    )}
                </div>
            </section>
        </main>
    );
}

export default function WeddingWorkbenchPage() {
    const { canvasId = "" } = useParams();
    return <Navigate to={legacyWeddingWorkbenchPath(canvasId)} replace />;
}
