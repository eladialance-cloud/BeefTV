import { describe, expect, test } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dir, "..");

function source(path: string) {
    const absolute = resolve(root, path);
    return existsSync(absolute) ? readFileSync(absolute, "utf8") : "";
}

describe("manju studio hosted integration", () => {
    test("canvas sidebar and router expose list and project routes", () => {
        const tools = source("src/constant/navigation-tools.ts");
        const sidebar = source("src/components/layout/workspace-sidebar-nav.tsx");
        const router = source("src/router.tsx");
        const routeModules = source("src/lib/workspace-route-modules.ts");

        expect(tools).toContain('slug: "manju"');
        expect(tools).toContain('label: "漫剧工作室"');
        expect(sidebar).toContain('toolItem("manju", "/manju")');
        expect(routeModules).toContain('manju: () => import("@/pages/manju")');
        expect(router).toContain("const ManjuStudioPage = lazy(loadManjuStudioPage)");
        expect(router).toContain('{ path: "/manju", element: deferred(<ManjuStudioPage />) }');
        expect(router).toContain('{ path: "/manju/:projectId", element: deferred(<ManjuStudioPage />) }');
        expect(sidebar).toContain('toolItem("projects", "/projects")');
        expect(sidebar).toContain('toolItem("canvas", "/canvas")');
        expect(router).toContain("const ProjectsPage = lazy(loadProjectsPage)");
        expect(router).toContain("element: deferred(<ProjectsPage />)");
    });

    test("page uses hosted APIs instead of an iframe or browser business storage", () => {
        const page = source("src/pages/manju/index.tsx");

        expect(page).toContain("listManjuProjects");
        expect(page).toContain("getManjuProject");
        expect(page).toContain("saveManjuLock");
        expect(page).toContain("inviteManjuMember");
        expect(page).toContain("saveManjuModels");
        expect(page).not.toContain("<iframe");
        expect(page).not.toContain("manju-studio.html");
        expect(page).not.toContain("localStorage");
    });

    test("project library lets only the owner delete with confirmation", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");

        expect(page).toContain("deleteManjuProject");
        expect(page).toContain('project.accessRole === "owner"');
        expect(page).toContain('<article className="manju-project-card"');
        expect(page).toContain("manju-project-card-actions");
        expect(page).toContain("删除漫剧项目");
        expect(page).toContain("P0-P5 项目数据会删除且不可恢复");
        expect(page).toContain("已完成任务、结果记录和独立资源文件会保留");
        expect(page).toContain("okButtonProps={{ danger: true }}");
        expect(api).toContain("export function deleteManjuProject");
        expect(api).toContain("http.delete<{ deleted: boolean; id: string }>(`/manju/projects/${encodeURIComponent(projectId)}`)");
    });

    test("project model panel persists only text, image and video bindings", () => {
        const page = source("src/pages/manju/index.tsx");

        expect(page).toContain("保存模型配置");
        expect(page).toContain("文本模型");
        expect(page).toContain("图片模型");
        expect(page).toContain("视频模型");
        expect(page).not.toContain("音频模型");
        expect(page).not.toContain("阶段级绑定尚未持久化");
        expect(page).toContain("画布后台还没有可用模型");
        expect(page).toContain("hasPartialSelection");
        expect(page).toContain("canSaveDraft");
        expect(page).toContain("allowClear");
        expect(page).not.toContain("const allSelected");
    });

    test("P0 style lock uses the V7.4.26 progressive catalog instead of a free-form card id", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");

        expect(api).toContain("getManjuStyleCatalog");
        expect(api).toContain('"/manju/style-catalog"');
        expect(page).toContain("影像基底");
        expect(page).toContain("技法家族");
        expect(page).toContain("风格候选卡");
        expect(page).toContain("查看全部 74 组");
        expect(page).toContain("按你的推荐走");
        expect(page).toContain("正向画风");
        expect(page).not.toContain("<span>画风卡 ID</span>");
        expect(page).not.toContain('placeholder="必填，例如 style-cold-realism"');
    });

    test("P0 hard locks remain empty until the user explicitly confirms each one", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");

        expect(api).toContain("export type ManjuLockKey");
        expect(api).toContain("confirmedLocks: ManjuLockKey[]");
        expect(api).toContain("confirmedLocks?: ManjuLockKey[]");
        expect(page).toContain("useState<Set<ManjuLockKey>>");
        expect(page).toContain("function defaultLock(): ManjuLockInput");
        expect(page).toContain('track: "",');
        expect(page).toContain('source: "",');
        expect(page).toContain('videoModel: "",');
        expect(page).toContain('aspectRatio: "",');
        expect(page).toContain('deliveryMode: "",');
        expect(page).toContain('styleBase: "",');
        expect(page).toContain("confirmedLocks: [],");
        expect(page).toContain("const readyToLockP0");
        expect(page).toContain("disabled={!canEdit || !readyToLockP0}");
        expect(page).toContain("锁定 P0");
    });

    test("project model slots stack on narrow screens without horizontal clipping", () => {
        const css = source("src/pages/manju/manju-studio.css");

        expect(css).toContain(".manju-model-slot .ant-select-selection-item {");
        expect(css).toContain(".manju-model-groups,\n    .manju-model-groups.is-routing {");
        expect(css).toContain("grid-template-columns: minmax(0, 1fr);");
    });

    test("page keeps unfinished P1-P5 stages truthful", () => {
        const page = source("src/pages/manju/index.tsx");
        expect(page).toContain("P3-P5");
        expect(page).not.toContain("P2-P5");
        expect(page).not.toContain("P1-P2 与 P4-P5 使用真实文本任务");
        expect(page.match(/从立项锁开始。/g)).toHaveLength(1);

        for (const stage of ["P1 剧本", "P2 数字资产", "P3 导演工作台", "P4 投喂封装", "P5 质检与交付"]) {
            expect(page).toContain(stage);
        }
        expect(page).toContain("未执行");
        expect(page).not.toContain("机检已完成");
        expect(page).not.toContain("交付包已生成");
    });

    test("P3-P5 workspace is driven by real production run APIs", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");

        for (const token of ["createManjuProductionRun", "listManjuProductionRuns", "getManjuProductionRun", "runManjuProductionStage", "confirmManjuProductionStage", "getManjuCanvasPackage", "saveManjuCanvasImport"]) {
            expect(api).toContain(token);
            expect(page).toContain(token);
        }
        expect(page).toContain("function ProductionPipeline");
        expect(page).toContain("视频交付与人工看片边界");
        expect(page).not.toContain("未读取生成视频");
        expect(page).not.toContain("P5 文本质检");
        expect(page).not.toContain("function FutureStageContracts");
        expect(page).toContain("故事板表");
        expect(page).toContain("故事板图");
        expect(page).toContain("function ManjuCanvasPackagePanel");
        expect(page).toContain("createManjuCanvasPackage");
        expect(page).toContain("canvasPackageStage");
        expect(page).toContain("故事板图");
        expect(page).toContain("剧情分镜格");
        expect(page).toContain("P3 交付清单");
        expect(page).toContain("素材接线单");
        expect(page).toContain("待主理人看片");
        expect(page).not.toContain("机检已完成");
        expect(page).not.toContain("交付包已生成");
    });

    test("P1 timeline is driven by real script run and stage APIs", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");

        for (const token of ["createManjuScriptRun", "getManjuScriptRun", "registerManjuScriptSource", "runManjuScriptStage", "confirmManjuScriptStage"]) {
            expect(api).toContain(token);
            expect(page).toContain(token);
        }
        expect(page).toContain("来源登记");
        expect(page).toContain("任务执行中");
        expect(page).toContain("确认并进入下一阶段");
        expect(page).toContain("阶段工件");
        expect(page).not.toContain("P1-P5 尚未接入真实执行器");
    });

    test("four P0 sources route polish and trend through explicit inputs", () => {
        const page = source("src/pages/manju/index.tsx");
        const api = source("src/services/api/manju.ts");
        const sourcePanel = source("src/pages/manju/source-library-steps-panel.tsx");

        expect(api).toContain('export type ManjuSource = "original" | "adaptation" | "benchmark" | "trend";');
        expect(api).toContain("inputKind");
        expect(api).toContain("trendResolution");
        expect(page).toContain("const INPUT_KIND_OPTIONS");
        expect(page).toContain('{ value: "novel", label: "小说 / 网文" }');
        expect(page).toContain('{ value: "outline", label: "大纲 / 梗概" }');
        expect(page).toContain('{ value: "finished_script", label: "成型外部剧本" }');
        expect(page).toContain("const TREND_RESOLUTION_OPTIONS");
        expect(page).toContain('{ value: "original", label: "选定方向后按原创执行" }');
        expect(page).toContain('{ value: "benchmark", label: "选定方向后按对标执行" }');
        expect(page).toContain("inputKind");
        expect(page).toContain("trendResolution");
        expect(sourcePanel).toContain("inputKind");
    });

    test("P2 asset workspace is a real gated flow with no M0/M1 image upload", () => {
        const page = source("src/pages/manju/index.tsx");
        expect(page).toContain("function AssetPipeline");
        expect(page).toContain("执行实体提取");
        expect(page).toContain("确认清单并进入编译");
        expect(page).toContain("最终确认 P2");
        expect(page).toContain("M0/M1 不需要上传图片");
        expect(page).toContain('assetRun.run.assetMode === "M2"');
    });

    test("legacy standalone mock studio is retired", () => {
        expect(existsSync(resolve(root, "public/manju-studio.html"))).toBe(false);
        expect(existsSync(resolve(root, "public/manju-studio-icons.js"))).toBe(false);
        expect(existsSync(resolve(root, "test/manju-studio-v7420.browser.test.ts"))).toBe(false);
    });

    test("manju projects are separated from generic projects and canvas chrome", () => {
        const projectDetail = source("src/pages/projects/detail.tsx");
        const canvasProject = source("src/pages/canvas/project.tsx");

        expect(projectDetail).toContain('project?.type === "manju"');
        expect(projectDetail).toContain("return <Navigate to={`/manju/${encodeURIComponent(project.id)}`} replace />");
        expect(canvasProject).toContain("const isManjuLinkedProject");
        expect(canvasProject).toContain("currentProject?.projectId && !isManjuLinkedProject");
    });

    test("API service owns every hosted manju endpoint", () => {
        const api = source("src/services/api/manju.ts");

        expect(api).toContain('"/manju/projects"');
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}`");
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}/lock`");
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}/members`");
        expect(api).toContain("export function saveManjuModels");
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}/models`");
        expect(api).toContain("export function createManjuAssetRun");
        expect(api).toContain("export function runManjuAssetStage");
        expect(api).toContain("export function confirmManjuAssetManifest");
        expect(api).toContain("export function confirmManjuAssetRun");
        expect(api).toContain("export function registerManjuAssetReference");
        expect(api).toContain("export function createManjuProductionRun");
        expect(api).toContain("export function listManjuProductionRuns");
        expect(api).toContain("export function getManjuProductionRun");
        expect(api).toContain("export function runManjuProductionStage");
        expect(api).toContain("export function confirmManjuProductionStage");
        expect(api).toContain("export function startManjuStoryboardImages");
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/storyboard-images`");
        expect(api).toContain("export function startManjuProductionVideos");
        expect(api).toContain("`/manju/projects/${encodeURIComponent(projectId)}/production-runs/${encodeURIComponent(runId)}/videos`");
        expect(api).toContain("production-runs");
    });
    test("studio workspaces fill the available canvas width", () => {
        const css = source("src/pages/manju/manju-studio.css");

        expect(css).not.toContain("max-width: 1280px;");
        expect(css).not.toContain("width: min(1160px, 100%);");
        expect(css).toContain("padding: 18px 16px 30px;");
        expect(css).toContain("grid-template-columns: repeat(auto-fill, minmax(min(100%, 280px), 1fr));");
        expect(css).toContain("grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr));");
        expect(css).not.toContain("grid-template-columns: repeat(auto-fit");
        expect(css).toContain(".manju-fact-strip {\n    display: grid;\n    grid-template-columns: repeat(4, minmax(0, 1fr));");
        expect(css).toContain(".manju-page-header:not(.is-project) + .manju-page-body {");
        expect(css).toContain("min-height: max(360px, calc(100dvh - 270px));");
    });
});
