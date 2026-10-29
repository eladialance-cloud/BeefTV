import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AppModal } from "@/components/ui/product/app-modal";
import { ManjuArtifactEditor } from "@/components/manju/manju-artifact-editor";
import { WalletChip } from "@/components/billing/wallet-chip";
import { Alert, Button, Checkbox, Form, Input, InputNumber, Select, Spin, Upload } from "antd";
import {
    ArrowLeft,
    ArrowRight,
    BookOpenText,
    Boxes,
    ChevronRight,
    Check,
    Clapperboard,
    Clock3,
    Download,
    FileCheck2,
    FileText,
    Film,
    FolderOpen,
    Image as ImageIcon,
    ImagePlus,
    Layers3,
    LoaderCircle,
    LockKeyhole,
    Play,
    Plus,
    RefreshCw,
    ShieldCheck,
    Sparkles,
    Trash2,
    UserPlus,
    Users,
    Video,
    type LucideIcon,
} from "lucide-react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";

import { getModelCatalog } from "@/services/api/model-catalog";
import { buildManjuScriptRunInput, canCreateManjuScriptRun } from "@/lib/canvas/manju-script-routing";
import {
    createManjuProject,
    deleteManjuProject,
    confirmManjuScriptStage,
    createManjuScriptRun,
    decideManjuRoundtable,
    getManjuScriptRun,
    getManjuProject,
    getManjuStyleCatalog,
    getManjuRuleLibrary,
    inviteManjuMember,
    listManjuScriptRuns,
    listManjuProjects,
    manualCorrectManjuScriptArtifact,
    removeManjuMember,
    registerManjuScriptSource,
    regenerateManjuScriptArtifact,
    runManjuScriptStage,
    saveManjuLock,
    saveManjuModels,
    type ManjuScriptArtifact,
    type ManjuScriptBranch,
    type ManjuScriptInputKind,
    type ManjuScriptMode,
    type ManjuScriptRun,
    type ManjuScriptRunDetail,
    type ManjuScriptSourceInput,
    type ManjuScriptStage,
    type ManjuScriptStageStatus,
    type ManjuLockInput,
    type ManjuLockKey,
    type ManjuMemberRole,
    type ManjuProjectDetail,
    type ManjuProjectModelBinding,
    type ManjuProjectSummary,
    type ManjuRuleLibraryStatus,
    type ManjuRoundtableDecision,
    type ManjuStyleBaseID,
    type ManjuStyleCard,
    type ManjuTrendResolution,
} from "@/services/api/manju";
import {
    confirmManjuAssetManifest,
    confirmManjuAssetRun,
    createManjuAssetRun,
    getManjuAssetRun,
    listManjuAssetRuns,
    manualCorrectManjuAssetArtifact,
    regenerateManjuAssetArtifact,
    registerManjuAssetReference,
    runManjuAssetStage,
    confirmManjuProductionStage,
    createManjuProductionRun,
    generateManjuProductionPrevis,
    getManjuProductionRun,
    getManjuProductionRunDeliverables,
    listManjuProductionRuns,
    manualCorrectManjuProductionArtifact,
    previewManjuProductionStoryboardCompile,
    regenerateManjuProductionArtifact,
    runManjuProductionStage,
    getManjuCanvasPackage,
    listManjuCanvasImports,
    saveManjuCanvasImport,
    getManjuCanvasImportReferences,
    type ManjuAssetArtifact,
    type ManjuAssetMode,
    type ManjuAssetRun,
    type ManjuAssetRunDetail,
    type ManjuAssetStage,
    type ManjuAssetStageStatus,
    type ManjuCanvasPackage,
    type ManjuCanvasPackageStage,
    type ManjuCanvasImportReference,
    type ManjuCanvasTask,
    type ManjuProductionArtifact,
    type ManjuProductionImage,
    type ManjuProductionRun,
    type ManjuProductionRunDetail,
    type ManjuProductionStage,
    type ManjuProductionStageStatus,
    type ManjuCompilePreview,
    type ManjuUserDeliverable,
} from "@/services/api/manju";
import { useUserStore } from "@/stores/use-user-store";
import { createManjuCanvasPackage, manjuAssetNameFromCode, manjuCanvasViewportForNodes, mergeManjuCanvasPackage, nextManjuCanvasCenter, normalizeManjuCanvasLayout, type ManjuCanvasReference } from "@/lib/canvas/canvas-manju";
import { isManjuAssetTable, manjuReferenceCodeKey } from "@/lib/canvas/canvas-manju-asset-table";
import { manjuP5CanvasPath, primaryManjuProjectCanvas } from "@/lib/canvas/manju-canvas-entry";
import { buildManjuUnitPackage } from "@/lib/canvas/manju-unit-package";
import { analyzeManjuStyleReferenceFile } from "@/lib/canvas/manju-style-reference";
import { createCanvasProjectWithRemoteSync, loadCanvasProjectForEditing, syncLocalCanvasSnapshot } from "@/services/local-workspace-sync";
import { resourceFileUrl, resourceStorageKey, uploadResourceFile } from "@/services/api/resources";
import { SourceLibraryStepsPanel } from "./source-library-steps-panel";
import "./manju-studio.css";

type StudioTab = "overview" | "p0" | "stages" | "members" | "models";

type StageContract = {
    id: string;
    title: string;
    eyebrow: string;
    icon: LucideIcon;
    input: string;
    output: string;
    consumer: string;
    routes?: Array<{ name: string; input: string; output: string }>;
};

const STUDIO_TABS: Array<{ id: StudioTab; label: string; icon: LucideIcon }> = [
    { id: "overview", label: "项目总览", icon: Clapperboard },
    { id: "p0", label: "P0 立项锁", icon: LockKeyhole },
    { id: "stages", label: "P1-P5 流程", icon: Layers3 },
    { id: "members", label: "成员", icon: Users },
    { id: "models", label: "模型", icon: Boxes },
];

const STAGE_CONTRACTS: StageContract[] = [
    {
        id: "p1",
        title: "P1 剧本",
        eyebrow: "来源登记 -> 六阶段剧本包",
        icon: BookOpenText,
        input: "P0 锁、来源作品与授权、本次章节/段落、创作锁定；必要时先有对标情报卡。",
        output: "可追溯分集剧本、剧本清单与连续性账本；润色路径另出 POLISHED_SCRIPT.md、POLISH_CONFIRM_CARD.md、POLISH_FINDINGS.md。",
        consumer: "P2 数字资产包读取锁定剧本、角色/IP 清单、来源改编地图与连续性账本。",
        routes: [
            { name: "小说改编", input: "来源作品、授权范围、本次改编章节/段落、原文", output: "来源账本、改编蓝图、分集剧本与剧本清单" },
            { name: "原创故事", input: "故事种子、受众、赛道、篇幅与创作边界", output: "IP 设定、故事圣经、分集计划与原创剧本" },
            { name: "对标改编", input: "对标作品信息、情报卡、可借鉴维度与禁止复制项", output: "对标情报卡、差异化改编蓝图与剧本" },
            { name: "外来剧本润色", input: "完整外部剧本、原场次结构、润色边界", output: "润色稿、回签卡、发现清单；不得擅改台词/事件/人物" },
        ],
    },
    {
        id: "p2",
        title: "P2 数字资产",
        eyebrow: "实体提取 -> 资产包",
        icon: Boxes,
        input: "已锁定 P1 剧本、角色/IP 清单、素材形态 M0/M1/M2 与风格合同。",
        output: "角色、场景、道具、声音等资产包；M0 外观档案、M1 提示词与素材接线单、M2 资产图册。",
        consumer: "P3 调度与分镜读取资产编号、外观档案、空间资产和参考图。",
    },
    {
        id: "p3",
        title: "P3 导演工作台",
        eyebrow: "调度 -> 分镜 -> 故事板",
        icon: Film,
        input: "锁定资产包、剧本、表演档案、空间坐标、镜头策略与段缝方式。",
        output: "角色表演档案、逐场节拍表、导演稿、逐镜分镜、故事板图与 CLEAN_GRID、条件性剧情分镜格。",
        consumer: "P4 读取逐镜分镜、故事板动作相位、声线锁和连续性要求生成投喂稿。",
    },
    {
        id: "p4",
        title: "P4 投喂封装",
        eyebrow: "P3 交接 -> 模型投喂稿",
        icon: Video,
        input: "P3 分镜与故事板、资产接线单、模型适配器、画幅、段缝和六道闸门。",
        output: "Seedance 2.5/2.0 或 MiniMax H3 可投喂稿；多段任务附段缝画面锁与末镜定格要求。",
        consumer: "视频模型执行生成；P5 读取投喂稿、生成结果和段缝边界做质检。",
    },
    {
        id: "p5",
        title: "P5 质检与交付",
        eyebrow: "机检 -> 人审 -> 交付",
        icon: FileCheck2,
        input: "P4 投喂稿、P4 画布真实视频、P0-P2 锁定产物、连续性与段缝台账。",
        output: "质检结论、问题清单、改动复验报告与最终交付清单。",
        consumer: "主理人/制片验收；未通过项回到产生它的阶段修复，不能在 P5 偷改上游事实。",
    },
];

type PipelineStageId = "p1" | "p2" | "p3" | "p4" | "p5";

const PIPELINE_STAGES: Array<{ id: PipelineStageId; label: string; detail: string; icon: LucideIcon }> = [
    { id: "p1", label: "P1 剧本", detail: "来源、剧本与审核", icon: BookOpenText },
    { id: "p2", label: "P2 数字资产", detail: "实体、清单与资产包", icon: Boxes },
    { id: "p3", label: "P3 分镜", detail: "表演、调度与故事板", icon: Film },
    { id: "p4", label: "P4 投喂", detail: "提示词与分段视频", icon: Video },
    { id: "p5", label: "P5 质检交付", detail: "机检、看片与裁决", icon: FileCheck2 },
];

function isPipelineStage(value: string | null): value is PipelineStageId {
    return PIPELINE_STAGES.some((stage) => stage.id === value);
}

const TRACK_OPTIONS = [
    { value: "domestic", label: "国内剧" },
    { value: "overseas", label: "海外剧" },
    { value: "explainer", label: "解说叙事" },
    { value: "other", label: "其他" },
];

const SOURCE_OPTIONS = [
    { value: "original", label: "原创故事" },
    { value: "adaptation", label: "小说/外部素材改编" },
    { value: "benchmark", label: "对标改编" },
    { value: "trend", label: "趋势热点改编" },
];

const INPUT_KIND_OPTIONS: Array<{ value: ManjuScriptInputKind; label: string }> = [
    { value: "novel", label: "小说 / 网文" },
    { value: "outline", label: "大纲 / 梗概" },
    { value: "finished_script", label: "成型外部剧本" },
];

const TREND_RESOLUTION_OPTIONS: Array<{ value: Exclude<ManjuTrendResolution, "">; label: string }> = [
    { value: "original", label: "选定方向后按原创执行" },
    { value: "benchmark", label: "选定方向后按对标执行" },
];

const MODEL_OPTIONS = [
    { value: "seedance-2.5", label: "Seedance 2.5 · 单段 30 秒" },
    { value: "seedance-2.0", label: "Seedance 2.0 · 单段 15 秒" },
    { value: "minimax-h3", label: "MiniMax H3 · 单段 15 秒" },
];

const ASPECT_OPTIONS = [
    { value: "16:9", label: "16:9 横屏" },
    { value: "9:16", label: "9:16 竖屏" },
    { value: "21:9", label: "21:9 宽银幕" },
];

const DELIVERY_OPTIONS = [
    { value: "board", label: "故事板交付" },
    { value: "markdown", label: "Markdown 交付" },
    { value: "both", label: "故事板 + Markdown" },
];

const STYLE_OPTIONS = [
    { value: "live-action", label: "实拍质感" },
    { value: "2d-animation", label: "2D 动画" },
    { value: "3d-cg", label: "3D CG" },
    { value: "ink-2d", label: "国风 2D" },
];

const SEAM_OPTIONS = [
    { value: "tail-frame-freeze", label: "尾帧定格承接" },
    { value: "hard-cut", label: "硬切" },
    { value: "insert-shot", label: "插入镜头" },
    { value: "creative-transition", label: "技巧转场" },
];

const ROLE_OPTIONS: Array<{ value: Exclude<ManjuMemberRole, "owner">; label: string }> = [
    { value: "producer", label: "制片 · 可保存 P0" },
    { value: "writer", label: "编剧" },
    { value: "director", label: "导演" },
    { value: "reviewer", label: "审片" },
    { value: "viewer", label: "只读" },
];

const ROLE_LABELS: Record<ManjuMemberRole, string> = {
    owner: "主理人",
    producer: "制片",
    writer: "编剧",
    director: "导演",
    reviewer: "审片",
    viewer: "只读",
};

const MODEL_LIMIT_SECONDS: Record<Exclude<ManjuLockInput["videoModel"], "">, number> = {
    "seedance-2.5": 30,
    "seedance-2.0": 15,
    "minimax-h3": 15,
};

const REQUIRED_P0_LOCK_KEYS: ManjuLockKey[] = ["track", "source", "videoModel", "aspectRatio", "deliveryMode", "styleCard"];

function isStudioTab(value: string | null): value is StudioTab {
    return STUDIO_TABS.some((tab) => tab.id === value);
}

function formatDate(value?: string) {
    if (!value) return "尚未更新";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "尚未更新";
    return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }).format(date);
}

function sourceLabel(value: string) {
    return SOURCE_OPTIONS.find((item) => item.value === value)?.label || value || "未登记";
}

function isCurrentLockRevision(lockRevision: number, currentRevision?: number) {
    return currentRevision !== undefined && lockRevision === currentRevision;
}

function roleLabel(value: ManjuMemberRole) {
    return ROLE_LABELS[value] || value;
}

function canEditP0(role: ManjuMemberRole) {
    return role === "owner" || role === "producer";
}

function canEditModels(role: ManjuMemberRole) {
    return role === "owner" || role === "producer";
}

const STYLE_BASE_LABELS: Record<ManjuStyleBaseID, string> = {
    "live-action": "真人实拍质感",
    "2d-animation": "2D 手绘动画",
    "3d-cg": "3D CG 动画",
    "ink-2d": "水墨二维",
};

function styleRating(value: number) {
    return value > 0 ? "★".repeat(value) : "—";
}

function styleRecommendationScore(card: ManjuStyleCard, track: ManjuLockInput["track"]) {
    const capability = card.fightRating + card.effectRating;
    const gradePenalty = card.grade === "C" ? 3 : card.grade === "B" ? 1 : 0;
    const genre = card.nativeGenre;
    let trackScore = 0;
    if (track === "domestic" && /玄幻|仙侠|武侠|神话|古装|都市/.test(genre)) trackScore += 3;
    if (track === "overseas" && /欧美|科幻|犯罪|惊悚|史诗/.test(genre)) trackScore += 3;
    if (track === "explainer" && /水墨|手绘|生活|历史|油画|插画/.test(genre)) trackScore += 3;
    if (track === "other" && card.grade === "A") trackScore += 1;
    return trackScore + capability - gradePenalty;
}

type ManjuModelCapability = "text" | "image" | "video";
type ManjuModelDraft = Pick<ManjuProjectModelBinding, "text" | "image" | "video">;
type ManjuModelOption = {
    value: string;
    label: string;
    channelId: string;
    channelName: string;
    modelKey: string;
    modelName: string;
};

function modelSelectionValue(channelId: string, modelKey: string) {
    return `${encodeURIComponent(channelId)}::${encodeURIComponent(modelKey)}`;
}

function emptyModelDraft(): ManjuModelDraft {
    return {
        text: { channelId: "", modelKey: "" },
        image: { channelId: "", modelKey: "" },
        video: { channelId: "", modelKey: "" },
    };
}

function modelDraftFromDetail(detail: ManjuProjectDetail): ManjuModelDraft {
    return detail.models ? { text: detail.models.text, image: detail.models.image, video: detail.models.video } : emptyModelDraft();
}

function styleForbiddenText(items?: string[]) {
    return (items || []).join("\n");
}

function parseStyleForbiddenItems(value: string) {
    return value
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter(Boolean);
}

function defaultLock(): ManjuLockInput {
    return {
        track: "",
        source: "",
        videoModel: "",
        aspectRatio: "",
        deliveryMode: "",
        seamMode: "",
        targetDurationSeconds: 0,
        styleBase: "",
        styleCardId: "",
        styleCustomDescription: "",
        styleVisualPrompt: "",
        styleMotionParameters: "",
        styleForbiddenItems: [],
        styleReferenceResourceId: "",
        styleReferenceAnalysis: "",
        styleReferencePalette: "",
        assetMode: "M1",
        confirmedLocks: [],
    };
}

function InlineNotice({ tone, children }: { tone: "info" | "error" | "success"; children: React.ReactNode }) {
    return (
        <div className={`manju-notice is-${tone}`} role={tone === "error" ? "alert" : "status"}>
            {children}
        </div>
    );
}

function StudioBrand({ compact = false }: { compact?: boolean }) {
    return (
        <div className={`manju-brand${compact ? " is-compact" : ""}`}>
            <img src="/visideep-icon-32.png" alt="VisiDeep AI" />
            <div>
                <strong>VisiDeep AI</strong>
                <span>漫剧工作室</span>
            </div>
        </div>
    );
}

export default function ManjuStudioPage() {
    const params = useParams<{ projectId?: string }>();
    const projectId = params.projectId?.trim() || "";
    return projectId ? <ManjuProjectWorkspace projectId={projectId} /> : <ManjuProjectLibrary />;
}

function ManjuProjectLibrary() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [createOpen, setCreateOpen] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState<ManjuProjectSummary | null>(null);
    const [createForm] = Form.useForm<{ name: string; aspectRatio: ManjuLockInput["aspectRatio"]; sourceType: ManjuLockInput["source"] }>();
    const projectsQuery = useQuery({
        queryKey: ["manju", "projects"],
        queryFn: listManjuProjects,
    });
    const deleteMutation = useMutation({
        mutationFn: (projectId: string) => deleteManjuProject(projectId),
        onSuccess: () => {
            setDeleteTarget(null);
            void queryClient.invalidateQueries({ queryKey: ["manju", "projects"] });
        },
    });
    const createMutation = useMutation({
        mutationFn: createManjuProject,
        onSuccess: ({ project }) => {
            void queryClient.invalidateQueries({ queryKey: ["manju", "projects"] });
            setCreateOpen(false);
            navigate(`/manju/${project.id}`);
        },
    });

    const openCreate = () => {
        setCreateOpen(true);
    };

    return (
        <main className="manju-studio-page" aria-label="VisiDeep AI 漫剧工作室">
            <header className="manju-page-header">
                <StudioBrand />
                <div className="manju-header-actions">
                    <WalletChip compact />
                    <Button type="primary" icon={<Plus />} onClick={openCreate}>
                        新建漫剧项目
                    </Button>
                </div>
            </header>

            <div className="manju-page-body">
                <section className="manju-library-intro" aria-labelledby="manju-library-title">
                    <div>
                        <span className="manju-kicker">
                            <Sparkles />
                            VisiDeep AI · V7.4.26
                        </span>
                        <h1 id="manju-library-title">漫剧项目</h1>
                        <p>从立项锁开始。主理人和制片可以落锁，其他成员按角色协作；P1-P5 使用真实服务端状态。P2-P4 的图片与视频任务会导入画布生成，P5 由主理人结合画布真实资源看片验收。</p>
                    </div>
                    <div className="manju-library-principles" aria-label="工作室规则">
                        <span>
                            <ShieldCheck />
                            P0 服务端锁
                        </span>
                        <span>
                            <Users />
                            项目成员隔离
                        </span>
                        <span>
                            <LockKeyhole />
                            不覆盖历史回执
                        </span>
                    </div>
                </section>

                {projectsQuery.isError ? (
                    <div className="manju-state-panel">
                        <Alert type="error" showIcon message="漫剧项目读取失败" description={projectsQuery.error instanceof Error ? projectsQuery.error.message : "请检查登录状态和后端服务。"} />
                        <Button icon={<RefreshCw />} onClick={() => void projectsQuery.refetch()}>
                            重新读取
                        </Button>
                    </div>
                ) : projectsQuery.isLoading ? (
                    <div className="manju-loading-panel">
                        <Spin />
                        <span>正在读取漫剧项目</span>
                    </div>
                ) : projectsQuery.data?.items.length ? (
                    <section className="manju-project-grid" aria-label="漫剧项目列表">
                        {projectsQuery.data.items.map((project) => (
                            <ManjuProjectCard
                                key={project.id}
                                project={project}
                                onDelete={() => {
                                    deleteMutation.reset();
                                    setDeleteTarget(project);
                                }}
                            />
                        ))}
                    </section>
                ) : (
                    <section className="manju-empty-state">
                        <FolderOpen />
                        <h2>还没有漫剧项目</h2>
                        <p>新建项目后先完成 P0 立项锁。未锁定的项目不会进入 P1-P5。</p>
                        <Button type="primary" icon={<Plus />} onClick={openCreate}>
                            创建第一个项目
                        </Button>
                    </section>
                )}
            </div>

            <AppModal
                open={createOpen}
                title="新建漫剧项目"
                okText="创建并填写 P0"
                cancelText="取消"
                confirmLoading={createMutation.isPending}
                onCancel={() => {
                    if (createMutation.isPending) return;
                    setCreateOpen(false);
                }}
                onOk={() => createForm.submit()}
            >
                <Form form={createForm} layout="vertical" initialValues={{ aspectRatio: "9:16", sourceType: "adaptation" }} onFinish={(values) => createMutation.mutate(values)}>
                    <Form.Item name="name" label="项目名称" rules={[{ required: true, whitespace: true, message: "请填写项目名称" }]}>
                        <Input autoFocus maxLength={120} placeholder="例如：雾港回声" />
                    </Form.Item>
                    <div className="manju-form-grid is-two">
                        <Form.Item name="sourceType" label="素材来源">
                            <Select options={SOURCE_OPTIONS} />
                        </Form.Item>
                        <Form.Item name="aspectRatio" label="画幅">
                            <Select options={ASPECT_OPTIONS} />
                        </Form.Item>
                    </div>
                    {createMutation.isError ? <InlineNotice tone="error">{createMutation.error instanceof Error ? createMutation.error.message : "项目创建失败"}</InlineNotice> : null}
                </Form>
            </AppModal>

            <AppModal
                open={Boolean(deleteTarget)}
                title="删除漫剧项目"
                okText="永久删除"
                cancelText="取消"
                okButtonProps={{ danger: true }}
                confirmLoading={deleteMutation.isPending}
                onCancel={() => {
                    if (deleteMutation.isPending) return;
                    deleteMutation.reset();
                    setDeleteTarget(null);
                }}
                onOk={() => {
                    if (deleteTarget) deleteMutation.mutate(deleteTarget.id);
                }}
            >
                <div className="manju-delete-confirm">
                    <p>
                        确认删除「<strong>{deleteTarget?.name}</strong>」？
                    </p>
                    <ul>
                        <li>P0-P5 项目数据会删除且不可恢复。</li>
                        <li>成员、立项锁和模型绑定会一并移除。</li>
                        <li>已完成任务、结果记录和独立资源文件会保留。</li>
                    </ul>
                    {deleteMutation.isError ? <InlineNotice tone="error">{deleteMutation.error instanceof Error ? deleteMutation.error.message : "漫剧项目删除失败"}</InlineNotice> : null}
                </div>
            </AppModal>
        </main>
    );
}

function ManjuProjectCard({ project, onDelete }: { project: ManjuProjectSummary; onDelete: () => void }) {
    const canDelete = project.accessRole === "owner";
    return (
        <article className="manju-project-card">
            <div className="manju-project-card-actions">
                <span className="manju-project-icon">
                    <Clapperboard />
                </span>
                <div className="manju-project-card-badges">
                    <span className="manju-project-role">{roleLabel(project.accessRole)}</span>
                    {canDelete ? (
                        <Button type="text" danger size="small" className="manju-project-delete" icon={<Trash2 />} aria-label={`删除漫剧项目 ${project.name}`} onClick={onDelete}>
                            删除
                        </Button>
                    ) : null}
                </div>
            </div>
            <Link to={`/manju/${project.id}`} className="manju-project-card-link">
                <strong>{project.name}</strong>
                <span className="manju-project-meta">
                    <span>{sourceLabel(project.sourceType)}</span>
                    <span>{project.aspectRatio || "未设画幅"}</span>
                    <span>{formatDate(project.updatedAt)}</span>
                </span>
                <span className="manju-project-card-foot">
                    <span>项目 revision {project.revision}</span>
                    <ArrowRight />
                </span>
            </Link>
        </article>
    );
}

function ManjuProjectWorkspace({ projectId }: { projectId: string }) {
    const queryClient = useQueryClient();
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const currentTab = searchParams.get("tab");
    const activeTab: StudioTab = isStudioTab(currentTab) ? currentTab : "overview";
    const currentPipelineStage = searchParams.get("stage");
    const activePipelineStage: PipelineStageId = isPipelineStage(currentPipelineStage) ? currentPipelineStage : "p1";
    const detailQuery = useQuery({
        queryKey: ["manju", "project", projectId],
        queryFn: () => getManjuProject(projectId),
        retry: false,
    });
    const setTab = (tab: StudioTab) => {
        if (tab === "overview") setSearchParams({}, { replace: true });
        else if (tab === "stages") setSearchParams({ tab, stage: activePipelineStage }, { replace: true });
        else setSearchParams({ tab }, { replace: true });
    };
    const setPipelineStage = (stage: PipelineStageId) => {
        setSearchParams({ tab: "stages", stage }, { replace: true });
        window.requestAnimationFrame(() => document.querySelector(".manju-pipeline-panel")?.scrollIntoView({ block: "start", behavior: "smooth" }));
    };
    const detail = detailQuery.data?.item;
    const refreshProject = () => {
        void queryClient.invalidateQueries({ queryKey: ["manju", "project", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["manju", "projects"] });
        void queryClient.invalidateQueries({ queryKey: ["manju", "script-runs", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["manju", "asset-runs", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["manju", "production-runs", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["manju", "rule-library", projectId] });
    };

    return (
        <main className="manju-studio-page" aria-label="VisiDeep AI 漫剧项目工作台">
            <header className="manju-page-header is-project">
                <div className="manju-project-header-main">
                    <Button aria-label="返回漫剧项目列表" type="text" icon={<ArrowLeft />} onClick={() => navigate("/manju")} />
                    <StudioBrand compact />
                    <span className="manju-header-divider" aria-hidden />
                    <div className="manju-project-heading">
                        <span className="manju-kicker">漫剧工作室</span>
                        <h1>{detail?.project.name || (detailQuery.isLoading ? "正在读取项目" : "项目不可用")}</h1>
                    </div>
                </div>
                <div className="manju-header-actions">
                    {detail ? <span className={`manju-role-badge is-${detail.accessRole}`}>{roleLabel(detail.accessRole)}</span> : null}
                    <WalletChip compact />
                </div>
            </header>

            {detailQuery.isError ? (
                <div className="manju-page-body">
                    <div className="manju-state-panel">
                        <Alert type="error" showIcon message="项目无法打开" description={detailQuery.error instanceof Error ? detailQuery.error.message : "项目不存在，或当前账号没有访问权限。"} />
                        <div className="manju-inline-actions">
                            <Button icon={<RefreshCw />} onClick={() => void detailQuery.refetch()}>
                                重试
                            </Button>
                            <Link to="/manju">
                                <Button>返回项目列表</Button>
                            </Link>
                        </div>
                    </div>
                </div>
            ) : !detail ? (
                <div className="manju-loading-panel">
                    <Spin />
                    <span>正在读取项目状态</span>
                </div>
            ) : (
                <>
                    <nav className="manju-tabbar" aria-label="项目工作区">
                        {STUDIO_TABS.map((tab) => {
                            const Icon = tab.icon;
                            return (
                                <button key={tab.id} type="button" className={activeTab === tab.id ? "is-active" : ""} onClick={() => setTab(tab.id)}>
                                    <Icon />
                                    {tab.label}
                                </button>
                            );
                        })}
                    </nav>
                    <div className="manju-page-body is-workspace">
                        {activeTab === "overview" ? <ProjectOverview detail={detail} onOpenP0={() => setTab("p0")} onOpenStages={() => setTab("stages")} /> : null}
                        {activeTab === "p0" ? <P0LockPanel detail={detail} onSaved={refreshProject} /> : null}
                        {activeTab === "stages" ? <PipelineWorkspace detail={detail} activeStage={activePipelineStage} onStageChange={setPipelineStage} onChanged={refreshProject} /> : null}
                        {activeTab === "members" ? <MemberPanel detail={detail} onChanged={refreshProject} /> : null}
                        {activeTab === "models" ? <ModelPanel detail={detail} onChanged={refreshProject} /> : null}
                    </div>
                </>
            )}
        </main>
    );
}

function ProjectOverview({ detail, onOpenP0, onOpenStages }: { detail: ManjuProjectDetail; onOpenP0: () => void; onOpenStages: () => void }) {
    const locked = Boolean(detail.lock);
    return (
        <div className="manju-overview">
            <section className="manju-overview-lead">
                <div>
                    <span className="manju-kicker">当前生产位置</span>
                    <h2>{locked ? `P0 已锁定 · revision ${detail.lock?.revision}` : "先完成 P0 立项锁"}</h2>
                    <p>
                        {locked
                            ? "P0 回执已由服务端保存。P2-P4 会把真实任务包导入画布，由画布统一生图和生视频；漫剧工作室读取已关联画布的任务资源，未全部就绪时拒绝锁定对应阶段。P5 仍须主理人逐段看片验收。"
                            : "赛道、来源、模型、画幅、交付、画风卡与目标时长必须先在服务端锁定，P1 才能开始。"}
                    </p>
                </div>
                <div className="manju-overview-actions">
                    <Button type={locked ? "default" : "primary"} icon={<LockKeyhole />} onClick={onOpenP0}>
                        {locked ? "查看 / 更新 P0" : "填写 P0"}
                    </Button>
                    <Button icon={<Layers3 />} onClick={onOpenStages}>
                        查看完整流程
                    </Button>
                </div>
            </section>

            <section className="manju-fact-strip" aria-label="项目属性">
                <div>
                    <span>素材来源</span>
                    <strong>{sourceLabel(detail.project.sourceType)}</strong>
                </div>
                <div>
                    <span>画幅</span>
                    <strong>{detail.project.aspectRatio || "未设置"}</strong>
                </div>
                <div>
                    <span>当前权限</span>
                    <strong>{roleLabel(detail.accessRole)}</strong>
                </div>
                <div>
                    <span>P0 回执</span>
                    <strong>{locked ? `revision ${detail.lock?.revision}` : "未锁定"}</strong>
                </div>
            </section>

            <section className="manju-stage-list" aria-labelledby="manju-stage-title">
                <header>
                    <div>
                        <span className="manju-kicker">P0-P5</span>
                        <h2 id="manju-stage-title">生产阶段</h2>
                    </div>
                    <span className="manju-stage-legend">
                        <Check />
                        P0-P5 有服务端状态；媒体生成不伪造
                    </span>
                </header>
                <div className="manju-stage-rail">
                    <div className={`manju-stage-row${locked ? " is-locked" : ""}`}>
                        <span className="manju-stage-number">P0</span>
                        <span className="manju-stage-icon">
                            <LockKeyhole />
                        </span>
                        <span className="manju-stage-copy">
                            <strong>立项锁</strong>
                            <small>{locked ? detail.lock?.contentHash.slice(0, 12) : "等待主理人或制片落锁"}</small>
                        </span>
                        <span className="manju-stage-status">{locked ? "已锁" : "未锁定"}</span>
                    </div>
                    {STAGE_CONTRACTS.map((stage) => {
                        const Icon = stage.icon;
                        const connected = true;
                        return (
                            <div className={`manju-stage-row${connected ? "" : " is-pending"}`} key={stage.id}>
                                <span className="manju-stage-number">{stage.id.toUpperCase()}</span>
                                <span className="manju-stage-icon">
                                    <Icon />
                                </span>
                                <span className="manju-stage-copy">
                                    <strong>{stage.title.replace(/^P\d\s*/, "")}</strong>
                                    <small>{stage.eyebrow}</small>
                                </span>
                                <span className="manju-stage-status">{connected ? "已接入" : "未执行"}</span>
                            </div>
                        );
                    })}
                </div>
            </section>

            <InlineNotice tone="info">
                P0-P5 已接入 VisiDeep AI 的服务端任务和工件链。P2-P4 不再在漫剧页直接调用图片或视频模型，而是把生产包导入画布；只有画布任务取得真实资源后，阶段才允许锁定。P5 仍须主理人看片，页面不会伪造 PASS、成片或下载。
            </InlineNotice>
        </div>
    );
}

export function P0LockPanel({ detail, onSaved }: { detail: ManjuProjectDetail; onSaved: () => void }) {
    const initial = detail.lock?.snapshot;
    const [form, setForm] = useState<ManjuLockInput>(() =>
        initial
            ? {
                  track: initial.track,
                  source: initial.source,
                  videoModel: initial.videoModel,
                  aspectRatio: initial.aspectRatio,
                  deliveryMode: initial.deliveryMode,
                  seamMode: initial.seamMode || "",
                  targetDurationSeconds: initial.targetDurationSeconds,
                  styleBase: initial.styleBase,
                  styleCardId: initial.styleCardId,
                  styleCustomDescription: "",
                  styleVisualPrompt: initial.styleVisualPrompt || initial.styleCustomDescription || initial.stylePromptParameters || "",
                  styleMotionParameters: initial.styleMotionParameters || "",
                  styleForbiddenItems: initial.styleForbiddenItems || [],
                  styleReferenceResourceId: initial.styleReferenceResourceId || "",
                  styleReferenceAnalysis: initial.styleReferenceAnalysis || "",
                  styleReferencePalette: initial.styleReferencePalette || "",
                  assetMode: initial.assetMode || "M1",
                  confirmedLocks: initial.confirmedLocks || [],
              }
            : defaultLock(),
    );
    const [confirmedLocks, setConfirmedLocks] = useState<Set<ManjuLockKey>>(() => new Set(initial?.confirmedLocks || []));
    const [styleFamilyId, setStyleFamilyId] = useState("");
    const [showAllStyles, setShowAllStyles] = useState(false);
    const [styleReferenceUploading, setStyleReferenceUploading] = useState(false);
    const [styleReferencePreview, setStyleReferencePreview] = useState(() => (initial?.styleReferenceResourceId ? resourceFileUrl(initial.styleReferenceResourceId) : ""));
    const styleCatalogQuery = useQuery({
        queryKey: ["manju", "style-catalog", "V7.4.26"],
        queryFn: getManjuStyleCatalog,
        staleTime: 5 * 60_000,
    });
    const [notice, setNotice] = useState<{ tone: "error" | "success"; text: string } | null>(null);
    const canEdit = canEditP0(detail.accessRole);
    const segmentLimit = form.videoModel ? MODEL_LIMIT_SECONDS[form.videoModel] : 0;
    const segmentCount = segmentLimit > 0 ? Math.max(1, Math.ceil((form.targetDurationSeconds || 0) / segmentLimit)) : 1;
    const requiredLockKeys = useMemo<ManjuLockKey[]>(
        () => (segmentCount > 1 ? [...REQUIRED_P0_LOCK_KEYS, "seamMode"] : REQUIRED_P0_LOCK_KEYS),
        [segmentCount],
    );
    const readyToLockP0 = requiredLockKeys.every((key) => confirmedLocks.has(key));
    const compatibleFamilies = useMemo(() => {
        const catalog = styleCatalogQuery.data;
        if (!catalog) return [];
        const familyIds = new Set(catalog.items.filter((card) => card.compatibleStyleBases.includes(form.styleBase)).map((card) => card.family));
        return catalog.families.filter((family) => familyIds.has(family.id));
    }, [form.styleBase, styleCatalogQuery.data]);
    const selectedCard = useMemo(() => styleCatalogQuery.data?.items.find((card) => card.id === form.styleCardId), [form.styleCardId, styleCatalogQuery.data]);
    const candidateCards = useMemo(() => {
        const catalog = styleCatalogQuery.data;
        if (!catalog || !styleFamilyId) return [];
        return catalog.items
            .filter((card) => card.family === styleFamilyId && card.compatibleStyleBases.includes(form.styleBase))
            .sort((left, right) => styleRecommendationScore(right, form.track) - styleRecommendationScore(left, form.track) || left.number - right.number)
            .slice(0, 4);
    }, [form.styleBase, form.track, styleCatalogQuery.data, styleFamilyId]);
    const selectedFamily = useMemo(() => styleCatalogQuery.data?.families.find((family) => family.id === styleFamilyId), [styleCatalogQuery.data, styleFamilyId]);
    useEffect(() => {
        if (!compatibleFamilies.length) {
            setStyleFamilyId("");
            return;
        }
        if (selectedCard && compatibleFamilies.some((family) => family.id === selectedCard.family)) {
            setStyleFamilyId(selectedCard.family);
            return;
        }
        if (!compatibleFamilies.some((family) => family.id === styleFamilyId)) {
            setStyleFamilyId(compatibleFamilies[0].id);
        }
    }, [compatibleFamilies, selectedCard, styleFamilyId]);
    const mutation = useMutation({
        mutationFn: (input: ManjuLockInput) => saveManjuLock(detail.project.id, input),
        onSuccess: ({ item }) => {
            setForm({
                track: item.snapshot.track,
                source: item.snapshot.source,
                videoModel: item.snapshot.videoModel,
                aspectRatio: item.snapshot.aspectRatio,
                deliveryMode: item.snapshot.deliveryMode,
                seamMode: item.snapshot.seamMode || "",
                targetDurationSeconds: item.snapshot.targetDurationSeconds,
                styleBase: item.snapshot.styleBase,
                styleCardId: item.snapshot.styleCardId,
                styleCustomDescription: "",
                styleVisualPrompt: item.snapshot.styleVisualPrompt || item.snapshot.stylePromptParameters || "",
                styleMotionParameters: item.snapshot.styleMotionParameters || "",
                styleForbiddenItems: item.snapshot.styleForbiddenItems || [],
                styleReferenceResourceId: item.snapshot.styleReferenceResourceId || "",
                styleReferenceAnalysis: item.snapshot.styleReferenceAnalysis || "",
                styleReferencePalette: item.snapshot.styleReferencePalette || "",
                assetMode: item.snapshot.assetMode,
                confirmedLocks: item.snapshot.confirmedLocks || [],
            });
            setConfirmedLocks(new Set(item.snapshot.confirmedLocks || []));
            setNotice({
                tone: "success",
                text: `P0 revision ${item.revision} 已由服务端锁定。旧 revision 的 P1-P5 运行链已隔离，后续阶段只读取当前 revision。`,
            });
            onSaved();
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "P0 保存失败" }),
    });

    const setField = <Key extends keyof ManjuLockInput>(key: Key, value: ManjuLockInput[Key]) => {
        setNotice(null);
        setForm((current) => ({ ...current, [key]: value }));
    };

    const confirmLock = (key: ManjuLockKey) => {
        setNotice(null);
        setConfirmedLocks((current) => {
            if (current.has(key)) return current;
            const next = new Set(current);
            next.add(key);
            return next;
        });
    };

    const setConfirmedField = <Key extends keyof ManjuLockInput>(key: Key, value: ManjuLockInput[Key], lockKey: ManjuLockKey) => {
        setField(key, value);
        confirmLock(lockKey);
    };

    const clearStyleCardSelection = () => {
        setConfirmedLocks((current) => {
            if (!current.has("styleCard")) return current;
            const next = new Set(current);
            next.delete("styleCard");
            return next;
        });
        setForm((current) => ({
            ...current,
            styleCardId: "",
            styleCustomDescription: "",
            styleVisualPrompt: "",
            styleMotionParameters: "",
            styleForbiddenItems: [],
        }));
    };

    const changeStyleBase = (styleBase: ManjuStyleBaseID) => {
        setNotice(null);
        setShowAllStyles(false);
        setStyleFamilyId("");
        clearStyleCardSelection();
        setForm((current) => ({
            ...current,
            styleBase,
        }));
    };

    const selectStyleCard = (card: ManjuStyleCard) => {
        setNotice(null);
        setStyleFamilyId(card.family);
        confirmLock("styleCard");
        setForm((current) => ({
            ...current,
            styleCardId: card.id,
            styleCustomDescription: "",
            styleVisualPrompt: "",
            styleMotionParameters: "",
            styleForbiddenItems: [],
        }));
    };

    const useRecommendedCard = () => {
        const recommended = candidateCards[0];
        if (!recommended) {
            setNotice({ tone: "error", text: "当前技法家族没有与影像基底兼容的候选卡，请换一个家族或改用自定义画风。" });
            return;
        }
        selectStyleCard(recommended);
    };

    const chooseCustomStyle = () => {
        setNotice(null);
        confirmLock("styleCard");
        setForm((current) => ({
            ...current,
            styleCardId: "custom",
            styleVisualPrompt: current.styleVisualPrompt || current.styleCustomDescription || "",
        }));
    };

    const uploadStyleReference = async (file: File) => {
        if (!file.type.startsWith("image/")) {
            setNotice({ tone: "error", text: "风格参考图只支持 JPG、PNG 或 WebP 图片。" });
            return Upload.LIST_IGNORE;
        }
        if (file.size >= 50 * 1024 * 1024) {
            setNotice({ tone: "error", text: "风格参考图必须小于 50MB。" });
            return Upload.LIST_IGNORE;
        }
        setStyleReferenceUploading(true);
        setNotice(null);
        try {
            const [resource, analysis] = await Promise.all([uploadResourceFile(file, "image", { fileName: file.name }), analyzeManjuStyleReferenceFile(file)]);
            setStyleReferencePreview(resourceFileUrl(resource.id));
            setForm((current) => ({
                ...current,
                styleReferenceResourceId: resource.id,
                styleReferenceAnalysis: analysis.prompt,
                styleReferencePalette: analysis.palette.join(" / "),
            }));
            setNotice({ tone: "success", text: `风格参考图分析完成：${analysis.summary}` });
        } catch (error) {
            setNotice({ tone: "error", text: error instanceof Error ? error.message : "风格参考图上传或分析失败" });
        } finally {
            setStyleReferenceUploading(false);
        }
        return Upload.LIST_IGNORE;
    };

    const removeStyleReference = () => {
        setStyleReferencePreview("");
        setForm((current) => ({
            ...current,
            styleReferenceResourceId: "",
            styleReferenceAnalysis: "",
            styleReferencePalette: "",
        }));
    };

    const submit = () => {
        if (!canEdit) {
            setNotice({ tone: "error", text: "只有主理人或制片可以保存 P0。" });
            return;
        }
        if (!readyToLockP0) {
            setNotice({ tone: "error", text: "请逐项明确确认 P0 硬锁后再锁定。" });
            return;
        }
        if (!form.styleCardId.trim()) {
            setNotice({ tone: "error", text: "请选择一张风格候选卡，或明确选择自定义画风。" });
            return;
        }
        if (form.styleCardId === "custom" && !(form.styleVisualPrompt || "").trim()) {
            setNotice({ tone: "error", text: "选择自定义画风后，请填写正向画风。" });
            return;
        }
        if (segmentCount > 1 && !form.seamMode) {
            setNotice({ tone: "error", text: `目标时长需要 ${segmentCount} 段，请选择 D 段缝衔接方式。` });
            return;
        }
        mutation.mutate({
            ...form,
            styleCardId: form.styleCardId.trim(),
            seamMode: segmentCount > 1 ? form.seamMode : "",
            confirmedLocks: requiredLockKeys.filter((key) => confirmedLocks.has(key)),
        });
    };

    return (
        <div className="manju-p0-layout">
            <section className="manju-panel">
                <header className="manju-panel-head">
                    <div>
                        <span className="manju-kicker">P0 · 不可变决策</span>
                        <h2>立项锁</h2>
                        <p>每次保存都会创建新 revision，旧回执保留为 superseded。服务端会重新校验所有字段。</p>
                    </div>
                    {detail.lock ? (
                        <span className="manju-lock-chip">
                            <Check />
                            revision {detail.lock.revision}
                        </span>
                    ) : (
                        <span className="manju-lock-chip is-pending">
                            <LockKeyhole />
                            未锁定
                        </span>
                    )}
                </header>

                <div className="manju-form-grid">
                    <label>
                        <span>赛道</span>
                        <Select aria-label="赛道" value={form.track || undefined} options={TRACK_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("track", value, "track")} />
                    </label>
                    <label>
                        <span>素材来源</span>
                        <Select aria-label="素材来源" value={form.source || undefined} options={SOURCE_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("source", value, "source")} />
                    </label>
                    <label>
                        <span>视频模型</span>
                        <Select aria-label="视频模型" value={form.videoModel || undefined} options={MODEL_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("videoModel", value, "videoModel")} />
                    </label>
                    <label>
                        <span>画幅</span>
                        <Select aria-label="画幅" value={form.aspectRatio || undefined} options={ASPECT_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("aspectRatio", value, "aspectRatio")} />
                    </label>
                    <label>
                        <span>交付形态</span>
                        <Select aria-label="交付形态" value={form.deliveryMode || undefined} options={DELIVERY_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("deliveryMode", value, "deliveryMode")} />
                    </label>
                    <label>
                        <span>影像基底</span>
                        <Select aria-label="影像基底" value={form.styleBase || undefined} options={STYLE_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => changeStyleBase(value as ManjuStyleBaseID)} />
                    </label>
                    <label>
                        <span>目标时长</span>
                        <span className="manju-number-field">
                            <InputNumber aria-label="目标时长" min={0} max={3600} value={form.targetDurationSeconds} disabled={!canEdit} onChange={(value) => setField("targetDurationSeconds", Number(value) || 0)} />
                            <span>秒</span>
                        </span>
                    </label>
                    <label>
                        <span>
                            素材形态 <small>不是锁</small>
                        </span>
                        <Select
                            value={form.assetMode}
                            disabled={!canEdit}
                            onChange={(value) => setField("assetMode", value)}
                            options={[
                                { value: "M0", label: "M0 · 文字外观档案" },
                                { value: "M1", label: "M1 · 出图提示词 + 接线单（默认）" },
                                { value: "M2", label: "M2 · 资产图册" },
                            ]}
                        />
                    </label>
                    {segmentCount > 1 ? (
                        <label>
                            <span>
                                D 段缝衔接 <b>多段必填</b>
                            </span>
                            <Select aria-label="D 段缝衔接" value={form.seamMode || undefined} options={SEAM_OPTIONS} disabled={!canEdit} placeholder="请选择" onChange={(value) => setConfirmedField("seamMode", value, "seamMode")} />
                        </label>
                    ) : null}
                </div>

                <section className="manju-style-reference" aria-labelledby="manju-style-reference-title">
                    <header>
                        <div>
                            <span className="manju-kicker">全局强风格</span>
                            <h3 id="manju-style-reference-title">风格参考图</h3>
                            <p>只分析并迁移光线、色彩、材质、镜头和后期质感，不复刻构图、人物和场景。</p>
                        </div>
                        {form.styleReferenceResourceId ? (
                            <Button type="text" disabled={!canEdit} onClick={removeStyleReference}>
                                移除参考图
                            </Button>
                        ) : null}
                    </header>
                    <div className="manju-style-reference-body">
                        <Upload accept="image/jpeg,image/png,image/webp" disabled={!canEdit || styleReferenceUploading} maxCount={1} showUploadList={false} beforeUpload={uploadStyleReference}>
                            <button type="button" className="manju-style-reference-picker" disabled={!canEdit || styleReferenceUploading}>
                                {styleReferenceUploading ? (
                                    <span>
                                        <LoaderCircle className="is-spinning" />
                                        <strong>正在上传并分析风格</strong>
                                    </span>
                                ) : styleReferencePreview ? (
                                    <img src={styleReferencePreview} alt="风格参考图" />
                                ) : (
                                    <span>
                                        <ImagePlus />
                                        <strong>上传风格参考图</strong>
                                        <small>建议只放一张明确的色彩、光线和材质参考</small>
                                    </span>
                                )}
                            </button>
                        </Upload>
                        <div className="manju-style-reference-analysis">
                            <strong>{form.styleReferenceAnalysis ? "已生成强风格参数" : "尚未上传参考图"}</strong>
                            <p>{form.styleReferenceAnalysis || "上传后会自动提取明暗、反差、饱和度、冷暖倾向与主色板，并随 P0 一起冻结。"}</p>
                            {form.styleReferencePalette ? (
                                <div className="manju-style-palette">
                                    {form.styleReferencePalette
                                        .split("/")
                                        .map((color) => color.trim())
                                        .filter(Boolean)
                                        .map((color) => (
                                            <span key={color}>
                                                <i style={{ background: color }} />
                                                {color}
                                            </span>
                                        ))}
                                </div>
                            ) : null}
                        </div>
                    </div>
                </section>

                <div className="manju-segment-readout">
                    <section className="manju-style-lock" aria-labelledby="manju-style-lock-title">
                        <header className="manju-style-lock-head">
                            <div>
                                <span className="manju-kicker">
                                    <Sparkles /> 画风锁 · 74 组候选源
                                </span>
                                <h3 id="manju-style-lock-title">风格候选卡</h3>
                                <p>先确认影像基底，再按技法家族收窄；每次最多展示 4 张卡，选中后随 P0 一起冻结。</p>
                            </div>
                            <div className="manju-style-lock-actions">
                                <Button icon={<Sparkles />} disabled={!canEdit || !candidateCards.length} onClick={useRecommendedCard}>
                                    按你的推荐走
                                </Button>
                                <Button disabled={!canEdit} onClick={chooseCustomStyle}>
                                    自定义
                                </Button>
                                <Button type="text" onClick={() => setShowAllStyles((current) => !current)}>
                                    {showAllStyles ? "收起全部候选" : "查看全部 74 组"}
                                </Button>
                            </div>
                        </header>

                        {styleCatalogQuery.isLoading ? (
                            <div className="manju-style-state">
                                <Spin size="small" />
                                <span>读取 V7.4.26 画风目录</span>
                            </div>
                        ) : styleCatalogQuery.isError ? (
                            <InlineNotice tone="error">画风目录读取失败：{styleCatalogQuery.error instanceof Error ? styleCatalogQuery.error.message : "请稍后重试"}</InlineNotice>
                        ) : styleCatalogQuery.data ? (
                            <>
                                <div className="manju-style-family-row">
                                    <label>
                                        <span>技法家族</span>
                                        <Select
                                            value={styleFamilyId || undefined}
                                            disabled={!canEdit || !compatibleFamilies.length}
                                            options={compatibleFamilies.map((family, index) => ({
                                                value: family.id,
                                                label: `${index < 3 ? "推荐 · " : ""}${family.id} ${family.name}`,
                                            }))}
                                            onChange={(value) => {
                                                const nextFamily = String(value);
                                                setStyleFamilyId(nextFamily);
                                                if (selectedCard?.family !== nextFamily || form.styleCardId === "custom") {
                                                    clearStyleCardSelection();
                                                }
                                            }}
                                        />
                                    </label>
                                    <div className="manju-style-family-note">
                                        <strong>{selectedFamily?.name || "等待选择"}</strong>
                                        <span>{selectedFamily?.feature || "按基底过滤后，系统只展示兼容的技法家族。"}</span>
                                    </div>
                                    <span className="manju-style-count">{compatibleFamilies.length} 族兼容</span>
                                </div>

                                <div className="manju-style-candidates">
                                    {candidateCards.map((card, index) => (
                                        <button
                                            type="button"
                                            key={card.id}
                                            className={`manju-style-card${form.styleCardId === card.id ? " is-selected" : ""}`}
                                            disabled={!canEdit}
                                            aria-pressed={form.styleCardId === card.id}
                                            onClick={() => selectStyleCard(card)}
                                        >
                                            <span className="manju-style-card-index">{index === 0 ? "推荐" : String.fromCharCode(65 + index)}</span>
                                            <span className="manju-style-card-title">
                                                <strong>{card.name}</strong>
                                                <small>
                                                    #{String(card.number).padStart(2, "0")} · {card.group} · {card.nativeGenre}
                                                </small>
                                            </span>
                                            <span className="manju-style-ratings">
                                                <span>打斗 {styleRating(card.fightRating)}</span>
                                                <span>特效 {styleRating(card.effectRating)}</span>
                                                <span>落地 {card.grade}</span>
                                            </span>
                                            <span className="manju-style-card-params">{card.promptParameters}</span>
                                            <span className="manju-style-card-warning">易跑偏：{card.avoidanceWarning}</span>
                                        </button>
                                    ))}
                                </div>

                                {form.styleCardId === "custom" ? (
                                    <div className="manju-style-custom">
                                        <label>
                                            <span>正向画风</span>
                                            <Input.TextArea
                                                value={form.styleVisualPrompt || ""}
                                                disabled={!canEdit}
                                                rows={3}
                                                maxLength={1200}
                                                showCount
                                                placeholder="例如：北宋末年历史战争片，低饱和青灰冷色调，做旧铠甲与城砖，真实胶片颗粒"
                                                onChange={(event) => setField("styleVisualPrompt", event.target.value)}
                                            />
                                        </label>
                                        <label>
                                            <span>运镜参数</span>
                                            <Input.TextArea
                                                value={form.styleMotionParameters || ""}
                                                disabled={!canEdit}
                                                rows={3}
                                                maxLength={1200}
                                                showCount
                                                placeholder="例如：手持摄影，3-5度倾斜角，纪录片式微晃，轨道缓推"
                                                onChange={(event) => setField("styleMotionParameters", event.target.value)}
                                            />
                                        </label>
                                        <label>
                                            <span>禁用项</span>
                                            <Input.TextArea
                                                value={styleForbiddenText(form.styleForbiddenItems)}
                                                disabled={!canEdit}
                                                rows={3}
                                                maxLength={1200}
                                                showCount
                                                placeholder={"现代元素\n鲜艳色彩\n塑料质感铠甲"}
                                                onChange={(event) => setField("styleForbiddenItems", parseStyleForbiddenItems(event.target.value))}
                                            />
                                        </label>
                                    </div>
                                ) : selectedCard ? (
                                    <div className="manju-style-selection">
                                        <Check />
                                        <span>
                                            <strong>本次选择：{selectedCard.name}</strong>
                                            <small>
                                                内部登记 #{selectedCard.number} · {selectedFamily?.name} · {selectedCard.grade} 级落地度
                                            </small>
                                        </span>
                                    </div>
                                ) : (
                                    <p className="manju-style-empty">尚未选择。可以点候选卡、按你的推荐走，或改用自定义画风。</p>
                                )}

                                {showAllStyles ? (
                                    <div className="manju-style-all">
                                        <header>
                                            <strong>全部 74 组</strong>
                                            <span>与当前影像基底冲突的卡保持可见但不可选</span>
                                        </header>
                                        <div className="manju-style-all-grid">
                                            {styleCatalogQuery.data.items.map((card) => {
                                                const compatible = card.compatibleStyleBases.includes(form.styleBase);
                                                return (
                                                    <button
                                                        type="button"
                                                        key={card.id}
                                                        className={form.styleCardId === card.id ? "is-selected" : ""}
                                                        disabled={!canEdit || !compatible}
                                                        title={compatible ? card.promptParameters : `与${STYLE_BASE_LABELS[form.styleBase]}冲突`}
                                                        onClick={() => selectStyleCard(card)}
                                                    >
                                                        <span>#{String(card.number).padStart(2, "0")}</span>
                                                        <strong>{card.name}</strong>
                                                        <small>{compatible ? card.family : "基底冲突"}</small>
                                                    </button>
                                                );
                                            })}
                                        </div>
                                    </div>
                                ) : null}
                            </>
                        ) : null}
                    </section>

                    <span>
                        <Layers3 />
                    </span>
                    <div>
                        <strong>{segmentCount} 段</strong>
                        <small>
                            {form.targetDurationSeconds} 秒 ÷ 当前模型单段上限 {segmentLimit > 0 ? `${segmentLimit} 秒` : "待选模型"}
                        </small>
                    </div>
                    <span className="manju-segment-rule">{segmentCount > 1 ? "必须选择段缝" : "单段任务"}</span>
                </div>

                {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
                {!canEdit ? <InlineNotice tone="info">当前角色是{roleLabel(detail.accessRole)}，只能查看 P0。主理人或制片才能保存。</InlineNotice> : null}
                <footer className="manju-panel-actions">
                    <span>{detail.lock ? `上次保存 ${formatDate(detail.lock.createdAt)}` : "尚未生成 P0 回执"}</span>
                    <Button type="primary" icon={<LockKeyhole />} disabled={!canEdit || !readyToLockP0} loading={mutation.isPending} onClick={submit}>
                        锁定 P0
                    </Button>
                </footer>
            </section>

            <aside className="manju-receipt-panel">
                <header>
                    <span className="manju-kicker">服务端回执</span>
                    <h3>{detail.lock ? `Revision ${detail.lock.revision}` : "暂无回执"}</h3>
                </header>
                {detail.lock ? (
                    <>
                        <dl>
                            <div>
                                <dt>状态</dt>
                                <dd>locked</dd>
                            </div>
                            <div>
                                <dt>段数</dt>
                                <dd>{detail.lock.segmentCount}</dd>
                            </div>
                            <div>
                                <dt>画幅</dt>
                                <dd>{detail.lock.snapshot.aspectRatio}</dd>
                            </div>
                            <div>
                                <dt>模型</dt>
                                <dd>{detail.lock.snapshot.videoModel}</dd>
                            </div>
                            <div>
                                <dt>画风卡</dt>
                                <dd>{detail.lock.snapshot.styleCardName || detail.lock.snapshot.styleCardId}</dd>
                            </div>
                            <div>
                                <dt>画风族</dt>
                                <dd>{detail.lock.snapshot.styleFamily || "历史回执未记录"}</dd>
                            </div>
                            <div>
                                <dt>目录版本</dt>
                                <dd>{detail.lock.snapshot.styleCatalogVersion || "历史回执"}</dd>
                            </div>
                            <div>
                                <dt>创建时间</dt>
                                <dd>{formatDate(detail.lock.createdAt)}</dd>
                            </div>
                        </dl>
                        <div className="manju-hash">
                            <span>SHA-256</span>
                            <code>{detail.lock.contentHash}</code>
                        </div>
                    </>
                ) : (
                    <p className="manju-muted">完成左侧必填项并保存后，这里会出现当前 revision 和内容哈希。</p>
                )}
            </aside>
        </div>
    );
}

const SCRIPT_BRANCH_LABELS: Record<ManjuScriptBranch, string> = {
    adaptation: "小说改编",
    original: "原创故事",
    benchmark: "对标改编",
    polish: "外来剧本润色",
};

function scriptInputKindLabel(value?: "" | ManjuScriptInputKind) {
    return INPUT_KIND_OPTIONS.find((item) => item.value === value)?.label || "";
}

function trendResolutionLabel(value?: ManjuTrendResolution) {
    return TREND_RESOLUTION_OPTIONS.find((item) => item.value === value)?.label || "";
}

const SCRIPT_STATUS_LABELS: Record<ManjuScriptStageStatus, string> = {
    NOT_STARTED: "等待上游",
    IN_PROGRESS: "任务执行中",
    AWAITING_CONFIRMATION: "等待确认",
    AWAITING_USER_DECISION: "等待主理人决策",
    LOCKED: "已锁定",
    STALE: "上游已变更",
    BLOCKED: "执行失败",
};

function scriptStageStatusLabel(stage: ManjuScriptStage) {
    if (stage.status === "IN_PROGRESS") return stage.taskId ? "任务执行中" : "待执行";
    return SCRIPT_STATUS_LABELS[stage.status];
}

function productionStageOrder(stageKey: string) {
    if (stageKey === "director_storyboard") return 3;
    if (stageKey === "feed_package") return 4;
    if (stageKey === "qc_handoff") return 5;
    return 0;
}

function manjuDeliverableKindLabel(deliverable: ManjuUserDeliverable) {
    if (deliverable.kind === "markdown") return "Markdown 源稿";
    if (deliverable.kind === "board") return "离线 HTML 看板（仅 board/both）";
    if (deliverable.kind === "previs") return "预演故事板";
    if (deliverable.kind === "manifest") return "交付清单";
    return deliverable.kind;
}

function downloadManjuDeliverable(deliverable: ManjuUserDeliverable) {
    const url = URL.createObjectURL(new Blob([deliverable.content], { type: deliverable.mimeType }));
    const link = document.createElement("a");
    link.href = url;
    link.download = deliverable.fileName;
    link.rel = "noopener";
    link.click();
    URL.revokeObjectURL(url);
}

function PipelineWorkspace({ detail, activeStage, onStageChange, onChanged }: { detail: ManjuProjectDetail; activeStage: PipelineStageId; onStageChange: (stage: PipelineStageId) => void; onChanged: () => void }) {
    const scriptRunsQuery = useQuery({
        queryKey: ["manju", "script-runs", detail.project.id],
        queryFn: () => listManjuScriptRuns(detail.project.id),
    });
    const assetRunsQuery = useQuery({
        queryKey: ["manju", "asset-runs", detail.project.id],
        queryFn: () => listManjuAssetRuns(detail.project.id),
    });
    const productionRunsQuery = useQuery({
        queryKey: ["manju", "production-runs", detail.project.id],
        queryFn: () => listManjuProductionRuns(detail.project.id),
    });
    const currentRevision = detail.lock?.revision;
    const allScriptRuns = scriptRunsQuery.data?.items || [];
    const allAssetRuns = assetRunsQuery.data?.items || [];
    const allProductionRuns = productionRunsQuery.data?.items || [];
    const scriptRuns = allScriptRuns.filter((run) => isCurrentLockRevision(run.lockRevision, currentRevision));
    const assetRuns = allAssetRuns.filter((run) => isCurrentLockRevision(run.lockRevision, currentRevision));
    const productionRuns = allProductionRuns.filter((run) => isCurrentLockRevision(run.lockRevision, currentRevision));
    const latestScript = scriptRuns[0];
    const latestAsset = assetRuns[0];
    const latestProduction = productionRuns[0];
    const historicalRunCount = allScriptRuns.length - scriptRuns.length + allAssetRuns.length - assetRuns.length + allProductionRuns.length - productionRuns.length;

    const stageState = (stage: PipelineStageId) => {
        if (stage === "p1") {
            if (!latestScript) return { label: "未开始", tone: "waiting" };
            if (latestScript.status === "COMPLETED") return { label: "已锁定", tone: "done" };
            return { label: "进行中", tone: "active" };
        }
        if (stage === "p2") {
            if (!latestAsset) return { label: "未开始", tone: "waiting" };
            if (latestAsset.status === "COMPLETED") return { label: "已锁定", tone: "done" };
            return { label: "进行中", tone: "active" };
        }
        if (!latestProduction) return { label: "等待上游", tone: "waiting" };
        if (latestProduction.status === "COMPLETED") return { label: "已锁定", tone: "done" };
        const currentOrder = productionStageOrder(latestProduction.currentStage);
        const targetOrder = Number(stage.slice(1));
        if (targetOrder < currentOrder) return { label: "已锁定", tone: "done" };
        if (targetOrder === currentOrder) return { label: "进行中", tone: "active" };
        return { label: "等待上游", tone: "waiting" };
    };
    const activeMeta = PIPELINE_STAGES.find((stage) => stage.id === activeStage) || PIPELINE_STAGES[0];

    return (
        <div className="manju-pipeline-shell">
            <nav className="manju-pipeline-nav" aria-label="P1-P5 独立阶段">
                <header>
                    <span>P1-P5 独立阶段</span>
                    <small>一次只处理一个阶段</small>
                </header>
                {PIPELINE_STAGES.map((stage) => {
                    const Icon = stage.icon;
                    const state = stageState(stage.id);
                    return (
                        <button key={stage.id} type="button" className={`${activeStage === stage.id ? "is-active" : ""} is-${state.tone}`} aria-current={activeStage === stage.id ? "step" : undefined} onClick={() => onStageChange(stage.id)}>
                            <span className="manju-pipeline-nav-icon">
                                <Icon />
                            </span>
                            <span className="manju-pipeline-nav-copy">
                                <strong>{stage.label}</strong>
                                <small>{stage.detail}</small>
                            </span>
                            <span className="manju-pipeline-nav-state">{state.label}</span>
                        </button>
                    );
                })}
            </nav>
            <section className="manju-pipeline-panel" aria-label={`${activeMeta.label} 工作区`}>
                <header className="manju-pipeline-head">
                    <div>
                        <span className="manju-kicker">P1-P5 独立阶段</span>
                        <h2>{activeMeta.label}</h2>
                        <p>{activeMeta.detail}</p>
                    </div>
                </header>
                {historicalRunCount > 0 ? (
                    <InlineNotice tone="info">
                        检测到 {historicalRunCount} 条旧 P0 revision 的任务链，已从当前流程隔离。请基于当前 revision {currentRevision} 继续。
                    </InlineNotice>
                ) : null}
                {activeStage === "p1" ? <ScriptPipeline detail={detail} onChanged={onChanged} /> : null}
                {activeStage === "p2" ? <AssetPipeline detail={detail} scriptRuns={scriptRuns} onChanged={onChanged} /> : null}
                {activeStage === "p3" || activeStage === "p4" || activeStage === "p5" ? <ProductionPipeline detail={detail} activeStage={activeStage} onChanged={onChanged} /> : null}
            </section>
        </div>
    );
}

export function ScriptPipeline({ detail, onChanged }: { detail: ManjuProjectDetail; onChanged: () => void }) {
    const queryClient = useQueryClient();
    const [mode, setMode] = useState<ManjuScriptMode>("STEPWISE");
    const [inputKind, setInputKind] = useState<ManjuScriptInputKind>("novel");
    const [trendResolution, setTrendResolution] = useState<ManjuTrendResolution>("");
    const [selectedRunId, setSelectedRunId] = useState("");
    const [showSourceLibrary, setShowSourceLibrary] = useState(false);
    const [notice, setNotice] = useState<{ tone: "info" | "error" | "success"; text: string } | null>(null);
    const canEdit = ["owner", "producer", "writer"].includes(detail.accessRole);
    const source = detail.lock?.snapshot.source || "";
    const runsQuery = useQuery({
        queryKey: ["manju", "script-runs", detail.project.id],
        queryFn: () => listManjuScriptRuns(detail.project.id),
    });
    const currentRuns = (runsQuery.data?.items || []).filter((run) => isCurrentLockRevision(run.lockRevision, detail.lock?.revision) && run.status !== "ABANDONED");
    const selectedRun = currentRuns.find((run) => run.id === selectedRunId) || currentRuns[0];
    const runQuery = useQuery({
        queryKey: ["manju", "script-run", detail.project.id, selectedRun?.id],
        queryFn: () => getManjuScriptRun(detail.project.id, selectedRun!.id),
        enabled: Boolean(selectedRun?.id),
        refetchInterval: selectedRun?.status === "ACTIVE" ? 2500 : false,
    });
    const runDetail = runQuery.data?.item;
    const p1Loading = runsQuery.isLoading || runQuery.isLoading;
    const applyRun = (item: ManjuScriptRunDetail) => {
        queryClient.setQueryData(["manju", "script-run", detail.project.id, item.run.id], { item });
        queryClient.setQueryData<{ items: ManjuScriptRun[] }>(["manju", "script-runs", detail.project.id], (current) => {
            const items = current?.items || [];
            const exists = items.some((run) => run.id === item.run.id);
            return { items: exists ? items.map((run) => (run.id === item.run.id ? item.run : run)) : [item.run, ...items] };
        });
        void queryClient.invalidateQueries({ queryKey: ["manju", "script-runs", detail.project.id] });
        onChanged();
    };
    const createMutation = useMutation({
        mutationFn: () => createManjuScriptRun(detail.project.id, buildManjuScriptRunInput({
            mode,
            source,
            inputKind,
            trendResolution,
        })),
        onSuccess: ({ item }) => {
            applyRun(item);
            setSelectedRunId(item.run.id);
            setNotice({ tone: "success", text: `已建立 ${SCRIPT_BRANCH_LABELS[item.run.branch]}任务版。` });
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "剧本任务创建失败" }),
    });
    const unitPackage = runDetail ? buildManjuUnitPackage(runDetail) : null;
    const downloadUnitPackage = () => {
        if (!unitPackage) return;
        const url = URL.createObjectURL(new Blob([unitPackage.markdown], { type: "text/markdown;charset=utf-8" }));
        const link = document.createElement("a");
        link.href = url;
        link.download = unitPackage.fileName;
        link.click();
        URL.revokeObjectURL(url);
    };
    useEffect(() => {
        if (selectedRun) setMode(selectedRun.mode);
    }, [selectedRun?.id]);
    useEffect(() => {
        if (source !== "trend") setTrendResolution("");
    }, [source]);
    useEffect(() => {
        if (!currentRuns.length) {
            setSelectedRunId("");
            setShowSourceLibrary(true);
            return;
        }
        if (selectedRunId && !currentRuns.some((run) => run.id === selectedRunId) && runsQuery.isFetching) return;
        if (!currentRuns.some((run) => run.id === selectedRunId)) setSelectedRunId(currentRuns[0].id);
    }, [currentRuns, runsQuery.isFetching, selectedRunId]);

    if (!detail.lock) {
        return <InlineNotice tone="info">P0 尚未锁定。主理人或制片完成立项锁后，P1 才会开放。</InlineNotice>;
    }
    if (!detail.models?.text.modelKey) {
        return (
            <div className="manju-script-workspace">
                <InlineNotice tone="info">项目还没有文本模型路由。请先在“模型”页选择画布后台已启用的文本模型。</InlineNotice>
            </div>
        );
    }
    const adaptationBranch = source === "adaptation";
    const completedRuns = currentRuns.filter((run) => run.status === "COMPLETED");
    const runRouteLabel = runDetail
        ? [
            SCRIPT_BRANCH_LABELS[runDetail.run.branch],
            scriptInputKindLabel(runDetail.run.inputKind),
            runDetail.run.trendResolution ? `趋势落点：${trendResolutionLabel(runDetail.run.trendResolution)}` : "",
        ].filter(Boolean).join(" · ")
        : "";

    return (
        <div className="manju-script-workspace">
            {adaptationBranch ? (
                <section className="manju-script-library-bar">
                    <div>
                        <span className="manju-kicker">第一步 · 来源与范围</span>
                        <strong>当前项目来源库</strong>
                        <small>{currentRuns.length ? "需要制作新范围时再展开；已有任务版不会被覆盖。" : "先登记来源，再选择本次范围创建剧本任务版。"}</small>
                        <label>
                            <span>输入形态</span>
                            <Select
                                aria-label="改编输入形态"
                                value={inputKind}
                                options={INPUT_KIND_OPTIONS}
                                disabled={!canEdit || currentRuns.some((run) => run.status === "ACTIVE")}
                                onChange={setInputKind}
                            />
                        </label>
                    </div>
                    <Button icon={<BookOpenText />} onClick={() => setShowSourceLibrary((value) => !value)}>
                        {showSourceLibrary ? "收起来源库" : currentRuns.length ? "新建任务版" : "打开来源库"}
                    </Button>
                </section>
            ) : null}
            {adaptationBranch && showSourceLibrary ? (
                <SourceLibraryStepsPanel
                    projectId={detail.project.id}
                    canEdit={canEdit}
                    inputKind={inputKind}
                    completedRuns={completedRuns}
                    runsLoading={p1Loading}
                    hasActiveRun={currentRuns.some((run) => run.status === "ACTIVE" && Boolean(run.sourceId || run.sourceDocumentId))}
                    onCreated={(item) => {
                        applyRun(item);
                        setSelectedRunId(item.run.id);
                        setShowSourceLibrary(false);
                    }}
                />
            ) : null}
            <section className="manju-script-runbar">
                <div>
                    <span className="manju-kicker">VisiDeep AI · V7.4.26 · P1 剧本</span>
                    <h2>{runDetail ? runDetail.run.unitTitle || SCRIPT_BRANCH_LABELS[runDetail.run.branch] : "建立剧本任务版"}</h2>
                    <p>{runDetail ? `${runRouteLabel} · ${runDetail.run.sourceRangeLabel || "未登记来源范围"} · P0 revision ${runDetail.run.lockRevision}` : "支线由 P0 素材来源、输入形态与趋势落点共同决定。"}</p>
                </div>
                <div className="manju-script-run-actions">
                    {currentRuns.length ? (
                        <label>
                            <span>当前改编单元</span>
                            <Select
                                value={selectedRun?.id}
                                options={currentRuns.map((run) => ({
                                    value: run.id,
                                    label: `${run.unitTitle || "未命名改编单元"} · ${run.status === "COMPLETED" ? "已完成" : "进行中"}`,
                                }))}
                                onChange={setSelectedRunId}
                            />
                        </label>
                    ) : null}
                    {!adaptationBranch ? (
                        <>
                            {source === "trend" ? (
                                <label>
                                    <span>趋势落点</span>
                                    <Select
                                        aria-label="趋势落点"
                                        value={trendResolution || undefined}
                                        disabled={!canEdit || selectedRun?.status === "ACTIVE"}
                                        placeholder="必须选择执行方向"
                                        onChange={setTrendResolution}
                                        options={TREND_RESOLUTION_OPTIONS}
                                    />
                                </label>
                            ) : null}
                            <label>
                                <span>制作方式</span>
                                <Select
                                    value={mode}
                                    disabled={!canEdit || Boolean(selectedRun?.status === "ACTIVE")}
                                    onChange={setMode}
                                    options={[
                                        { value: "STEPWISE", label: "按阶段确认" },
                                        { value: "FULL_SCRIPT_PIPELINE", label: "完整流程" },
                                    ]}
                                />
                            </label>
                            <Button type="primary" icon={<Play />} disabled={!canEdit || selectedRun?.status === "ACTIVE" || !canCreateManjuScriptRun(source, trendResolution)} loading={createMutation.isPending} onClick={() => createMutation.mutate()}>
                                {selectedRun?.status === "ACTIVE" ? "当前任务版进行中" : selectedRun ? "创建新任务版" : "开始 P1"}
                            </Button>
                        </>
                    ) : null}
                </div>
            </section>

            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {runsQuery.isError ? <InlineNotice tone="error">剧本任务列表读取失败，请刷新后重试。</InlineNotice> : null}
            {!runDetail && p1Loading ? (
                <section className="manju-script-empty is-loading" aria-busy="true">
                    <Spin />
                    <strong>正在读取剧本任务版</strong>
                    <span>系统正在恢复当前改编单元和阶段状态。</span>
                </section>
            ) : !runDetail ? (
                <section className="manju-script-empty">
                    <BookOpenText />
                    <strong>尚未创建剧本任务版</strong>
                    <span>P0 当前来源是“{sourceLabel(detail.lock.snapshot.source)}”，将进入对应支线。</span>
                </section>
            ) : (
                <>
                    <SourceLedger detail={runDetail} />
                    <section className="manju-script-timeline" aria-label="P1 剧本阶段">
                        <header>
                            <div>
                                <span className="manju-kicker">六阶段编剧链</span>
                                <h3>{runDetail.run.mode === "STEPWISE" ? "逐步确认 · 真实阶段状态" : "完整流程 · 真实阶段状态"}</h3>
                            </div>
                            <div className="manju-inline-actions">
                                {unitPackage ? (
                                    <Button icon={<Download />} onClick={downloadUnitPackage}>
                                        下载本章交付文档
                                    </Button>
                                ) : null}
                                <Button icon={<RefreshCw />} loading={runQuery.isFetching} onClick={() => void runQuery.refetch()}>
                                    刷新任务状态
                                </Button>
                            </div>
                        </header>
                        <div className="manju-script-stage-stack">
                            {runDetail.stages
                                .filter((stage) => !(stage.key === "source_registration" && runDetail.source && runDetail.run.sourceDocumentId))
                                .map((stage) => (
                                    <ScriptStageCard
                                        key={stage.id}
                                        projectId={detail.project.id}
                                        projectName={detail.project.name}
                                        detail={runDetail}
                                        stage={stage}
                                        canEdit={canEdit}
                                        onUpdated={applyRun}
                                    />
                                ))}
                        </div>
                    </section>
                </>
            )}
        </div>
    );
}

function SourceLedger({ detail }: { detail: ManjuScriptRunDetail }) {
    if (!detail.source) return null;
    return (
        <section className="manju-script-source-ledger">
            <span className="manju-contract-icon">
                <FileText />
            </span>
            <div>
                <span className="manju-kicker">{detail.run.sourceDocumentId ? "来源账本 · 章节已自动登记" : "来源账本"}</span>
                <strong>{detail.source.title}</strong>
                <small>
                    {detail.source.rangeLabel || "完整范围"} · {detail.source.characterCount.toLocaleString()} 字
                </small>
            </div>
            <code>{detail.source.contentHash.slice(0, 12)}</code>
        </section>
    );
}

function ScriptStageCard({ projectId, projectName, detail, stage, canEdit, onUpdated }: { projectId: string; projectName: string; detail: ManjuScriptRunDetail; stage: ManjuScriptStage; canEdit: boolean; onUpdated: (item: ManjuScriptRunDetail) => void }) {
    const artifact = detail.artifacts?.find((item) => item.id === stage.artifactId);
    const [instruction, setInstruction] = useState("");
    const [acceptQualityRisk, setAcceptQualityRisk] = useState(false);
    const [notice, setNotice] = useState<{ tone: "error" | "success"; text: string } | null>(null);
    const [manualCorrectionOpen, setManualCorrectionOpen] = useState(false);
    const runMutation = useMutation({
        mutationFn: () => runManjuScriptStage(projectId, detail.run.id, stage.key, { instruction: instruction.trim() }),
        onSuccess: ({ item }) => {
            setNotice(null);
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "剧本阶段启动失败" }),
    });
    const confirmMutation = useMutation({
        mutationFn: () => confirmManjuScriptStage(projectId, detail.run.id, stage.key),
        onSuccess: ({ item }) => {
            setNotice(null);
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "剧本阶段确认失败" }),
    });
    const decideRoundtableMutation = useMutation({
        mutationFn: (input: { decision: ManjuRoundtableDecision; acceptQualityRisk: boolean }) =>
            decideManjuRoundtable(projectId, detail.run.id, input),
        onSuccess: ({ item }, input) => {
            setNotice({
                tone: "success",
                text: input.decision === "PROCEED_CURRENT" ? "已接受当前稿并锁定圆桌审核。" : "已重新打开正式剧本，等待生成新版本。",
            });
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "圆桌决策提交失败" }),
    });
    const regenerateArtifactMutation = useMutation({
        mutationFn: () => regenerateManjuScriptArtifact(projectId, detail.run.id, stage.key, instruction.trim()),
        onSuccess: ({ item }) => {
            setInstruction("");
            setNotice(null);
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "当前工件重新生成失败" }),
    });
    const manualCorrectionMutation = useMutation({
        mutationFn: (content: string) =>
            manualCorrectManjuScriptArtifact(projectId, detail.run.id, stage.key, {
                content,
                note: "用户人工修正当前 P1 工件",
            }),
        onSuccess: ({ item }) => {
            setManualCorrectionOpen(false);
            setNotice({ tone: "success", text: "人工修正已保存，请核对后确认锁定。" });
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "人工修正未通过校验" }),
    });
    const isCurrent = detail.run.currentStage === stage.key;
    const runActive = detail.run.status === "ACTIVE";
    const runReadable = runActive || detail.run.status === "COMPLETED";
    const canConfirm = canEdit && runActive && stage.key !== "roundtable_review" && stage.status === "AWAITING_CONFIRMATION";
    const canDecideRoundtable = canEdit && runActive && stage.key === "roundtable_review" && stage.status === "AWAITING_USER_DECISION";
    const qualityRiskRequired = stage.reviewGrade === "B" || stage.reviewGrade === "NOT_READY";
    const canRegenerateArtifact = canEdit && runReadable && Boolean(artifact) && !stage.taskId && ["LOCKED", "BLOCKED", "AWAITING_CONFIRMATION"].includes(stage.status);
    const canRun = canEdit && runActive && isCurrent && ["IN_PROGRESS", "BLOCKED", "STALE"].includes(stage.status) && !canRegenerateArtifact;
    const canManualCorrect = canEdit && stage.status === "BLOCKED" && Boolean(stage.draft?.content);
    const stageSubtitle =
        stage.key === "source_registration" && detail.source
            ? `${detail.source.rangeLabel || "完整范围"} · ${detail.source.characterCount.toLocaleString()} 字`
            : stage.taskId
              ? `任务 ${stage.taskId.slice(0, 8)}`
              : stage.key === "source_registration"
                ? "等待登记来源"
                : "尚未提交模型";

    if (stage.key === "source_registration" && !detail.source && isCurrent && stage.status === "IN_PROGRESS") {
        if (detail.run.branch === "adaptation") {
            return (
                <article className="manju-script-stage is-blocked">
                    <header>
                        <span className="manju-script-step">{String(stage.position).padStart(2, "0")}</span>
                        <span className="manju-script-stage-title">
                            <strong>当前单元没有绑定原著章节</strong>
                            <small>这是旧版本遗留任务，不需要再次手填小说正文</small>
                        </span>
                        <span className="manju-script-status is-blocked">需要重建</span>
                    </header>
                    <InlineNotice tone="error">请点上方“新建下一章”，从原著库重新选择章节。这个没有来源的旧单元会被自动废弃，不再阻塞下一章。</InlineNotice>
                </article>
            );
        }
        return <SourceRegistrationCard key={detail.run.branch} projectId={projectId} projectName={projectName} detail={detail} stage={stage} canEdit={canEdit} onUpdated={onUpdated} />;
    }

    return (
        <article className={`manju-script-stage is-${stage.status.toLowerCase()}`}>
            <header>
                <span className="manju-script-step">{String(stage.position).padStart(2, "0")}</span>
                <span className="manju-script-stage-title">
                    <strong>{stage.title}</strong>
                    <small>{stageSubtitle}</small>
                </span>
                <span className={`manju-script-status is-${stage.status.toLowerCase()}`}>{scriptStageStatusLabel(stage)}</span>
            </header>
            {stage.error ? <InlineNotice tone="error">{stage.error}</InlineNotice> : null}
            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {canManualCorrect ? (
                <div className="manju-script-input">
                    <Button icon={<FileText />} onClick={() => setManualCorrectionOpen(true)}>
                        人工修正当前工件
                    </Button>
                </div>
            ) : null}
            {stage.key === "source_registration" && detail.source ? (
                <div className="manju-script-source-summary">
                    <FileText />
                    <span>
                        <strong>{detail.source.title}</strong>
                        <small>
                            {detail.source.rangeLabel || "完整范围"} · {detail.source.characterCount.toLocaleString()} 字
                        </small>
                    </span>
                    <code>{detail.source.contentHash.slice(0, 12)}</code>
                </div>
            ) : null}
            {stage.status === "IN_PROGRESS" && stage.taskId ? (
                <div className="manju-script-running">
                    <Spin size="small" />
                    <span>VisiDeep AI 文本任务已提交，积分已按画布计费规则冻结。页面每 2.5 秒读取真实状态。</span>
                </div>
            ) : null}
            {canRun && !stage.taskId ? (
                <div className="manju-script-input">
                    <label>
                        <span>本阶段补充</span>
                        <Input.TextArea value={instruction} rows={3} maxLength={4000} showCount placeholder="可填写本次取舍、禁区、篇幅或审核重点；留空则只使用 P0、来源账本与上游工件。" onChange={(event) => setInstruction(event.target.value)} />
                    </label>
                    <Button type="primary" icon={<Play />} loading={runMutation.isPending} onClick={() => runMutation.mutate()}>
                        执行 {stage.title}
                    </Button>
                </div>
            ) : null}
            {artifact ? <ArtifactPreview artifact={artifact} /> : null}
            {canRegenerateArtifact ? (
                <>
                    <div className="manju-script-input">
                        <label>
                            <span>当前工件修正要求</span>
                            <Input.TextArea value={instruction} rows={3} maxLength={4000} showCount placeholder="可填写本次只针对当前工件的修正要求；留空则只使用 P0、来源账本与上游锁定工件。" onChange={(event) => setInstruction(event.target.value)} />
                        </label>
                    </div>
                    <footer className="manju-script-stage-actions">
                        <span>只重新生成当前工件；上游锁定工件保留，后续 P1 工件会等待新版本，已有 P2/P3 会转为历史版本。</span>
                        <Button icon={<RefreshCw />} loading={regenerateArtifactMutation.isPending} onClick={() => regenerateArtifactMutation.mutate()}>
                            重新生成当前工件
                        </Button>
                    </footer>
                </>
            ) : null}
            {canConfirm ? (
                <footer className="manju-script-stage-actions">
                    <span>确认后本阶段锁定，后续阶段才能读取。</span>
                    <Button type="primary" icon={<Check />} loading={confirmMutation.isPending} onClick={() => confirmMutation.mutate()}>
                        确认并进入下一阶段
                    </Button>
                </footer>
            ) : null}
            {canDecideRoundtable ? (
                <footer className="manju-script-stage-actions manju-roundtable-actions">
                    <span>五席意见已生成。请明确选择接受当前稿，或授权进入一次精修/重写。</span>
                    {qualityRiskRequired ? (
                        <Checkbox
                            className="manju-roundtable-risk-ack"
                            aria-label="确认接受圆桌质量风险"
                            checked={acceptQualityRisk}
                            onChange={(event) => setAcceptQualityRisk(event.target.checked)}
                        >
                            我已知晓当前等级为 {stage.reviewGrade}，仍接受质量风险（USER_ACCEPTED_QUALITY_RISK）
                        </Checkbox>
                    ) : null}
                    <Button
                        type="primary"
                        icon={<Check />}
                        loading={decideRoundtableMutation.isPending}
                        disabled={qualityRiskRequired && !acceptQualityRisk}
                        onClick={() => decideRoundtableMutation.mutate({ decision: "PROCEED_CURRENT", acceptQualityRisk })}
                    >
                        接受当前稿
                    </Button>
                    <Button
                        danger
                        icon={<RefreshCw />}
                        loading={decideRoundtableMutation.isPending}
                        onClick={() => decideRoundtableMutation.mutate({ decision: "REFINE_OR_REWRITE", acceptQualityRisk: false })}
                    >
                        精修并重写
                    </Button>
                </footer>
            ) : null}
            {runMutation.isError ? <InlineNotice tone="error">{runMutation.error instanceof Error ? runMutation.error.message : "阶段任务提交失败"}</InlineNotice> : null}
            {stage.draft ? (
                <ManjuArtifactEditor
                    open={manualCorrectionOpen}
                    title={`人工修正 ${stage.title}`}
                    content={stage.draft.content}
                    validationJson={
                        manualCorrectionMutation.error
                            ? JSON.stringify({
                                  issues: [
                                      manualCorrectionMutation.error instanceof Error
                                          ? manualCorrectionMutation.error.message
                                          : "人工修正未通过校验",
                                  ],
                              })
                            : stage.draft.validationJson
                    }
                    loading={manualCorrectionMutation.isPending}
                    onCancel={() => setManualCorrectionOpen(false)}
                    onSubmit={(content) => manualCorrectionMutation.mutate(content)}
                />
            ) : null}
        </article>
    );
}

function SourceRegistrationCard({
    projectId,
    projectName,
    detail,
    stage,
    canEdit,
    onUpdated,
}: {
    projectId: string;
    projectName: string;
    detail: ManjuScriptRunDetail;
    stage: ManjuScriptStage;
    canEdit: boolean;
    onUpdated: (item: ManjuScriptRunDetail) => void;
}) {
    const initial = sourceDraft(detail.run.branch, projectName);
    const [draft, setDraft] = useState<ManjuScriptSourceInput>(initial);
    const mutation = useMutation({
        mutationFn: () => registerManjuScriptSource(projectId, detail.run.id, draft),
        onSuccess: ({ item }) => onUpdated(item),
    });
    const copy = sourceInputCopy(detail.run.branch);
    return (
        <article className="manju-script-stage is-source is-in_progress">
            <header>
                <span className="manju-script-step">{String(stage.position).padStart(2, "0")}</span>
                <span className="manju-script-stage-title">
                    <strong>{copy.title}</strong>
                    <small>来源登记 · {SCRIPT_BRANCH_LABELS[detail.run.branch]}</small>
                </span>
                <span className="manju-script-status is-in_progress">等待输入</span>
            </header>
            <div className="manju-script-source-form">
                <label>
                    <span>{copy.titleLabel}</span>
                    <Input value={draft.title} disabled={!canEdit} maxLength={240} onChange={(event) => setDraft((current) => ({ ...current, title: event.target.value }))} />
                </label>
                <label>
                    <span>{copy.rangeLabel}</span>
                    <Input value={draft.rangeLabel} disabled={!canEdit} maxLength={240} placeholder={copy.rangePlaceholder} onChange={(event) => setDraft((current) => ({ ...current, rangeLabel: event.target.value }))} />
                </label>
                <label>
                    <span>{copy.authorizationLabel}</span>
                    <Input value={draft.authorizationScope} disabled={!canEdit} onChange={(event) => setDraft((current) => ({ ...current, authorizationScope: event.target.value }))} />
                </label>
                <label className="is-wide">
                    <span>{copy.contentLabel}</span>
                    <Input.TextArea value={draft.content} disabled={!canEdit} rows={9} showCount maxLength={500000} placeholder={copy.contentPlaceholder} onChange={(event) => setDraft((current) => ({ ...current, content: event.target.value }))} />
                </label>
            </div>
            {mutation.isError ? <InlineNotice tone="error">{mutation.error instanceof Error ? mutation.error.message : "来源登记失败"}</InlineNotice> : null}
            <footer className="manju-script-stage-actions">
                <span>登记后生成不可变来源哈希与 SOURCE_MANIFEST.json，不把正文直接回传到页面。</span>
                <Button type="primary" icon={<Check />} disabled={!canEdit || !draft.title.trim() || !draft.content.trim()} loading={mutation.isPending} onClick={() => mutation.mutate()}>
                    登记来源
                </Button>
            </footer>
        </article>
    );
}

function sourceDraft(branch: ManjuScriptBranch, projectName: string): ManjuScriptSourceInput {
    switch (branch) {
        case "original":
            return { title: projectName, authorizationScope: "本项目原创；无外部作品改编授权", rangeLabel: "目标集数与单集时长", content: "", metadata: { sourceType: "original" } };
        case "benchmark":
            return { title: "", authorizationScope: "仅用于差异化分析，不复制原作表达", rangeLabel: "参考作品与使用范围", content: "", metadata: { sourceType: "benchmark" } };
        case "polish":
            return { title: "外来剧本", authorizationScope: "已获原作者润色授权", rangeLabel: "完整剧本", content: "", metadata: { sourceType: "polish" } };
        default:
            return { title: "", authorizationScope: "", rangeLabel: "", content: "", metadata: { sourceType: "novel" } };
    }
}

function sourceInputCopy(branch: ManjuScriptBranch) {
    switch (branch) {
        case "original":
            return {
                title: "原创故事登记",
                titleLabel: "故事名",
                rangeLabel: "目标篇幅",
                rangePlaceholder: "例如：60 集 × 60 秒",
                authorizationLabel: "原创声明",
                contentLabel: "故事种子",
                contentPlaceholder: "填写人物、世界、核心冲突、受众、创作边界与不能写的禁区。",
            };
        case "benchmark":
            return {
                title: "对标材料登记",
                titleLabel: "参考作品",
                rangeLabel: "使用范围",
                rangePlaceholder: "例如：仅分析第 1-10 集结构",
                authorizationLabel: "边界声明",
                contentLabel: "对标材料正文",
                contentPlaceholder: "填写可公开分析的剧情结构、人物关系、节拍与市场信息；明确禁止复制项。",
            };
        case "polish":
            return {
                title: "外来剧本登记",
                titleLabel: "剧本名",
                rangeLabel: "润色范围",
                rangePlaceholder: "例如：全 60 集，仅补表演与调度",
                authorizationLabel: "授权范围",
                contentLabel: "外来剧本正文",
                contentPlaceholder: "粘贴待润色剧本。系统只补表演层与调度层，不改台词、事件、人物和结构。",
            };
        default:
            return {
                title: "小说改编登记",
                titleLabel: "原著名称",
                rangeLabel: "本次章节 / 段落",
                rangePlaceholder: "例如：第 1 章第 1-3 节（只登记本次要改编的范围）",
                authorizationLabel: "授权范围",
                contentLabel: "原著正文",
                contentPlaceholder: "粘贴本次获授权改编的章节或段落正文。",
            };
    }
}

function ArtifactPreview({ artifact }: { artifact: ManjuScriptArtifact }) {
    return (
        <details className="manju-script-artifact">
            <summary>
                <FileText />
                <span>阶段工件</span>
                <strong>{artifact.kind}</strong>
                <code>v{artifact.version}</code>
                <ChevronRight />
            </summary>
            <pre>{artifact.content}</pre>
        </details>
    );
}

const ASSET_STAGE_STATUS_LABELS: Record<ManjuAssetStageStatus, string> = {
    NOT_STARTED: "等待上游",
    IN_PROGRESS: "待执行 / 执行中",
    AWAITING_CONFIRMATION: "等待确认",
    LOCKED: "已锁定",
    STALE: "上游已变更",
    BLOCKED: "执行失败",
};

const ASSET_MODE_OPTIONS: Array<{ value: ManjuAssetMode; label: string; description: string }> = [
    { value: "M0", label: "M0 纯提示词", description: "不出图号，不要求图片，一致性靠文字锁。" },
    { value: "M1", label: "M1 编号占位", description: "默认模式；输出提示词、图号和素材接线单，图片由用户在外部平台挂载。" },
    { value: "M2", label: "M2 已入库", description: "只有用户主动交回并登记参考图后，才做图像级核验。" },
];

function assetStageStatusLabel(stage: ManjuAssetStage) {
    if (stage.status === "IN_PROGRESS" && stage.taskId) return "文本任务执行中";
    return ASSET_STAGE_STATUS_LABELS[stage.status];
}

function AssetArtifactPreview({ artifact }: { artifact: ManjuAssetArtifact }) {
    return (
        <details className="manju-script-artifact is-asset">
            <summary>
                <FileText />
                <span>P2 工件</span>
                <strong>{artifact.kind}</strong>
                <code>v{artifact.version}</code>
                <ChevronRight />
            </summary>
            <pre>{artifact.content}</pre>
        </details>
    );
}

type ManjuCanvasPackagePanelProps = {
    projectId: string;
    projectName: string;
    stage: ManjuCanvasPackageStage;
    runId: string;
    sourceStages?: ManjuCanvasPackageStage[];
    requiredSourceStages?: ManjuCanvasPackageStage[];
    sourceContext?: {
        productionRunId: string;
        p3ArtifactId: string;
        storyboardImages: ManjuProductionImage[];
    };
    canEdit: boolean;
    disabledReason?: string;
    onNotice: (notice: { tone: "info" | "error" | "success"; text: string }) => void;
};

function ManjuCanvasPackagePanel({ projectId, projectName, stage, runId, sourceStages = [], requiredSourceStages = sourceStages, sourceContext, canEdit, disabledReason, onNotice }: ManjuCanvasPackagePanelProps) {
    const queryClient = useQueryClient();
    const navigate = useNavigate();
    const importsQuery = useQuery({
        queryKey: ["manju", "canvas-imports", projectId],
        queryFn: () => listManjuCanvasImports(projectId),
    });
    const imports = importsQuery.data?.items || [];
    const activeImports = imports.filter((item) => item.status === "ACTIVE");
    const projectCanvasImport = imports.find((item) => item.stage === "p2_assets") || imports.find((item) => item.stage === "p3_images") || imports.find((item) => item.stage === "p4_videos");
    const currentImport = activeImports.find((item) => item.stage === stage && (stage === "p2_assets" ? item.assetRunId === runId : item.productionRunId === runId));
    const hasCurrentP3StoryboardImages = Boolean(sourceContext && sourceContext.p3ArtifactId && sourceContext.storyboardImages.some((image) =>
        image.runId === sourceContext.productionRunId
        && image.artifactId === sourceContext.p3ArtifactId
        && image.status === "READY"
        && image.status !== "SUPERSEDED"
        && Boolean(image.resourceId),
    ));
    const requiredMissing = requiredSourceStages.some((sourceStage) =>
        sourceStage === "p3_images"
            ? !hasCurrentP3StoryboardImages
            : !activeImports.some((item) => item.stage === sourceStage),
    );
    const blockedReason = disabledReason || (requiredMissing ? "请先把上一阶段导入画布，并在画布中完成对应资源生成。" : "");
    const importMutation = useMutation({
        mutationFn: async () => {
            const { item: pkg } = await getManjuCanvasPackage(projectId, stage, runId);
            const availableSourceStages = sourceStages.filter((sourceStage) =>
                sourceStage === "p3_images"
                    ? hasCurrentP3StoryboardImages
                    : activeImports.some((item) => item.stage === sourceStage),
            );
            const references = availableSourceStages.length
                ? await loadManjuCanvasReferences(projectId, availableSourceStages, sourceContext)
                : [];
            let canvasId = projectCanvasImport?.canvasId;
            let addedTaskCount = pkg.tasks.length;
            if (canvasId) {
                const existing = await loadCanvasProjectForEditing(canvasId);
                if (!existing) throw new Error("项目画布不存在或当前账号无权访问");
                const pipeline = createManjuCanvasPackage(pkg, references, nextManjuCanvasCenter(existing.nodes));
                const merged = mergeManjuCanvasPackage(existing.nodes, existing.connections, pipeline, stage);
                addedTaskCount = merged.addedTaskCount;
                const nodes = normalizeManjuCanvasLayout(merged.nodes);
                const focusNodes = nodes.filter((node) =>
                    node.metadata?.manjuStage === stage
                    || (stage === "p2_assets" && isManjuAssetTable(node))
                    || (stage === "p3_images" && node.metadata?.batchTable?.manjuStoryboardTable)
                    || (stage === "p4_videos" && node.metadata?.manjuStage === "p4_videos"),
                );
                const viewport = focusNodes.length && typeof window !== "undefined"
                    ? manjuCanvasViewportForNodes(focusNodes, {
                        width: Math.max(480, window.innerWidth),
                        height: Math.max(480, window.innerHeight - 56),
                    })
                    : undefined;
                await syncLocalCanvasSnapshot(canvasId, {
                    nodes,
                    connections: merged.connections,
                    ...(viewport ? { viewport } : {}),
                });
            } else {
                const pipeline = createManjuCanvasPackage(pkg, references, { x: 0, y: 0 });
                canvasId = (
                    await createCanvasProjectWithRemoteSync(`${projectName} · 漫剧项目画布`, projectId, {
                        nodes: pipeline.nodes,
                        connections: pipeline.connections,
                    })
                ).id;
            }
            await saveManjuCanvasImport(projectId, {
                stage,
                assetRunId: stage === "p2_assets" ? runId : undefined,
                productionRunId: stage === "p2_assets" ? undefined : runId,
                canvasId,
                packageHash: pkg.hash,
            });
            return { canvasId, taskCount: addedTaskCount };
        },
        onSuccess: ({ canvasId, taskCount }) => {
            void queryClient.invalidateQueries({ queryKey: ["manju", "canvas-imports", projectId] });
            onNotice({ tone: "success", text: taskCount ? `已向项目画布追加 ${taskCount} 个任务。` : "项目画布中已有这些任务，本次只更新了阶段映射。" });
            const studioStage = stage === "p2_assets" ? "p2" : stage === "p3_images" ? "p3" : "p4";
            navigate(`/canvas/${canvasId}?from=manju&manjuProjectId=${encodeURIComponent(projectId)}&stage=${studioStage}`);
        },
        onError: (error) => onNotice({ tone: "error", text: error instanceof Error ? error.message : "漫剧生产包导入画布失败" }),
    });

    return (
        <section className="manju-canvas-package" aria-label="画布生产包">
            <header>
                <div>
                    <strong>画布生产包</strong>
                    <span>{projectCanvasImport ? `项目画布已关联 · 本阶段${currentImport ? "已导入" : "待追加"}` : "首次导入会建立这个漫剧项目唯一的画布。"}</span>
                </div>
                <div>
                    {projectCanvasImport ? (
                        <Button
                            icon={<FolderOpen />}
                            onClick={() => navigate(`/canvas/${projectCanvasImport.canvasId}?from=manju&manjuProjectId=${encodeURIComponent(projectId)}&stage=${stage === "p2_assets" ? "p2" : stage === "p3_images" ? "p3" : "p4"}`)}
                        >
                            打开项目画布
                        </Button>
                    ) : null}
                    <Button type={currentImport ? "default" : "primary"} icon={<Sparkles />} disabled={!canEdit || Boolean(blockedReason) || importsQuery.isLoading} loading={importMutation.isPending} onClick={() => importMutation.mutate()}>
                        {projectCanvasImport ? "追加到项目画布" : "创建项目画布并导入"}
                    </Button>
                </div>
            </header>
            {blockedReason ? <InlineNotice tone="info">{blockedReason}</InlineNotice> : null}
            <div className="manju-canvas-package-flow">
                <span>漫剧阶段</span>
                <ArrowRight />
                <span>画布任务框</span>
                <ArrowRight />
                <span>真实生成</span>
                <ArrowRight />
                <span>项目资产库</span>
            </div>
            {importsQuery.isError ? <InlineNotice tone="error">画布导入状态读取失败，请刷新后重试。</InlineNotice> : null}
        </section>
    );
}

async function loadManjuCanvasReferences(
    projectId: string,
    sourceStages: ManjuCanvasPackageStage[],
    sourceContext?: {
        productionRunId: string;
        p3ArtifactId: string;
        storyboardImages: ManjuProductionImage[];
    },
): Promise<ManjuCanvasReference[]> {
    const byCode = new Map<string, ManjuCanvasImportReference>();
    const sourceReferencePages = await Promise.all(sourceStages.map(async (sourceStage) => {
        if (sourceStage === "p3_images") {
            if (!sourceContext) return { items: [] };
            const items = sourceContext.storyboardImages
                .filter((image) =>
                    image.runId === sourceContext.productionRunId
                    && image.artifactId === sourceContext.p3ArtifactId
                    && image.status === "READY"
                    && image.status !== "SUPERSEDED"
                    && Boolean(image.resourceId),
                )
                .map((image): ManjuCanvasImportReference => ({
                    code: image.segmentCode,
                    taskId: image.taskId,
                    title: image.segmentCode,
                    prompt: image.shotLabel,
                    content: `resource:${image.resourceId}`,
                    storageKey: `resource:${image.resourceId}`,
                    category: "material",
                    kind: "image",
                }));
            return { items };
        }
        return getManjuCanvasImportReferences(projectId, sourceStage);
    }));
    for (const { items } of sourceReferencePages) {
        for (const item of items) {
            const key = manjuReferenceCodeKey(item.code);
            const previous = byCode.get(key);
            byCode.set(key, previous ? {
                ...previous,
                ...item,
                code: key,
                title: item.title || previous.title,
                prompt: item.prompt || previous.prompt,
                content: item.content || previous.content,
                storageKey: item.storageKey || previous.storageKey,
                taskId: item.taskId || previous.taskId,
                assetCode: item.assetCode || previous.assetCode,
                assetId: item.assetId || previous.assetId,
                category: item.category || previous.category,
            } : { ...item, code: key });
        }
    }
    const visualCodes = Array.from(byCode.values())
        .map((item) => manjuReferenceCodeKey(item.code))
        .filter((code) => /^(?:CHR|SCN|PRP)-/.test(code));
    const codes = Array.from(new Set(visualCodes));
    return codes.map((code) => {
        const item = byCode.get(manjuReferenceCodeKey(code));
        if (!item) return { code, title: manjuAssetNameFromCode(code) };
        return {
            code,
            title: item.title || code,
            prompt: item.prompt,
            content: item.content,
            storageKey: item.storageKey,
            taskId: item.taskId,
            assetCode: item.assetCode || (code.startsWith("CHR-") || code.startsWith("SCN-") || code.startsWith("PRP-") ? code : undefined),
            assetId: item.assetId,
            category: item.category,
        };
    });
}

function AssetGate({ children }: { children: React.ReactNode }) {
    return (
        <section className="manju-asset-workspace" aria-label="P2 数字资产工作区">
            <header className="manju-asset-head">
                <div>
                    <span className="manju-kicker">VisiDeep AI · V7.4.26 · P2 数字资产</span>
                    <h3>P2 数字资产</h3>
                    <p>先提取实体，再确认完整资产清单，最后按素材形态编译交付。M0/M1 不需要上传图片。</p>
                </div>
                <span className="manju-truth-badge">等待上游</span>
            </header>
            {children}
        </section>
    );
}
function AssetPipeline({ detail, scriptRuns, onChanged }: { detail: ManjuProjectDetail; scriptRuns: ManjuScriptRun[]; onChanged: () => void }) {
    const queryClient = useQueryClient();
    const canEdit = ["owner", "producer", "director"].includes(detail.accessRole);
    const completedScripts = scriptRuns.filter((run) => run.status === "COMPLETED");
    const [scriptRunId, setScriptRunId] = useState(completedScripts[0]?.id || "");
    const [assetMode, setAssetMode] = useState<ManjuAssetMode>((detail.lock?.snapshot.assetMode as ManjuAssetMode | undefined) || "M1");
    const [baseAssetRunId, setBaseAssetRunId] = useState("");
    const [reuseAssets, setReuseAssets] = useState(false);
    const [selectedRunId, setSelectedRunId] = useState("");
    const [notice, setNotice] = useState<{ tone: "info" | "error" | "success"; text: string } | null>(null);
    const [referenceDraft, setReferenceDraft] = useState({ code: "", assetType: "CHR", resourceId: "", url: "", notes: "" });

    const runsQuery = useQuery({
        queryKey: ["manju", "asset-runs", detail.project.id],
        queryFn: () => listManjuAssetRuns(detail.project.id),
    });
    const currentRuns = useMemo(() => (runsQuery.data?.items || []).filter((run) => isCurrentLockRevision(run.lockRevision, detail.lock?.revision)), [runsQuery.data?.items, detail.lock?.revision]);
    const reusableAssetRuns = currentRuns.filter((run) => run.status === "COMPLETED" && run.assetMode === assetMode);
    useEffect(() => {
        if (!reusableAssetRuns.length) {
            setBaseAssetRunId("");
            setReuseAssets(false);
            return;
        }
        if (!reusableAssetRuns.some((run) => run.id === baseAssetRunId)) setBaseAssetRunId(reusableAssetRuns[0].id);
    }, [baseAssetRunId, reusableAssetRuns]);
    useEffect(() => {
        if (!selectedRunId && currentRuns[0]) setSelectedRunId(currentRuns[0].id);
        if (selectedRunId && !currentRuns.some((run) => run.id === selectedRunId) && currentRuns[0]) setSelectedRunId(currentRuns[0].id);
    }, [currentRuns, selectedRunId]);
    useEffect(() => {
        if (!scriptRunId && completedScripts[0]) setScriptRunId(completedScripts[0].id);
    }, [completedScripts, scriptRunId]);

    const runQuery = useQuery({
        queryKey: ["manju", "asset-run", detail.project.id, selectedRunId],
        queryFn: () => getManjuAssetRun(detail.project.id, selectedRunId),
        enabled: Boolean(selectedRunId),
        refetchInterval: (query) => (query.state.data?.item.run.status === "ACTIVE" ? 2500 : false),
    });
    const assetRun = currentRuns.some((run) => run.id === runQuery.data?.item.run.id) ? runQuery.data?.item : undefined;
    const activeRun = currentRuns.find((run) => run.status === "ACTIVE");
    const hasActiveRun = Boolean(activeRun);
    const sameModeActive = activeRun?.assetMode === assetMode;

    const applyAsset = (item: ManjuAssetRunDetail) => {
        setSelectedRunId(item.run.id);
        queryClient.setQueryData(["manju", "asset-run", detail.project.id, item.run.id], { item });
        void queryClient.invalidateQueries({ queryKey: ["manju", "asset-runs", detail.project.id] });
        onChanged();
    };

    const createMutation = useMutation({
        mutationFn: () =>
            createManjuAssetRun(detail.project.id, {
                scriptRunId,
                assetMode,
                baseAssetRunId: reuseAssets ? baseAssetRunId : undefined,
                reuseAssets,
            }),
        onSuccess: ({ item }) => {
            applyAsset(item);
            setNotice({ tone: "success", text: item.run.inheritedAssetCount > 0 ? `已建立 P2，并继承 ${item.run.inheritedAssetCount} 个已登记参考图。` : `已按 ${item.run.assetMode} 建立 P2 数字资产任务。` });
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "P2 任务创建失败" }),
    });
    const referenceMutation = useMutation({
        mutationFn: () => registerManjuAssetReference(detail.project.id, selectedRunId, referenceDraft),
        onSuccess: ({ item }) => {
            applyAsset(item);
            setReferenceDraft((current) => ({ ...current, code: "", resourceId: "", url: "", notes: "" }));
            setNotice({ tone: "success", text: "M2 参考图已登记；系统现在才允许对这张资产做图像级核验。" });
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "参考图登记失败" }),
    });

    if (!detail.models?.text.modelKey) {
        return (
            <AssetGate>
                <InlineNotice tone="info">P2 需要项目文本模型。请先在“模型”页选择画布后台已启用的文本模型。</InlineNotice>
            </AssetGate>
        );
    }
    if (!completedScripts.length) {
        return (
            <AssetGate>
                <InlineNotice tone="info">P1 剧本全部阶段完成并锁定后，P2 才会开放。未锁定草稿和小说原文不会被 P2 读取。</InlineNotice>
            </AssetGate>
        );
    }

    const manifestArtifact = assetRun?.artifacts?.find((artifact) => artifact.stageId === assetRun.stages.find((stage) => stage.key === "asset_manifest")?.artifactId);
    const requiredCodes = Array.from(new Set((manifestArtifact?.content || "").match(/\b(?:CHR|SCN|PRP|AUD)-[^\s|,，、:：)\]}]+/g) || [])).sort();
    const readyCodes = new Set((assetRun?.references || []).filter((reference) => reference.status === "READY").map((reference) => reference.code));
    const missingCodes = requiredCodes.filter((code) => !readyCodes.has(code));
    const compileStage = assetRun?.stages.find((stage) => stage.key === "asset_compile");
    const compileArtifact = assetRun?.artifacts?.find((artifact) => artifact.id === compileStage?.artifactId);
    const canvasPackageDisabledReason = !assetRun
        ? "请先建立 P2 资产任务。"
        : assetRun.run.assetMode === "M0"
          ? "M0 只产出文字外观档案，不产出出图提示词。要生成资产图，请在同一份 P1 上新建 M1 资产任务。"
          : !compileArtifact
            ? "请先生成 M0/M1/M2 资产交付稿，导入按钮才会开放。"
            : "";

    return (
        <section className="manju-asset-workspace" aria-label="P2 数字资产工作区">
            <header className="manju-asset-head">
                <div>
                    <span className="manju-kicker">VisiDeep AI · V7.4.26 · P2 数字资产</span>
                    <h3>{assetRun ? `资产任务 ${assetRun.run.id.slice(0, 8)}` : "从锁定 P1 建立资产任务"}</h3>
                    <p>先提取实体，再确认完整资产清单，最后按素材形态编译交付。M0/M1 不需要上传图片。</p>
                </div>
                {assetRun ? (
                    <span className={`manju-truth-badge is-${assetRun.run.status.toLowerCase()}`}>{assetRun.run.status === "COMPLETED" ? "P2 已完成" : assetRun.run.status === "SUPERSEDED" ? "历史版本" : `当前 ${assetRun.run.assetMode}`}</span>
                ) : null}
            </header>

            <div className="manju-asset-create">
                <label>
                    <span>读取哪个已完成 P1</span>
                    <Select
                        value={scriptRunId}
                        options={completedScripts.map((run) => ({ value: run.id, label: `${run.unitTitle || SCRIPT_BRANCH_LABELS[run.branch]} · ${run.sourceRangeLabel || "未登记范围"} · 锁定 ${run.lockRevision}` }))}
                        onChange={setScriptRunId}
                    />
                </label>
                {reusableAssetRuns.length ? (
                    <label>
                        <span>继承哪个已完成资产库</span>
                        <Select
                            value={baseAssetRunId || undefined}
                            options={reusableAssetRuns.map((run) => {
                                const script = completedScripts.find((item) => item.id === run.scriptRunId);
                                return {
                                    value: run.id,
                                    label: `${script?.unitTitle || run.scriptRunId.slice(0, 8)} · ${run.assetMode}${run.inheritedAssetCount ? ` · 已含 ${run.inheritedAssetCount} 张参考图` : ""}`,
                                };
                            })}
                            onChange={setBaseAssetRunId}
                        />
                    </label>
                ) : null}
                <label>
                    <span>本次素材形态</span>
                    <Select value={assetMode} options={ASSET_MODE_OPTIONS.map((item) => ({ value: item.value, label: item.label }))} onChange={setAssetMode} />
                </label>
                <label className="manju-asset-reuse">
                    <Checkbox checked={reuseAssets} disabled={!reusableAssetRuns.length} onChange={(event) => setReuseAssets(event.target.checked)}>
                        继承已完成资产编号
                    </Checkbox>
                    <small>同编号沿用已有外观；本章只补充新增或变化实体，M2 参考图自动带入。</small>
                </label>
                <Button type="primary" icon={<Play />} disabled={!canEdit || !scriptRunId || sameModeActive} loading={createMutation.isPending} onClick={() => createMutation.mutate()}>
                    {sameModeActive ? `${assetMode} 任务进行中` : hasActiveRun ? `切换为 ${assetMode} 并新建` : "开始 P2"}
                </Button>
            </div>
            <p className="manju-asset-mode-help">{ASSET_MODE_OPTIONS.find((item) => item.value === assetMode)?.description}</p>
            {activeRun && !sameModeActive ? (
                <InlineNotice tone="info">
                    新建 {assetMode} 会停用当前未完成的 {activeRun.assetMode} P2 任务；旧工件和历史记录保留，不会删除。
                </InlineNotice>
            ) : null}

            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {runsQuery.isError || runQuery.isError ? <InlineNotice tone="error">P2 状态读取失败，请刷新后重试。</InlineNotice> : null}

            {assetRun ? (
                <>
                    <div className="manju-asset-runbar">
                        <div>
                            <span>绑定 P1</span>
                            <strong>
                                {assetRun.scriptRun.unitTitle || SCRIPT_BRANCH_LABELS[assetRun.scriptRun.branch]} · {assetRun.scriptRun.sourceRangeLabel || assetRun.scriptRun.id.slice(0, 8)}
                            </strong>
                            <small>
                                P0 revision {assetRun.run.lockRevision} · 素材形态 {assetRun.run.assetMode}
                                {assetRun.run.reuseAssets ? ` · 继承 ${assetRun.run.baseAssetRunId?.slice(0, 8)} / ${assetRun.run.inheritedAssetCount} 张参考图` : ""}
                            </small>
                        </div>
                        <div>
                            <span>当前阶段</span>
                            <strong>{assetRun.stages.find((stage) => stage.key === assetRun.run.currentStage)?.title || "已完成"}</strong>
                            <small>{assetRun.run.status === "COMPLETED" ? "P2 已锁定，可供 P3 读取" : assetRun.run.status === "SUPERSEDED" ? "已被新版本替代，只保留历史查看" : "阶段完成按门禁推进"}</small>
                        </div>
                        <div className="manju-runbar-actions">
                            <Button icon={<RefreshCw />} loading={runQuery.isFetching} onClick={() => void runQuery.refetch()}>
                                刷新状态
                            </Button>
                        </div>
                    </div>

                    <ManjuCanvasPackagePanel projectId={detail.project.id} projectName={detail.project.name} stage="p2_assets" runId={assetRun.run.id} canEdit={canEdit} disabledReason={canvasPackageDisabledReason} onNotice={setNotice} />
                    {assetRun.run.assetMode !== "M0" && compileArtifact ? <AssetGenerationScriptTable projectId={detail.project.id} runId={assetRun.run.id} /> : null}

                    {assetRun.run.assetMode === "M2" ? (
                        <section className="manju-asset-reference">
                            <header>
                                <div>
                                    <span className="manju-kicker">仅 M2</span>
                                    <strong>用户交回图片后再登记</strong>
                                    <small>M0/M1 页面没有上传框，也不会因为缺图阻断 P3。</small>
                                </div>
                                <code>
                                    {readyCodes.size}/{requiredCodes.length || "清单确认后"} 已登记
                                </code>
                            </header>
                            <div className="manju-asset-reference-form">
                                <Select value={referenceDraft.assetType} options={["CHR", "SCN", "PRP", "AUD"].map((value) => ({ value, label: value }))} onChange={(value) => setReferenceDraft((current) => ({ ...current, assetType: value }))} />
                                <Input value={referenceDraft.code} placeholder="资产编码，例如 CHR-林澈" onChange={(event) => setReferenceDraft((current) => ({ ...current, code: event.target.value }))} />
                                <Input value={referenceDraft.resourceId} placeholder="资源 ID / 文件标识" onChange={(event) => setReferenceDraft((current) => ({ ...current, resourceId: event.target.value }))} />
                                <Input value={referenceDraft.url} placeholder="对象存储 URL（可选）" onChange={(event) => setReferenceDraft((current) => ({ ...current, url: event.target.value }))} />
                                <Button type="primary" disabled={!canEdit || !referenceDraft.code.trim() || !referenceDraft.resourceId.trim()} loading={referenceMutation.isPending} onClick={() => referenceMutation.mutate()}>
                                    登记参考图
                                </Button>
                            </div>
                            {assetRun.references?.length ? (
                                <div className="manju-asset-reference-list">
                                    {assetRun.references.map((reference) => (
                                        <div key={reference.id}>
                                            <code>{reference.code}</code>
                                            <span>{reference.assetType}</span>
                                            <small>{reference.resourceId}</small>
                                        </div>
                                    ))}
                                </div>
                            ) : null}
                            {missingCodes.length ? <InlineNotice tone="info">编译前还缺这些资产的已登记参考图：{missingCodes.join("、")}。</InlineNotice> : null}
                        </section>
                    ) : null}

                    <div className="manju-asset-stage-stack">
                        {assetRun.stages.map((stage) => (
                            <AssetStageCard
                                key={stage.id}
                                projectId={detail.project.id}
                                detail={assetRun}
                                stage={stage}
                                canEdit={canEdit}
                                compileReady={assetRun.run.assetMode !== "M2" || (requiredCodes.length > 0 && missingCodes.length === 0)}
                                onUpdated={applyAsset}
                            />
                        ))}
                    </div>
                </>
            ) : (
                <div className="manju-script-empty">
                    <Boxes />
                    <strong>尚未建立 P2 数字资产任务</strong>
                    <span>选择一个已完成 P1，确认素材形态后开始实体提取。</span>
                </div>
            )}
        </section>
    );
}

const ASSET_CATEGORY_LABELS: Record<ManjuCanvasTask["category"], string> = {
    character: "角色",
    environment: "场景",
    prop: "道具",
    material: "物料",
    other: "其他",
};

function AssetGenerationScriptTable({ projectId, runId }: { projectId: string; runId: string }) {
    const [keyword, setKeyword] = useState("");
    const [category, setCategory] = useState<ManjuCanvasTask["category"] | "all">("all");
    const [previewTask, setPreviewTask] = useState<ManjuCanvasTask | null>(null);
    const packageQuery = useQuery({
        queryKey: ["manju", "asset-generation-script", projectId, runId],
        queryFn: () => getManjuCanvasPackage(projectId, "p2_assets", runId),
    });
    const tasks = packageQuery.data?.item.tasks || [];
    const categories = useMemo(() => Array.from(new Set(tasks.map((task) => task.category))), [tasks]);
    const filteredTasks = useMemo(() => {
        const normalized = keyword.trim().toLowerCase();
        return tasks.filter((task) => {
            if (category !== "all" && task.category !== category) return false;
            if (!normalized) return true;
            return [task.code, task.assetCode, task.title, task.prompt].filter(Boolean).join(" ").toLowerCase().includes(normalized);
        });
    }, [category, keyword, tasks]);

    return (
        <section className="manju-asset-script-table" aria-label="资产生成脚本表格">
            <header>
                <div>
                    <span className="manju-kicker">P2 · 2.3 资产交付编译</span>
                    <h3>资产生成脚本表</h3>
                    <p>每行是一个画布图片任务；这里展示原版编译提示词，实际编辑和生成在项目画布中完成。</p>
                </div>
                <span className="manju-truth-badge">{tasks.length} 个生成任务</span>
            </header>
            <div className="manju-asset-script-toolbar">
                <Input allowClear value={keyword} placeholder="搜索编号、资产编码、名称或提示词" onChange={(event) => setKeyword(event.target.value)} />
                <Select value={category} onChange={setCategory} options={[{ value: "all", label: "全部类型" }, ...categories.map((value) => ({ value, label: ASSET_CATEGORY_LABELS[value] }))]} />
                <span>
                    {filteredTasks.length}/{tasks.length}
                </span>
            </div>
            {packageQuery.isLoading ? <div className="manju-asset-script-state">正在读取资产生成脚本…</div> : null}
            {packageQuery.isError ? <InlineNotice tone="error">资产生成脚本读取失败，请先完成 2.3 资产交付编译。</InlineNotice> : null}
            {!packageQuery.isLoading && !packageQuery.isError ? (
                filteredTasks.length ? (
                    <div className="manju-asset-script-scroll">
                        <table>
                            <thead>
                                <tr>
                                    <th>编号</th>
                                    <th>资产编码</th>
                                    <th>名称</th>
                                    <th>类型</th>
                                    <th>画幅</th>
                                    <th>生成提示词</th>
                                    <th>操作</th>
                                </tr>
                            </thead>
                            <tbody>
                                {filteredTasks.map((task) => (
                                    <tr key={task.id}>
                                        <td>
                                            <code>{task.code}</code>
                                        </td>
                                        <td>
                                            <code>{task.assetCode || "—"}</code>
                                        </td>
                                        <td title={task.title}>{task.title}</td>
                                        <td>
                                            <span className={`manju-asset-type is-${task.category}`}>{ASSET_CATEGORY_LABELS[task.category]}</span>
                                        </td>
                                        <td>{task.aspectRatio || "16:9"}</td>
                                        <td title={task.prompt}>
                                            <p>{task.prompt}</p>
                                        </td>
                                        <td>
                                            <Button size="small" onClick={() => setPreviewTask(task)}>
                                                查看脚本
                                            </Button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                ) : (
                    <div className="manju-asset-script-state">没有匹配的资产任务。</div>
                )
            ) : null}
            <AppModal open={Boolean(previewTask)} title={previewTask ? `${previewTask.code} · ${previewTask.title}` : "资产生成脚本"} footer={null} onCancel={() => setPreviewTask(null)} width={860}>
                {previewTask ? (
                    <div className="manju-asset-script-preview">
                        <dl>
                            <div>
                                <dt>资产编码</dt>
                                <dd>{previewTask.assetCode || "未提供"}</dd>
                            </div>
                            <div>
                                <dt>类型</dt>
                                <dd>{ASSET_CATEGORY_LABELS[previewTask.category]}</dd>
                            </div>
                            <div>
                                <dt>画幅</dt>
                                <dd>{previewTask.aspectRatio || "16:9"}</dd>
                            </div>
                        </dl>
                        <pre>{previewTask.prompt}</pre>
                    </div>
                ) : null}
            </AppModal>
        </section>
    );
}

function AssetStageCard({ projectId, detail, stage, canEdit, compileReady, onUpdated }: { projectId: string; detail: ManjuAssetRunDetail; stage: ManjuAssetStage; canEdit: boolean; compileReady: boolean; onUpdated: (item: ManjuAssetRunDetail) => void }) {
    const [instruction, setInstruction] = useState("");
    const [notice, setNotice] = useState<{ tone: "error" | "success"; text: string } | null>(null);
    const [manualCorrectionOpen, setManualCorrectionOpen] = useState(false);
    const artifact = detail.artifacts?.find((item) => item.id === stage.artifactId);
    const runMutation = useMutation({
        mutationFn: () => runManjuAssetStage(projectId, detail.run.id, stage.key, { instruction: instruction.trim() }),
        onSuccess: ({ item }) => onUpdated(item),
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "资产阶段执行失败" }),
    });
    const manifestMutation = useMutation({
        mutationFn: () => confirmManjuAssetManifest(projectId, detail.run.id),
        onSuccess: ({ item }) => onUpdated(item),
    });
    const finalMutation = useMutation({
        mutationFn: () => confirmManjuAssetRun(projectId, detail.run.id, "确认 P2 资产交付"),
        onSuccess: ({ item }) => onUpdated(item),
    });
    const regenerateArtifactMutation = useMutation({
        mutationFn: () => regenerateManjuAssetArtifact(projectId, detail.run.id, stage.key, instruction.trim()),
        onSuccess: ({ item }) => {
            setInstruction("");
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "当前工件重新生成失败" }),
    });
    const manualCorrectionMutation = useMutation({
        mutationFn: (content: string) =>
            manualCorrectManjuAssetArtifact(projectId, detail.run.id, stage.key, {
                content,
                note: "用户人工修正当前 P2 工件",
            }),
        onSuccess: ({ item }) => {
            setManualCorrectionOpen(false);
            setNotice({ tone: "success", text: "人工修正已保存，请核对后继续确认。" });
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "人工修正未通过校验" }),
    });
    const isCurrent = detail.run.currentStage === stage.key;
    const runActive = detail.run.status === "ACTIVE";
    const runReadable = runActive || detail.run.status === "COMPLETED";
    const compileBlocked = stage.key === "asset_compile" && detail.run.assetMode === "M2" && !compileReady;
    const canRegenerateArtifact = canEdit && runReadable && Boolean(artifact) && !stage.taskId && ["LOCKED", "BLOCKED", "AWAITING_CONFIRMATION"].includes(stage.status);
    const canRun = canEdit && runActive && isCurrent && ["IN_PROGRESS", "BLOCKED", "STALE"].includes(stage.status) && !stage.taskId && (stage.key !== "asset_compile" || compileReady) && !canRegenerateArtifact;
    const canApproveManifest = canEdit && runActive && stage.key === "asset_manifest" && stage.status === "AWAITING_CONFIRMATION";
    const canApproveFinal = canEdit && runActive && stage.key === "asset_compile" && stage.status === "AWAITING_CONFIRMATION";
    const manualCorrectionContent = stage.key === "asset_compile" && artifact?.content
        ? artifact.content
        : stage.draft?.content || "";
    const canManualCorrect = canEdit && Boolean(manualCorrectionContent) && (
        stage.status === "BLOCKED"
        || (stage.key === "asset_compile" && ["AWAITING_CONFIRMATION", "LOCKED"].includes(stage.status))
    );
    return (
        <article className={`manju-asset-stage is-${stage.status.toLowerCase()}`}>
            <header>
                <span className="manju-asset-stage-index">2{stage.position}</span>
                <div>
                    <strong>{stage.title}</strong>
                    <small>{compileBlocked ? "M2 参考图未齐，资产交付编译尚未开始" : stage.taskId ? `文本任务 ${stage.taskId.slice(0, 8)}` : artifact ? `工件 ${artifact.kind}` : "等待可执行条件"}</small>
                </div>
                <span className={`manju-script-status is-${stage.status.toLowerCase()}`}>{assetStageStatusLabel(stage)}</span>
            </header>
            {stage.error ? <InlineNotice tone="error">{stage.error}</InlineNotice> : null}
            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {canManualCorrect ? (
                <div className="manju-asset-stage-input">
                    <span>
                        {stage.status === "BLOCKED"
                            ? "保留当前失败正文，人工修改后重新运行 P2 官方机检。"
                            : "直接编辑当前已生成工件；保存后重新运行 P2 官方机检，并生成新的工件版本。"}
                    </span>
                    <Button icon={<FileText />} onClick={() => setManualCorrectionOpen(true)}>
                        人工修正当前工件
                    </Button>
                </div>
            ) : null}
            {compileBlocked ? (
                <div className="manju-asset-compile-block">
                    <ImageIcon />
                    <div>
                        <strong>M2 资产交付编译被参考图门禁暂停</strong>
                        <span>清单中的每个实体都登记参考图后，编译按钮才会出现。现在没有提交模型任务，也不会扣除模型成本。</span>
                    </div>
                    <Button onClick={() => document.querySelector(".manju-asset-reference")?.scrollIntoView({ behavior: "smooth", block: "start" })}>去登记参考图</Button>
                </div>
            ) : null}
            {canRun ? (
                <div className="manju-asset-stage-input">
                    <Input.TextArea
                        value={instruction}
                        rows={3}
                        placeholder={
                            stage.key === "entity_extraction"
                                ? "可选：补充实体提取边界，例如只提取 A/B 级角色；不得要求读取小说原文。"
                                : stage.key === "asset_manifest"
                                  ? "可选：补充清单检查重点，例如亲缘推导或场景状态变体。"
                                  : "可选：补充本次编译要求；M0/M1 都不得索要图片。"
                        }
                        onChange={(event) => setInstruction(event.target.value)}
                    />
                    <Button type="primary" icon={<Play />} loading={runMutation.isPending} onClick={() => runMutation.mutate()}>
                        {stage.key === "entity_extraction" ? "执行实体提取" : stage.key === "asset_manifest" ? "生成资产清单" : `生成 ${detail.run.assetMode} 交付稿`}
                    </Button>
                </div>
            ) : null}
            {stage.taskId && stage.status === "IN_PROGRESS" ? (
                <div className="manju-script-running">
                    <RefreshCw />
                    <span>
                        <strong>等待文本模型返回</strong>
                        <small>页面会自动刷新；只有真实成功结果才会生成不可变工件。</small>
                    </span>
                </div>
            ) : null}
            {artifact ? <AssetArtifactPreview artifact={artifact} /> : null}
            {canRegenerateArtifact ? (
                <footer className="manju-asset-confirm">
                    <span>只重新生成当前工件；上游锁定工件和其他阶段保持不变，下游会等待新工件。</span>
                    <div className="manju-asset-stage-input">
                        <Input.TextArea
                            value={instruction}
                            rows={2}
                            placeholder="可选：填写当前工件需要修正的内容。"
                            onChange={(event) => setInstruction(event.target.value)}
                        />
                        <Button icon={<RefreshCw />} loading={regenerateArtifactMutation.isPending} onClick={() => regenerateArtifactMutation.mutate()}>
                            重新生成当前工件
                        </Button>
                    </div>
                </footer>
            ) : null}
            {canApproveManifest ? (
                <footer className="manju-asset-confirm">
                    <span>请确认本清单“无遗漏、无错位”。这一步只确认文字/编号，不要求任何图片。</span>
                    <Button type="primary" icon={<Check />} loading={manifestMutation.isPending} onClick={() => manifestMutation.mutate()}>
                        确认清单并进入编译
                    </Button>
                </footer>
            ) : null}
            {canApproveFinal ? (
                <footer className="manju-asset-confirm">
                    <span>{detail.run.assetMode === "M2" ? "最终确认会把已登记参考图和资产图册锁定为 P3 唯一资产依据。" : "最终确认会锁定文字/编号交付，M0/M1 不因缺图阻塞 P3。"}</span>
                    <Button type="primary" icon={<Check />} loading={finalMutation.isPending} onClick={() => finalMutation.mutate()}>
                        最终确认 P2
                    </Button>
                </footer>
            ) : null}
            {manualCorrectionContent ? (
                <ManjuArtifactEditor
                    open={manualCorrectionOpen}
                    title={`人工修正 ${stage.title}`}
                    content={manualCorrectionContent}
                    validationJson={
                        manualCorrectionMutation.error
                            ? JSON.stringify({
                                  issues: [
                                      manualCorrectionMutation.error instanceof Error
                                          ? manualCorrectionMutation.error.message
                                          : "人工修正未通过校验",
                                  ],
                              })
                            : stage.draft?.validationJson
                    }
                    loading={manualCorrectionMutation.isPending}
                    onCancel={() => setManualCorrectionOpen(false)}
                    onSubmit={(content) => manualCorrectionMutation.mutate(content)}
                />
            ) : null}
        </article>
    );
}

const PRODUCTION_STAGE_STATUS_LABELS: Record<ManjuProductionStageStatus, string> = {
    NOT_STARTED: "等待上游",
    IN_PROGRESS: "待执行 / 执行中",
    AWAITING_CONFIRMATION: "等待人工确认",
    LOCKED: "已确认锁定",
    BLOCKED: "执行失败",
    STALE: "上游已升级，需重新生成",
};

const PRODUCTION_STAGE_META: Record<string, { input: string; output: string; consumer: string; placeholder: string; runLabel: string; icon: LucideIcon }> = {
    decision_anchor: {
        input: "读取已锁定的 P1 剧本动作、P2 资产事实与 P0 模型/画幅合同。",
        output: "EMOTION_ANCHOR_TABLE.md",
        consumer: "P3 只能读取已锁定的六列情绪锚点表与 A/B 构图雷区判定。",
        placeholder: "可选补充：本段情绪节拍、情境 ID 或必须核对的白名单雷区。",
        runLabel: "执行 P0B 决策锚点",
        icon: Sparkles,
    },
    director_storyboard: {
        input: "只读取已锁定的 P2 资产包、P1 剧本事实、P0B 决策锚点与 P0 模型/画幅/段缝合同。",
        output: "DIRECTOR_STORYBOARD.md",
        consumer: "人工确认并写入 project_state 后，P4 由官方 compile_feed.py 读取 segments/shots 生成投喂稿。",
        placeholder: "可选补充：每段动作相位、重点镜头、故事板格数或剧情分镜格要求。",
        runLabel: "执行 P3 分镜与故事板",
        icon: Film,
    },
    feed_package: {
        input: "读取已锁定的 project_state.json 与 P3 segments/shots，由官方 compile_feed.py 封装，再过 validate_prompt.py 与 gate_check.py。",
        output: "FEED_PACKAGE.md",
        consumer: "P4 生产包会导入画布按段生成真实视频；画布任务全部取得资源并锁定后，P5 读取投喂稿、接线单和真实视频。",
        placeholder: "",
        runLabel: "官方编译 P4 投喂稿",
        icon: Video,
    },
    qc_handoff: {
        input: "读取已锁定的 P4 投喂稿、P4 画布真实视频、P0-P2 锁定事实和连续性台账。",
        output: "QC_HANDOFF.md",
        consumer: "主理人/制片逐段看片做最终裁决；文本机检和资源存在性检查不能冒充视觉、声音验收。",
        placeholder: "可选补充：本轮重点复核的连续性、段缝、声音或机检项。",
        runLabel: "生成 P5 质检交付",
        icon: FileCheck2,
    },
};

function productionStageStatusLabel(stage: ManjuProductionStage) {
    if (stage.key === "feed_package" && stage.status === "IN_PROGRESS") return "等待官方编译";
    if (stage.status === "IN_PROGRESS" && stage.taskId) return "文本任务执行中";
    return PRODUCTION_STAGE_STATUS_LABELS[stage.status];
}

function ProductionArtifactPreview({ artifact }: { artifact: ManjuProductionArtifact }) {
    return (
        <details className="manju-script-artifact is-production">
            <summary>
                <FileText />
                <span>生产工件</span>
                <strong>{artifact.kind}</strong>
                <code>v{artifact.version}</code>
                <ChevronRight />
            </summary>
            <pre>{artifact.content}</pre>
        </details>
    );
}

const PRODUCTION_PIPELINE_META: Record<"p3" | "p4" | "p5", { key: string; title: string; description: string; icon: LucideIcon; gate: string }> = {
    p3: {
        key: "director_storyboard",
        title: "P3 分镜与故事板",
        description: "先锁定 P0B 决策锚点，再读取资产生成表演、调度、分镜文字和逐段故事板图片。",
        icon: Film,
        gate: "先完成 P2 资产交付与最终确认，再锁定 P0B 决策锚点。",
    },
    p4: {
        key: "feed_package",
        title: "P4 投喂封装",
        description: "由官方 compile_feed.py 把已锁定 P3 台账编译成七段式投喂稿，再导入画布按段生成真实视频。",
        icon: Video,
        gate: "先完成 P3 分镜文字、故事板图片和人工锁定。",
    },
    p5: {
        key: "qc_handoff",
        title: "P5 质检与交付",
        description: "读取真实视频与 P4 投喂稿，完成机检、连续性和主理人最终看片裁决。",
        icon: FileCheck2,
        gate: "先完成 P4 投喂稿和全部视频资源锁定。",
    },
};

function RuleLibraryStatusBar({ status }: { status: ManjuRuleLibraryStatus }) {
    const state = status.projectState;
    const p4Ready = state?.stageKey === "production:director_storyboard";
    return (
        <div className="manju-rule-runtime">
            <ShieldCheck />
            <div>
                <strong>官方规则库 {status.version} 已内置</strong>
                <span>
                    {status.fileCount} 个原文件 · {status.officialScripts.length} 个官方脚本 ·{state ? `project_state 已建立（${state.stageKey}）` : "project_state 等待阶段回写"}
                    {p4Ready ? " · P4 可编译" : ""}
                </span>
            </div>
            <code>{(state?.contentHash || status.contentHash).slice(0, 12)}</code>
        </div>
    );
}

function ProductionPipeline({ detail, activeStage, onChanged }: { detail: ManjuProjectDetail; activeStage: "p3" | "p4" | "p5"; onChanged: () => void }) {
    const queryClient = useQueryClient();
    const canEdit = ["owner", "producer", "director"].includes(detail.accessRole);
    const [selectedAssetRunId, setSelectedAssetRunId] = useState("");
    const [selectedProductionRunId, setSelectedProductionRunId] = useState("");
    const [notice, setNotice] = useState<{ tone: "info" | "error" | "success"; text: string } | null>(null);
    const assetRunsQuery = useQuery({
        queryKey: ["manju", "asset-runs", detail.project.id],
        queryFn: () => listManjuAssetRuns(detail.project.id),
    });
    const productionRunsQuery = useQuery({
        queryKey: ["manju", "production-runs", detail.project.id],
        queryFn: () => listManjuProductionRuns(detail.project.id),
    });
    const completedAssetRuns = useMemo(() => (assetRunsQuery.data?.items || []).filter((run) => run.status === "COMPLETED" && isCurrentLockRevision(run.lockRevision, detail.lock?.revision)), [assetRunsQuery.data?.items, detail.lock?.revision]);
    const ruleLibraryQuery = useQuery({
        queryKey: ["manju", "rule-library", detail.project.id],
        queryFn: () => getManjuRuleLibrary(detail.project.id),
    });
    const productionRuns = useMemo(() => (productionRunsQuery.data?.items || []).filter((run) => isCurrentLockRevision(run.lockRevision, detail.lock?.revision)), [productionRunsQuery.data?.items, detail.lock?.revision]);
    const completedProductionRuns = useMemo(() => productionRuns.filter((run) => run.status === "COMPLETED"), [productionRuns]);
    const selectableProductionRuns = useMemo(
        () => productionRuns.filter((run) => run.status === "ACTIVE" || completedProductionRuns.some((completed) => completed.id === run.id)),
        [completedProductionRuns, productionRuns],
    );
    const hasActiveProduction = productionRuns.some((run) => run.status === "ACTIVE");

    useEffect(() => {
        if (!selectedAssetRunId && completedAssetRuns[0]) setSelectedAssetRunId(completedAssetRuns[0].id);
        if (selectedAssetRunId && !completedAssetRuns.some((run) => run.id === selectedAssetRunId) && completedAssetRuns[0]) {
            setSelectedAssetRunId(completedAssetRuns[0].id);
        }
    }, [completedAssetRuns, selectedAssetRunId]);
    useEffect(() => {
        if (!selectedProductionRunId && productionRuns[0]) setSelectedProductionRunId(productionRuns[0].id);
        if (selectedProductionRunId && !productionRuns.some((run) => run.id === selectedProductionRunId) && productionRuns[0]) {
            setSelectedProductionRunId(productionRuns[0].id);
        }
    }, [productionRuns, selectedProductionRunId]);

    const productionRunQuery = useQuery({
        queryKey: ["manju", "production-run", detail.project.id, selectedProductionRunId],
        queryFn: () => getManjuProductionRun(detail.project.id, selectedProductionRunId),
        enabled: Boolean(selectedProductionRunId),
        refetchInterval: (query) => (query.state.data?.item.run.status === "ACTIVE" ? 2500 : false),
    });
    const productionRun = productionRuns.some((run) => run.id === productionRunQuery.data?.item.run.id) ? productionRunQuery.data?.item : undefined;

    const applyProduction = (item: ManjuProductionRunDetail) => {
        setSelectedProductionRunId(item.run.id);
        queryClient.setQueryData(["manju", "production-run", detail.project.id, item.run.id], { item });
        void queryClient.invalidateQueries({ queryKey: ["manju", "production-runs", detail.project.id] });
        onChanged();
    };

    const createMutation = useMutation({
        mutationFn: () => createManjuProductionRun(detail.project.id, { assetRunId: selectedAssetRunId }),
        onSuccess: ({ item }) => {
            applyProduction(item);
            setNotice({ tone: "success", text: "已建立 P0B-P5 生产链，先完成 P0B 决策锚点。" });
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "生产链创建失败" }),
    });

    const currentStage = productionRun?.stages.find((stage) => stage.key === productionRun.run.currentStage);
    const runBadge = productionRun?.run.status === "COMPLETED"
        ? "生产链已完成"
        : productionRun?.run.status === "SUPERSEDED"
          ? "历史版本"
          : productionRun
            ? "生产链进行中"
            : "等待已完成 P2";
    const activeMeta = PRODUCTION_PIPELINE_META[activeStage];
    const productionStagesForTab = productionRun
        ? activeStage === "p3"
            ? productionRun.stages.filter((stage) => stage.key === "decision_anchor" || stage.key === activeMeta.key)
            : productionRun.stages.filter((stage) => stage.key === activeMeta.key)
        : [];
    const ActiveMetaIcon = activeMeta.icon;

    return (
        <section className="manju-production-workspace" aria-label="P0B-P5 生产链工作区">
            <header className="manju-production-head">
                <div>
                    <span className="manju-kicker">VisiDeep AI · V7.4.26 · {activeMeta.title}</span>
                    <h3>{activeMeta.title}</h3>
                    <p>{activeMeta.description}</p>
                </div>
                <span className={`manju-truth-badge${productionRun?.run.status === "COMPLETED" ? " is-completed" : ""}`}>{runBadge}</span>
            </header>

            <InlineNotice tone="info">当前只显示 {activeStage === "p3" ? "P0B 与 P3" : activeMeta.title}。阶段未确认锁定前，下一阶段不会读取未确认成果。</InlineNotice>
            {ruleLibraryQuery.data?.item ? <RuleLibraryStatusBar status={ruleLibraryQuery.data.item} /> : null}
            {!detail.models?.text.modelKey ? <InlineNotice tone="info">P0B、P3 与 P5 需要项目文本模型；P4 使用官方 compile_feed.py，不调用文本模型。请先在“模型”页选择文本模型。</InlineNotice> : null}
            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {assetRunsQuery.isError || productionRunsQuery.isError || productionRunQuery.isError ? <InlineNotice tone="error">生产链状态读取失败，请刷新后重试。</InlineNotice> : null}
            {activeStage === "p5" && selectableProductionRuns.length > 1 ? (
                <section className="manju-production-run-selector" aria-label="历史生产链选择">
                    <div>
                        <span>历史生产链</span>
                        <small>切换只读查看相同 P0 revision 下已完成的旧生产链。</small>
                    </div>
                    <Select
                        aria-label="选择生产链版本"
                        value={selectedProductionRunId}
                        options={selectableProductionRuns.map((run) => ({
                            value: run.id,
                            label: `${run.status === "COMPLETED" ? "已完成" : "进行中"} · ${run.id.slice(0, 8)} · P0 revision ${run.lockRevision}`,
                        }))}
                        onChange={(runId) => setSelectedProductionRunId(runId)}
                    />
                </section>
            ) : null}

            {!completedAssetRuns.length ? (
                <section className="manju-production-gate">
                    <LockKeyhole />
                    <div>
                        <strong>P3 还不能开始</strong>
                        <span>先完成 P1 剧本、P2 实体提取、清单确认、资产编译与最终确认。未锁定的 P2 不会开放执行按钮。</span>
                    </div>
                </section>
            ) : !productionRuns.length && activeStage === "p3" ? (
                <div className="manju-production-create">
                    <label>
                        <span>读取哪个已锁定 P2</span>
                        <Select value={selectedAssetRunId} options={completedAssetRuns.map((run) => ({ value: run.id, label: `${run.assetMode} · ${run.id.slice(0, 8)} · P0 revision ${run.lockRevision}` }))} onChange={setSelectedAssetRunId} />
                    </label>
                    <Button type="primary" icon={<Play />} disabled={!canEdit || !selectedAssetRunId || !detail.models?.text.modelKey || hasActiveProduction} loading={createMutation.isPending} onClick={() => createMutation.mutate()}>
                        建立 P0B-P5 生产链
                    </Button>
                </div>
            ) : !productionRuns.length ? (
                <section className="manju-production-gate">
                    <LockKeyhole />
                    <div>
                        <strong>{activeMeta.title} 还不能开始</strong>
                        <span>{activeMeta.gate}</span>
                    </div>
                </section>
            ) : null}

            {productionRun ? (
                <>
                    <div className="manju-production-runbar">
                        <div>
                            <span>绑定 P2</span>
                            <strong>
                                {productionRun.scriptRun.unitTitle || "未命名改编单元"} · {productionRun.assetRun.assetMode}
                            </strong>
                            <small>
                                {productionRun.scriptRun.sourceRangeLabel || "未登记范围"} · P0 revision {productionRun.run.lockRevision} ·{" "}
                                {currentStage?.key === "feed_package" ? "官方 compile_feed.py + 双机检" : `文本模型 ${detail.models?.text.modelKey || "未绑定"}`}
                            </small>
                        </div>
                        <div>
                            <span>当前阶段</span>
                            <strong>{productionRun.run.status === "COMPLETED" ? "P0B-P5 已完成" : productionRun.run.status === "SUPERSEDED" ? "历史版本" : currentStage?.title || "等待阶段"}</strong>
                            <small>{productionRun.run.status === "COMPLETED" ? "最终人工确认已保存，输入与工件均已锁定。" : productionRun.run.status === "SUPERSEDED" ? "该版本已保留，只能查看历史工件和媒体。" : "当前阶段未确认前，下一阶段不能读取。"}</small>
                        </div>
                        <Button icon={<RefreshCw />} loading={productionRunQuery.isFetching} onClick={() => void productionRunQuery.refetch()}>
                            刷新生产链
                        </Button>
                    </div>
                    <div className="manju-production-stage-stack">
                        {productionStagesForTab.map((stage) => (
                            <ProductionStageCard key={stage.id} projectId={detail.project.id} projectName={detail.project.name} detail={productionRun} stage={stage} canEdit={canEdit} onUpdated={applyProduction} />
                        ))}
                    </div>
                </>
            ) : null}
            {!productionRun ? (
                <div className="manju-production-preview-grid is-single" aria-label={`${activeMeta.title} 阶段合同预览`}>
                    <article>
                        <ActiveMetaIcon />
                        <strong>{activeMeta.title}</strong>
                        <span>{activeMeta.description}</span>
                        <span>{activeMeta.gate}</span>
                        <code>{PRODUCTION_STAGE_META[activeMeta.key]?.output}</code>
                    </article>
                </div>
            ) : null}
        </section>
    );
}

function ProductionStageCard({
    projectId,
    projectName,
    detail,
    stage,
    canEdit,
    onUpdated,
}: {
    projectId: string;
    projectName: string;
    detail: ManjuProductionRunDetail;
    stage: ManjuProductionStage;
    canEdit: boolean;
    onUpdated: (item: ManjuProductionRunDetail) => void;
}) {
    const navigate = useNavigate();
    const meta = PRODUCTION_STAGE_META[stage.key];
    const Icon = meta?.icon || Layers3;
    const artifact = detail.artifacts?.find((item) => item.id === stage.artifactId);
    const previs = detail.previs;
    const previsArtifact = detail.artifacts?.find((item) => item.kind === "PREVIS_STORYBOARD.md");
    const previsReady = previs?.status === "READY" && Boolean(previs.artifactId) && Boolean(previsArtifact);
    const previsLive = previs?.status === "QUEUED" || previs?.status === "RUNNING";
    const readyStoryboardImageCount = (detail.images || []).filter((image) => image.status === "READY" && Boolean(image.resourceId)).length;
    const sourceP3ArtifactId = detail.stages.find((item) => item.key === "director_storyboard")?.artifactId || "";
    const isOfficialCompiler = stage.key === "feed_package";
    const legacyNonCompliant = stage.machineCheckStatus === "LEGACY_NON_COMPLIANT";
    const machineCheckPassed = stage.machineCheckStatus === "PASS" && stage.machineCheckProofVerified === true;
    const machineCheckProofMissing = isOfficialCompiler && Boolean(artifact) && !legacyNonCompliant && !machineCheckPassed;
    const machineCheckWarnings = stage.machineCheckWarnings || [];
    const officialAuditLabel = isOfficialCompiler
        ? artifact
            ? legacyNonCompliant
                ? stage.machineCheckNote || "历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。"
                : machineCheckProofMissing
                  ? "未记录完整四步机检，不能作为 PASS；请重新执行官方 P4 编译链"
                : machineCheckWarnings.length > 0
                  ? "compile_feed.py + validate_prompt.py + gate_check.py + script_feed_diff.py 已通过；台词完整性存在需人工核对的部分覆盖警告"
                  : "compile_feed.py + validate_prompt.py + gate_check.py + script_feed_diff.py 已通过"
            : stage.error
              ? "官方 P4 完整机检链未通过"
              : "等待官方 P4 完整机检链"
        : stage.key === "qc_handoff"
          ? artifact
              ? "audit_contract.py 全流程契约审计已通过"
              : stage.error
                ? "官方全流程契约审计未通过"
                : "等待官方全流程契约审计"
          : "";
    const officialAuditTone = artifact
        ? legacyNonCompliant || machineCheckProofMissing
            ? "is-failed"
            : machineCheckWarnings.length > 0
              ? "is-warning"
              : "is-passed"
        : stage.error
          ? "is-failed"
          : "is-pending";
    const [instruction, setInstruction] = useState("");
    const [notice, setNotice] = useState<{ tone: "info" | "error" | "success"; text: string } | null>(null);
    const [regenerateOpen, setRegenerateOpen] = useState(false);
    const [regenerateInstruction, setRegenerateInstruction] = useState("");
    const [manualCorrectionOpen, setManualCorrectionOpen] = useState(false);
    const [compilePreview, setCompilePreview] = useState<ManjuCompilePreview | undefined>();
    const canvasImportsQuery = useQuery({
        queryKey: ["manju", "canvas-imports", projectId],
        queryFn: () => listManjuCanvasImports(projectId),
        enabled: stage.key === "qc_handoff",
    });
    const deliverablesQuery = useQuery({
        queryKey: ["manju", "deliverables", projectId, detail.run.id],
        queryFn: () => getManjuProductionRunDeliverables(projectId, detail.run.id),
        enabled: stage.key === "qc_handoff",
    });
    const deliverables = deliverablesQuery.data?.items || [];
    const projectCanvasId = primaryManjuProjectCanvas(canvasImportsQuery.data?.items || [])?.canvasId || "";
    const runMutation = useMutation({
        mutationFn: () => runManjuProductionStage(projectId, detail.run.id, stage.key, {
            instruction: instruction.trim(),
        }),
        onSuccess: ({ item }) => {
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "阶段执行失败" }),
    });
    const confirmMutation = useMutation({
        mutationFn: () => confirmManjuProductionStage(projectId, detail.run.id, stage.key, `确认 ${stage.title}`),
        onSuccess: ({ item }) => {
            onUpdated(item);
            if (stage.key === "qc_handoff") void deliverablesQuery.refetch();
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "阶段锁定失败" }),
    });
    const regenerateMutation = useMutation({
        mutationFn: () => regenerateManjuProductionArtifact(projectId, detail.run.id, stage.key, regenerateInstruction.trim()),
        onSuccess: ({ item }) => {
            setRegenerateOpen(false);
            setRegenerateInstruction("");
            onUpdated(item);
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "阶段重新生成失败" }),
    });
    const manualCorrectionMutation = useMutation({
        mutationFn: (content: string) =>
            manualCorrectManjuProductionArtifact(projectId, detail.run.id, stage.key, {
                content,
                note: "用户人工修正当前 P3 工件",
            }),
        onSuccess: ({ item }) => {
            setManualCorrectionOpen(false);
            setNotice({ tone: "success", text: "人工修正已通过官方机检，请核对后确认锁定。" });
            onUpdated(item);
        },
        onError: (error) => {
            setNotice({ tone: "error", text: error instanceof Error ? error.message : "人工修正未通过官方机检" });
        },
    });
    const compilePreviewMutation = useMutation({
        mutationFn: (content: string) =>
            previewManjuProductionStoryboardCompile(projectId, detail.run.id, content),
        onSuccess: ({ item }) => setCompilePreview(item),
        onError: (error) =>
            setNotice({ tone: "error", text: error instanceof Error ? error.message : "编译预览失败" }),
    });
    const previsMutation = useMutation({
        mutationFn: () => generateManjuProductionPrevis(projectId, detail.run.id),
        onSuccess: ({ item }) => onUpdated(item),
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "P4.5 预演故事板生成失败" }),
    });
    const isCurrent = detail.run.currentStage === stage.key;
    const runActive = detail.run.status === "ACTIVE";
    const runReadable = runActive || detail.run.status === "COMPLETED";
    const decisionAnchorLocked = detail.stages.find((item) => item.key === "decision_anchor")?.status === "LOCKED";
    const decisionGateBlocked = stage.key === "director_storyboard" && !decisionAnchorLocked;
    const canConfirm = canEdit && runActive && isCurrent && stage.status === "AWAITING_CONFIRMATION" && Boolean(artifact) && (stage.key !== "feed_package" || previsReady);
    const canGeneratePrevis = canEdit && runActive && stage.key === "feed_package" && Boolean(artifact) && !previsReady && !previsLive;
    const hasLiveWork = detail.stages.some((item) => item.status === "IN_PROGRESS" && Boolean(item.taskId)) ||
        previsLive ||
        (detail.images || []).some((item) => item.status === "QUEUED" || item.status === "RUNNING") ||
        (detail.videos || []).some((item) => item.status === "QUEUED" || item.status === "RUNNING");
    const canRegenerate =
        canEdit &&
        runReadable &&
        !hasLiveWork &&
        Boolean(artifact) &&
        ["decision_anchor", "director_storyboard", "feed_package"].includes(stage.key) &&
        ["AWAITING_CONFIRMATION", "LOCKED", "BLOCKED", "STALE"].includes(stage.status) &&
        (stage.key === "decision_anchor" || decisionAnchorLocked);
    const canManualCorrect =
        canEdit &&
        stage.key === "director_storyboard" &&
        stage.status === "BLOCKED" &&
        Boolean(stage.draft?.content);
    const canRun = canEdit && runActive && isCurrent && !decisionGateBlocked && !stage.taskId && ["IN_PROGRESS", "BLOCKED"].includes(stage.status) && !canRegenerate;
    const canvasPackageStage: ManjuCanvasPackageStage | null = stage.key === "director_storyboard" ? "p3_images" : stage.key === "feed_package" || stage.key === "qc_handoff" ? "p4_videos" : null;
    const canvasSourceStages: ManjuCanvasPackageStage[] = canvasPackageStage === "p3_images" ? ["p2_assets"] : canvasPackageStage === "p4_videos" ? ["p2_assets", "p3_images"] : [];

    return (
        <article className={`manju-production-stage is-${stage.status.toLowerCase()}`}>
            <header>
                <span className="manju-production-stage-index">{stage.key === "decision_anchor" ? "P0B" : `P${stage.position + 1}`}</span>
                <span className="manju-production-stage-icon">
                    <Icon />
                </span>
                <div>
                    <strong>{stage.title}</strong>
                    <small>{isOfficialCompiler ? (artifact ? `官方编译工件 ${artifact.kind}` : "等待官方 compile_feed.py 编译") : stage.taskId ? `文本任务 ${stage.taskId.slice(0, 8)}` : artifact ? `输出工件 ${artifact.kind}` : "尚未提交文本模型"}</small>
                </div>
                <span className={`manju-script-status is-${stage.status.toLowerCase()}`}>{productionStageStatusLabel(stage)}</span>
            </header>

            <dl className="manju-production-contract">
                <div>
                    <dt>输入</dt>
                    <dd>{meta?.input}</dd>
                </div>
                <div>
                    <dt>输出</dt>
                    <dd>
                        <code>{meta?.output}</code>
                    </dd>
                </div>
                <div>
                    <dt>下游读取</dt>
                    <dd>{meta?.consumer}</dd>
                </div>
            </dl>

            {stage.key === "director_storyboard" ? (
                <div className="manju-production-schema">
                    <strong>故事板表合同</strong>
                    <span>分镜总表：镜号 / 整数秒 / 景别 / 焦段 / 机位 / 运镜 / 画面 / 台词 / 音效 / 资产引用</span>
                    <span>故事板表：段 / 格 / 动作相位 / 站位+视线 / 资产引用</span>
                    <span>P3 交付清单：逐段标明故事板已就绪状态或合法豁免理由；逐场剧情分镜格或豁免理由。</span>
                    <span>P3 必须逐段完成故事板交付。未完成图片任务的段落只有在《P3 交付清单》中按 V7.4.26 写明合法豁免时才可锁定。</span>
                    <span>普通“无需图片”不能绕过。当前已就绪 {readyStoryboardImageCount} 张故事板图。</span>
                </div>
            ) : null}
            {stage.key === "feed_package" ? (
                <div className="manju-production-schema">
                    <strong>P4 投喂封装合同</strong>
                    <span>由官方 compile_feed.py 读取 project_state.json，自动展开角色、声线锁、站位、分镜和末镜定格。</span>
                    <span>逐段投喂块 + 素材接线单 + 台词完整性 + 六道闸门。</span>
                    <span>M0 不出现图号；M1/M2 的图号必须与素材接线单双向一致。</span>
                    <span>{readyStoryboardImageCount > 0 ? `读取 ${readyStoryboardImageCount} 张已就绪故事板图作为视觉参考。` : "当前为文字故事板模式：P4 仅读取文字分镜、动作相位和站位，不声称使用故事板图片。"}</span>
                    <span>投喂稿确认后，导入画布按段生成视频；P4.5 必须通过 check_p45.py，且所有画布任务取得真实资源后才能锁定 P4。</span>
                </div>
            ) : null}
            {stage.key === "feed_package" ? (
                <div className="manju-production-schema">
                    <strong>P4.5 预演故事板</strong>
                    <span>独立文本附件，只出故事板提示词，不生成图片，不进入视频投喂稿。</span>
                    <span>
                        {previsReady
                            ? "check_p45.py 已通过，P4.5 工件已就绪。"
                            : previsLive
                              ? "P4.5 正在由项目文本模型生成；页面会自动刷新。"
                              : previs?.status === "FAILED"
                                ? "P4.5 未通过机检，请按错误修正后重新生成。"
                                : "P4.5 尚未生成，P4 不能锁定。"}
                    </span>
                    {previs?.error ? <InlineNotice tone="error">{previs.error}</InlineNotice> : null}
                    {canGeneratePrevis ? (
                        <Button
                            icon={<FileText />}
                            loading={previsMutation.isPending}
                            onClick={() => previsMutation.mutate()}
                        >
                            {previs?.status === "FAILED" ? "重新生成 P4.5 预演故事板" : "生成 P4.5 预演故事板"}
                        </Button>
                    ) : null}
                </div>
            ) : null}
            {stage.key === "qc_handoff" ? (
                <div className="manju-production-schema is-qc">
                    <strong>视频交付与人工看片边界</strong>
                    <span>核对已锁定的真实视频 resourceId，不把资源存在性检查冒充画面或声音验收。</span>
                    <span>逐段状态标记“待主理人看片”，最终由主理人验收成片。</span>
                    <span>FAIL / WARN / PASS 必须分项说明；发现问题只路由回责任阶段。</span>
                </div>
            ) : null}

            {isOfficialCompiler || stage.key === "qc_handoff" ? (
                <div className={`manju-production-audit ${officialAuditTone}`}>
                    <ShieldCheck />
                    <div>
                        <strong>官方机检</strong>
                        <span>{officialAuditLabel}</span>
                        {machineCheckWarnings.map((warning) => <span key={warning}>{warning}</span>)}
                    </div>
                </div>
            ) : null}

            {stage.error ? <InlineNotice tone="error">{stage.error}</InlineNotice> : null}
            {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            {decisionGateBlocked ? (
                <div className="manju-production-stage-input">
                    <span>P0B 锁定后才能执行 P3 分镜。</span>
                    <Button disabled icon={<LockKeyhole />}>执行 P3 分镜与故事板</Button>
                </div>
            ) : null}
            {canManualCorrect ? (
                <div className="manju-production-stage-input">
                    <span>保留当前失败正文，人工修改后重新运行同一官方机检。</span>
                    <Button
                        icon={<FileText />}
                        onClick={() => {
                            setCompilePreview(undefined);
                            setManualCorrectionOpen(true);
                            if (stage.draft?.content) compilePreviewMutation.mutate(stage.draft.content);
                        }}
                    >
                        人工修正当前工件
                    </Button>
                </div>
            ) : null}
            {canRun ? (
                <div className={`manju-production-stage-input${isOfficialCompiler ? " is-compiler" : ""}`}>
                    {isOfficialCompiler ? null : <Input.TextArea rows={3} value={instruction} placeholder={meta?.placeholder} onChange={(event) => setInstruction(event.target.value)} />}
                    <Button type="primary" icon={<Play />} loading={runMutation.isPending} onClick={() => runMutation.mutate()}>
                        {meta?.runLabel}
                    </Button>
                </div>
            ) : null}
            {stage.taskId && !isOfficialCompiler && stage.status === "IN_PROGRESS" ? (
                <div className="manju-script-running">
                    <RefreshCw />
                    <span>
                        <strong>等待文本模型返回</strong>
                        <small>页面会自动刷新；只有真实成功结果才会生成不可变工件。</small>
                    </span>
                </div>
            ) : null}
            {artifact ? <ProductionArtifactPreview artifact={artifact} /> : null}
            {stage.key === "feed_package" && previsArtifact ? <ProductionArtifactPreview artifact={previsArtifact} /> : null}
            {canRegenerate ? (
                <footer className="manju-production-regenerate">
                    <span>只重新生成当前阶段工件；上游锁定工件、其他阶段和历史版本保持不变。</span>
                    <Button
                        icon={<RefreshCw />}
                        onClick={() => {
                            setRegenerateInstruction("");
                            setRegenerateOpen(true);
                        }}
                    >
                        {stage.key === "decision_anchor" ? "重新生成 P0B 工件" : `重新生成 ${stage.key === "director_storyboard" ? "P3" : "P4"} 工件`}
                    </Button>
                </footer>
            ) : null}
            {canvasPackageStage && artifact ? (
                <ManjuCanvasPackagePanel
                    projectId={projectId}
                    projectName={projectName}
                    stage={canvasPackageStage}
                    runId={detail.run.id}
                    sourceStages={canvasSourceStages}
                    requiredSourceStages={canvasPackageStage ? ["p2_assets"] : []}
                    sourceContext={canvasPackageStage === "p4_videos" ? {
                        productionRunId: detail.run.id,
                        p3ArtifactId: sourceP3ArtifactId,
                        storyboardImages: detail.images || [],
                    } : undefined}
                    canEdit={canEdit}
                    disabledReason={stage.key === "qc_handoff" && canvasPackageStage === "p4_videos" ? "P5 使用已锁定的 P4 视频画布进行主理人看片；这里只读取，不新建生成任务。" : undefined}
                    onNotice={setNotice}
                />
            ) : null}
            {canConfirm ? (
                <footer className="manju-production-confirm">
                    <span>
                        {stage.key === "qc_handoff"
                            ? "请逐段查看真实视频，并核对质检边界、FAIL/WARN 项和最终裁决。文本机检通过不等于成片通过。"
                            : stage.key === "decision_anchor"
                              ? "核对六列情绪锚点表、3-5 个节拍、E01-E15/EXnn 情境 ID，以及 A/B 构图雷区判定。锁定后才会开放 P3。"
                              : stage.key === "director_storyboard"
                              ? "P3 必须逐段完成故事板交付。未完成图片任务的段落只有在《P3 交付清单》中按 V7.4.26 写明合法豁免时才可锁定，普通“无需图片”不能绕过。"
                              : stage.key === "feed_package"
                                ? "P4 投喂稿、真实视频和已通过 check_p45.py 的 P4.5 预演故事板会一起锁定，再开放 P5。"
                                : "请人工核对本阶段输出；确认后工件锁定，下一阶段才能读取。"}
                    </span>
                    <Button type="primary" icon={<Check />} loading={confirmMutation.isPending} onClick={() => confirmMutation.mutate()}>
                        {stage.key === "qc_handoff" ? "人工裁决并完成" : "确认并锁定，开放下一阶段"}
                    </Button>
                </footer>
            ) : null}
            {stage.status === "LOCKED" ? (
                <footer className="manju-production-locked">
                    <ShieldCheck />
                    <span>
                        已由 {stage.confirmedBy?.slice(0, 8) || "项目成员"} 于 {stage.confirmedAt ? formatDate(stage.confirmedAt) : "此前"} 确认锁定。
                    </span>
                </footer>
            ) : null}
            {stage.key === "qc_handoff" ? (
                <section className="manju-production-canvas-entry" aria-label="P5 项目画布入口">
                    <div>
                        <FolderOpen />
                        <span>
                            <strong>项目画布</strong>
                            <small>{projectCanvasId ? "进入已创建的项目画布查看真实视频与素材。" : "请先在 P2/P3/P4 阶段创建项目画布。"}</small>
                        </span>
                    </div>
                    <Button
                        icon={<FolderOpen />}
                        disabled={!projectCanvasId || canvasImportsQuery.isLoading}
                        loading={canvasImportsQuery.isLoading}
                        onClick={() => {
                            if (projectCanvasId) navigate(manjuP5CanvasPath(projectId, projectCanvasId));
                        }}
                    >
                        进入画布
                    </Button>
                </section>
            ) : null}
            {stage.key === "qc_handoff" && canvasImportsQuery.isError ? (
                <InlineNotice tone="error">项目画布状态读取失败，请刷新后重试。</InlineNotice>
            ) : null}
            {stage.key === "qc_handoff" && detail.run.status === "COMPLETED" ? (
                <section className="manju-production-deliverables" aria-label="P5 已持久化交付物">
                    <header>
                        <Download />
                        <span>
                            <strong>最终交付物</strong>
                            <small>
                                {deliverables[0]?.complianceStatus === "LEGACY_COMPLIANT"
                                    ? "历史基础交付集已兼容；原有不可变行保持不动，缺失的预演故事板会单独补齐。"
                                    : "以下文件均来自服务端已锁定的交付记录。"}
                            </small>
                        </span>
                    </header>
                    {deliverablesQuery.isLoading ? <span>正在读取交付清单...</span> : null}
                    {deliverablesQuery.isError ? <InlineNotice tone="error">交付清单读取失败，请刷新后重试。</InlineNotice> : null}
                    {!deliverablesQuery.isLoading && !deliverablesQuery.isError && deliverables.length === 0 ? (
                        <span>服务端尚未生成交付文件。</span>
                    ) : null}
                    <div>
                        {deliverables.map((deliverable) => (
                            <article key={deliverable.id}>
                                <span>
                                    <strong>{manjuDeliverableKindLabel(deliverable)}</strong>
                                    <small>{deliverable.fileName}</small>
                                    <code>SHA-256 {deliverable.contentHash.slice(0, 12)}</code>
                                </span>
                                <Button icon={<Download />} onClick={() => downloadManjuDeliverable(deliverable)}>
                                    下载
                                </Button>
                            </article>
                        ))}
                    </div>
                </section>
            ) : null}
            <AppModal
                open={regenerateOpen}
                title={stage.key === "decision_anchor" ? "重新生成 P0B 工件" : stage.key === "director_storyboard" ? "重新生成 P3 工件" : "重新生成 P4 工件"}
                onCancel={() => setRegenerateOpen(false)}
                width={620}
                footer={
                    <div className="manju-regenerate-actions">
                        <Button onClick={() => setRegenerateOpen(false)}>取消</Button>
                        <Button
                            type="primary"
                            icon={<RefreshCw />}
                            loading={regenerateMutation.isPending}
                            onClick={() => regenerateMutation.mutate()}
                        >
                            只重新生成当前工件
                        </Button>
                    </div>
                }
            >
                <div className="manju-regenerate-dialog">
                    {stage.key === "decision_anchor" ? (
                        <p>只重新生成当前 P0B 工件。历史 P0B 工件和媒体保留；P3-P5 会等待新的决策锚点并重新校验。</p>
                    ) : stage.key === "director_storyboard" ? (
                        <>
                            <p>只重新生成当前 P3 工件。已锁定 P2、历史 P3/P4/P5 工件和媒体均保留；P4 与 P5 会等待新 P3。</p>
                            <label>
                                <span>本次补充要求</span>
                                <Input.TextArea
                                    rows={3}
                                    value={regenerateInstruction}
                                    placeholder="例如：减少环境复述，逐格保留动作相位。"
                                    onChange={(event) => setRegenerateInstruction(event.target.value)}
                                />
                            </label>
                        </>
                    ) : (
                        <p>只重新生成当前 P4 工件。已锁定 P3、历史投喂稿与视频保留；P5 会等待新 P4。</p>
                    )}
                </div>
            </AppModal>
            {stage.draft ? (
                <ManjuArtifactEditor
                    open={manualCorrectionOpen}
                    title="人工修正 P3 工件"
                    content={stage.draft.content}
                    validationJson={
                        manualCorrectionMutation.error
                            ? JSON.stringify({
                                  issues: [
                                      manualCorrectionMutation.error instanceof Error
                                          ? manualCorrectionMutation.error.message
                                          : "人工修正未通过官方机检",
                                  ],
                              })
                            : stage.draft.validationJson
                    }
                    loading={manualCorrectionMutation.isPending}
                    preview={compilePreview}
                    previewLoading={compilePreviewMutation.isPending}
                    onCancel={() => setManualCorrectionOpen(false)}
                    onSubmit={(content) => manualCorrectionMutation.mutate(content)}
                    onPreview={(content) => compilePreviewMutation.mutate(content)}
                />
            ) : null}
        </article>
    );
}

function MemberPanel({ detail, onChanged }: { detail: ManjuProjectDetail; onChanged: () => void }) {
    const user = useUserStore((state) => state.user);
    const [form, setForm] = useState<{ userId: string; role: Exclude<ManjuMemberRole, "owner"> }>({ userId: "", role: "producer" });
    const [notice, setNotice] = useState<{ tone: "error" | "success"; text: string } | null>(null);
    const canManage = detail.accessRole === "owner";
    const inviteMutation = useMutation({
        mutationFn: () => inviteManjuMember(detail.project.id, { userId: form.userId.trim(), role: form.role }),
        onSuccess: () => {
            setForm((current) => ({ ...current, userId: "" }));
            setNotice({ tone: "success", text: "成员已保存。重复邀请同一用户会更新原成员角色。" });
            onChanged();
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "成员邀请失败" }),
    });
    const removeMutation = useMutation({
        mutationFn: (memberId: string) => removeManjuMember(detail.project.id, memberId),
        onSuccess: () => {
            setNotice({ tone: "success", text: "成员已移除。" });
            onChanged();
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "成员移除失败" }),
    });
    const ownerLabel = detail.project.userId === user?.id ? `${user?.displayName || user?.username}（当前账号）` : detail.project.userId;

    return (
        <div className="manju-members-layout">
            <section className="manju-panel">
                <header className="manju-panel-head">
                    <div>
                        <span className="manju-kicker">项目权限</span>
                        <h2>成员</h2>
                        <p>主理由 Project.UserID 推导，不写入成员表。邀请按已有账号的用户 ID 添加。</p>
                    </div>
                    <span className="manju-member-count">{1 + (detail.members?.length || 0)} 人</span>
                </header>
                <div className="manju-member-list">
                    <div className="manju-member-row">
                        <span className="manju-member-avatar">主</span>
                        <span className="manju-member-copy">
                            <strong>{ownerLabel}</strong>
                            <small>{detail.project.userId}</small>
                        </span>
                        <span className="manju-role-badge is-owner">主理人</span>
                    </div>
                    {detail.members?.map((member) => (
                        <div className="manju-member-row" key={member.id}>
                            <span className="manju-member-avatar">{roleLabel(member.role).slice(0, 1)}</span>
                            <span className="manju-member-copy">
                                <strong>{roleLabel(member.role)}</strong>
                                <small>{member.userId}</small>
                            </span>
                            <span className={`manju-member-status is-${member.status}`}>{member.status === "active" ? "有效" : "停用"}</span>
                            {canManage ? (
                                <Button type="text" danger aria-label={`移除成员 ${member.userId}`} icon={<Trash2 />} loading={removeMutation.isPending && removeMutation.variables === member.id} onClick={() => removeMutation.mutate(member.id)} />
                            ) : null}
                        </div>
                    ))}
                    {!detail.members?.length ? <p className="manju-muted">还没有协作者。</p> : null}
                </div>
            </section>

            <aside className="manju-panel is-side">
                <header className="manju-panel-head">
                    <div>
                        <span className="manju-kicker">邀请成员</span>
                        <h2>添加已有账号</h2>
                    </div>
                </header>
                {canManage ? (
                    <>
                        <label className="manju-field">
                            <span>用户 ID</span>
                            <Input value={form.userId} placeholder="输入已注册用户 ID" onChange={(event) => setForm((current) => ({ ...current, userId: event.target.value }))} />
                        </label>
                        <label className="manju-field">
                            <span>项目角色</span>
                            <Select value={form.role} options={ROLE_OPTIONS} onChange={(value) => setForm((current) => ({ ...current, role: value }))} />
                        </label>
                        <Button type="primary" block icon={<UserPlus />} disabled={!form.userId.trim()} loading={inviteMutation.isPending} onClick={() => inviteMutation.mutate()}>
                            邀请成员
                        </Button>
                        <p className="manju-field-hint">用户名搜索接口尚未开放，当前必须使用用户 ID。不能把邮箱、显示名或 URL 参数当作身份。</p>
                    </>
                ) : (
                    <InlineNotice tone="info">只有主理人可以添加或移除成员。</InlineNotice>
                )}
                {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
            </aside>
        </div>
    );
}

function ModelPanel({ detail, onChanged }: { detail: ManjuProjectDetail; onChanged: () => void }) {
    const [draft, setDraft] = useState<ManjuModelDraft>(() => modelDraftFromDetail(detail));
    const [notice, setNotice] = useState<{ tone: "error" | "info" | "success"; text: string } | null>(null);
    const modelsQuery = useQuery({
        queryKey: ["model-catalog"],
        queryFn: getModelCatalog,
        staleTime: 60_000,
    });
    const groups = useMemo(() => {
        const result: Record<ManjuModelCapability, ManjuModelOption[]> = { text: [], image: [], video: [] };
        for (const channel of modelsQuery.data?.channels || []) {
            const channelName = channel.displayName || channel.name;
            for (const model of channel.models) {
                if (!model.available || !["text", "image", "video"].includes(model.capability)) continue;
                const capability = model.capability as ManjuModelCapability;
                result[capability].push({
                    value: modelSelectionValue(channel.id, model.modelKey),
                    label: `${model.displayName} · ${channelName}`,
                    channelId: channel.id,
                    channelName,
                    modelKey: model.modelKey,
                    modelName: model.displayName,
                });
            }
        }
        return result;
    }, [modelsQuery.data]);
    const canEdit = canEditModels(detail.accessRole);
    const slots: Array<{ capability: ManjuModelCapability; label: string; icon: LucideIcon }> = [
        { capability: "text", label: "文本模型", icon: BookOpenText },
        { capability: "image", label: "图片模型", icon: ImageIcon },
        { capability: "video", label: "视频模型", icon: Video },
    ];
    const selectedOption = (capability: ManjuModelCapability) => groups[capability].find((option) => option.channelId === draft[capability].channelId && option.modelKey === draft[capability].modelKey);
    const selectedCount = slots.filter(({ capability }) => Boolean(selectedOption(capability))).length;
    const hasPartialSelection = slots.some(({ capability }) => Boolean(draft[capability].channelId) !== Boolean(draft[capability].modelKey));
    const hasInvalidSelection = slots.some(({ capability }) => Boolean(draft[capability].channelId && draft[capability].modelKey) && !selectedOption(capability));
    const staleLabels = modelsQuery.isLoading ? [] : slots.filter(({ capability }) => draft[capability].channelId && !selectedOption(capability)).map(({ label }) => label);
    const missingCapabilities = slots.filter(({ capability }) => groups[capability].length === 0).map(({ label }) => label);
    const availableModelCount = slots.reduce((total, { capability }) => total + groups[capability].length, 0);
    const canSaveDraft = canEdit && selectedCount > 0 && !hasPartialSelection && !hasInvalidSelection;
    const mutation = useMutation({
        mutationFn: () => saveManjuModels(detail.project.id, draft),
        onSuccess: ({ item }) => {
            setDraft({ text: item.text, image: item.image, video: item.video });
            setNotice({ tone: "success", text: "当前模型路由已保存。尚未选择的能力会在使用到它的阶段提示补充。" });
            onChanged();
        },
        onError: (error) => setNotice({ tone: "error", text: error instanceof Error ? error.message : "模型配置保存失败" }),
    });
    const setSlot = (capability: ManjuModelCapability, value?: string) => {
        if (!value) {
            setNotice(null);
            setDraft((current) => ({ ...current, [capability]: { channelId: "", modelKey: "" } }));
            return;
        }
        const option = groups[capability].find((item) => item.value === value);
        if (!option) return;
        setNotice(null);
        setDraft((current) => ({
            ...current,
            [capability]: { channelId: option.channelId, modelKey: option.modelKey },
        }));
    };

    return (
        <div className="manju-model-layout">
            <section className="manju-panel">
                <header className="manju-panel-head">
                    <div>
                        <span className="manju-kicker">项目级模型路由</span>
                        <h2>文本、图片、视频</h2>
                        <p>只读取画布管理后台已经启用的系统模型。计费、支付渠道和对象存储仍由画布后台统一管理。</p>
                    </div>
                    <Button icon={<RefreshCw />} loading={modelsQuery.isFetching} onClick={() => void modelsQuery.refetch()}>
                        刷新
                    </Button>
                </header>
                {modelsQuery.isError ? (
                    <Alert type="error" showIcon message="模型目录读取失败" description={modelsQuery.error instanceof Error ? modelsQuery.error.message : "请检查画布模型配置。"} />
                ) : modelsQuery.isLoading ? (
                    <div className="manju-loading-panel">
                        <Spin />
                        <span>读取模型目录</span>
                    </div>
                ) : (
                    <div className="manju-model-groups is-routing">
                        {availableModelCount === 0 ? (
                            <Alert type="warning" showIcon message="画布后台还没有可用模型" description="请管理员先到画布管理后台的“模型渠道”新增并启用渠道，再为模型设置文本、图片或视频能力。配置完成后回到这里点“刷新”。" />
                        ) : missingCapabilities.length ? (
                            <InlineNotice tone="info">当前缺少{missingCapabilities.join("、")}。可以先保存已有模型，使用到缺失能力时系统会阻止对应阶段运行。</InlineNotice>
                        ) : null}
                        {slots.map(({ capability, label, icon: Icon }) => {
                            const options = groups[capability];
                            const selected = selectedOption(capability);
                            return (
                                <section className="manju-model-group manju-model-slot" key={capability}>
                                    <header>
                                        <span>
                                            <Icon /> {label}
                                        </span>
                                        <small>{options.length} 个可用</small>
                                    </header>
                                    <Select
                                        aria-label={label}
                                        value={selected?.value}
                                        options={options.map((option) => ({ value: option.value, label: option.label }))}
                                        placeholder={`请选择${label}`}
                                        allowClear
                                        showSearch
                                        optionFilterProp="label"
                                        disabled={!canEdit || options.length === 0}
                                        onChange={(value) => setSlot(capability, value ? String(value) : undefined)}
                                        notFoundContent="画布后台没有启用此类模型"
                                    />
                                    {selected ? (
                                        <p className="manju-model-selected">
                                            <span>{selected.channelName}</span>
                                            <code>{selected.modelKey}</code>
                                        </p>
                                    ) : (
                                        <p className="manju-muted">尚未选择</p>
                                    )}
                                </section>
                            );
                        })}
                    </div>
                )}
            </section>
            <aside className="manju-panel is-side">
                <header className="manju-panel-head">
                    <div>
                        <span className="manju-kicker">当前已保存</span>
                        <h2>项目模型路由</h2>
                    </div>
                </header>
                <dl className="manju-model-summary">
                    {slots.map(({ capability, label }) => {
                        const saved = detail.models?.[capability];
                        const option = groups[capability].find((item) => item.channelId === saved?.channelId && item.modelKey === saved?.modelKey);
                        return (
                            <div key={capability}>
                                <dt>{label}</dt>
                                <dd>{option?.modelName || saved?.modelKey || "未配置"}</dd>
                            </div>
                        );
                    })}
                </dl>
                {staleLabels.length ? <InlineNotice tone="error">以下已保存模型在当前画布目录中不可用，请重新选择：{staleLabels.join("、")}。</InlineNotice> : null}
                {hasPartialSelection ? <InlineNotice tone="error">模型槽必须同时保留渠道和模型标识，或者完整清空后再保存。</InlineNotice> : null}
                {notice ? <InlineNotice tone={notice.tone}>{notice.text}</InlineNotice> : null}
                {!canEdit ? <InlineNotice tone="info">当前角色是{roleLabel(detail.accessRole)}，只能查看模型配置。主理人或制片可以保存。</InlineNotice> : null}
                <Button type="primary" block icon={<Check />} disabled={!canSaveDraft} loading={mutation.isPending} onClick={() => mutation.mutate()}>
                    {selectedCount > 0 ? "保存模型配置" : "至少选择一个模型"}
                </Button>
                <p className="manju-field-hint">可先保存当前已有能力；每个阶段执行时会再次校验它需要的模型。P0 仍独立锁定视频业务模型与多段规则。</p>
            </aside>
        </div>
    );
}
