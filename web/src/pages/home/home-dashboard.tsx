import { ArrowRight, LayoutTemplate, Plus, RefreshCw, Workflow } from "lucide-react";
import { Link, useNavigate } from "react-router";
import { parseCreationTemplatePayload, type PublicCreationTemplate } from "@/lib/canvas/creation-template-workflow";
import type { CanvasLibrarySummary } from "@/services/api/workspace-data";
import { ProjectPreview } from "@/components/canvas/canvas-project-card";
import { beefTVCapabilityItems } from "./home-data";
function formatDate(value: string) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "刚刚更新";
    return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(date).replaceAll("/", "-");
}

export function HomeDashboard({
    projects,
    loading,
    error,
    onRetry,
    workflows = [],
    workflowsLoading = false,
    workflowsError = false,
    onRetryWorkflows = () => {},
    onSelectWorkflow = () => {},
}: {
    projects: CanvasLibrarySummary[];
    loading: boolean;
    error: boolean;
    onRetry: () => void;
    workflows?: PublicCreationTemplate[];
    workflowsLoading?: boolean;
    workflowsError?: boolean;
    onRetryWorkflows?: () => void;
    onSelectWorkflow?: (template: PublicCreationTemplate) => void;
}) {
    const navigate = useNavigate();
    return (
        <main className="beeftv-home" aria-label="VisiDeep AI 首页">
            <section className="beeftv-home-hero" aria-label="新建画布">
                <span className="beeftv-home-dot-base" aria-hidden />
                <span className="beeftv-home-dot-flow" aria-hidden />
                <span className="beeftv-home-dot-flow beeftv-home-dot-flow-delay" aria-hidden />
                <button type="button" className="beeftv-home-create" onClick={() => navigate("/canvas?mode=new")}>
                    <span className="beeftv-home-create-icon"><Plus /></span>
                    <span className="beeftv-home-create-title">新建画布创作</span>
                </button>
            </section>

            <nav className="beeftv-capabilities" aria-label="创作能力">
                {beefTVCapabilityItems.map(({ id, label, detail, to, icon: Icon, disabled }) => (
                    disabled ? (
                        <span key={id} className="beeftv-capability is-disabled" aria-disabled="true" title={`${label}：${detail}`}>
                            <span className="beeftv-capability-icon"><Icon /></span>
                            <strong>{label}</strong>
                        </span>
                    ) : (
                        <Link key={id} to={to} className="beeftv-capability" title={detail}>
                            <span className="beeftv-capability-icon"><Icon /></span>
                            <strong>{label}</strong>
                        </Link>
                    )
                ))}
            </nav>

            <section className="beeftv-home-section beeftv-recents">
                <header className="beeftv-section-heading"><h2>最近项目</h2><Link to="/project">查看全部 <ArrowRight /></Link></header>
                {error ? (
                    <button className="beeftv-home-error" type="button" onClick={onRetry}><RefreshCw />画布读取失败，点击重试</button>
                ) : (
                    <div className="beeftv-recent-grid">
                        {loading ? Array.from({ length: 4 }, (_, index) => <div className="beeftv-recent-card is-loading" key={index} />) : projects.length ? projects.slice(0, 4).map((project) => {
                            return <Link to={`/canvas/${project.id}`} className="beeftv-recent-card" key={project.id}>
                                <span className="beeftv-recent-preview"><ProjectPreview project={{ id: project.id, nodes: project.previewNodes }} emptyVariant="libtv" /></span>
                                <span className="beeftv-recent-copy"><strong>{project.title || "未命名"}</strong><small>{formatDate(project.updatedAt)}</small></span>
                                <ArrowRight className="beeftv-recent-arrow" />
                            </Link>;
                        }) : <button type="button" className="beeftv-recent-card is-empty" onClick={() => navigate("/canvas?mode=new")}><span className="beeftv-recent-preview"><Plus /></span><span className="beeftv-recent-copy"><strong>创建第一个画布</strong><small>让灵感有一个开始的地方</small></span></button>}
                    </div>
                )}
            </section>

            <section className="beeftv-home-section beeftv-workflow-section" aria-label="工作流">
                <header className="beeftv-section-heading">
                    <h2>工作流</h2>
                    <span>选择后台配置的流程，直接开始创作</span>
                </header>
                {workflowsError ? (
                    <button className="beeftv-home-error" type="button" onClick={onRetryWorkflows}><RefreshCw />工作流读取失败，点击重试</button>
                ) : (
                    <div className="beeftv-workflow-grid">
                        {workflowsLoading ? Array.from({ length: 3 }, (_, index) => <div className="beeftv-workflow-card is-loading" key={index} />) : workflows.length ? workflows.map((workflow) => (
                            <button type="button" className="beeftv-workflow-card" key={workflow.id} onClick={() => onSelectWorkflow(workflow)}>
                                <span className="beeftv-workflow-icon"><Workflow /></span>
                                <span className="beeftv-workflow-copy">
                                    <strong>{workflow.name}</strong>
                                    <small>{workflow.description || "按后台配置自动创建流程画布"}</small>
                                </span>
                                <span className="beeftv-workflow-meta">
                                    <LayoutTemplate />
                                    {workflowStepCount(workflow)} 步
                                </span>
                                <ArrowRight className="beeftv-workflow-arrow" />
                            </button>
                        )) : <div className="beeftv-workflow-empty">暂无已启用的工作流，请先在管理后台配置。</div>}
                    </div>
                )}
            </section>
        </main>
    );
}

function workflowStepCount(workflow: PublicCreationTemplate) {
    try {
        const payload = parseCreationTemplatePayload(workflow.payloadJson);
        return payload.stepCount || payload.nodes.length;
    } catch {
        return 0;
    }
}
