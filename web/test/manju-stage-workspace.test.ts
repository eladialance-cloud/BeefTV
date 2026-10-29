import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const page = readFileSync(resolve(import.meta.dir, "../src/pages/manju/index.tsx"), "utf8");
const api = readFileSync(resolve(import.meta.dir, "../src/services/api/manju.ts"), "utf8");
const styles = readFileSync(resolve(import.meta.dir, "../src/pages/manju/manju-studio.css"), "utf8");
const sourceLibraryPanel = readFileSync(resolve(import.meta.dir, "../src/pages/manju/source-library-steps-panel.tsx"), "utf8");

test("P1-P5 render as independent stage navigation", () => {
    expect(page).toContain("function PipelineWorkspace");
    expect(page).toContain('className="manju-pipeline-nav"');
    expect(page).toContain("P1-P5 独立阶段");
    for (const label of ["P1 剧本", "P2 数字资产", "P3 分镜", "P4 投喂", "P5 质检交付"]) {
        expect(page).toContain(label);
    }
});

test("generated markdown uses a readable artifact font size", () => {
    expect(styles).toMatch(/\.manju-script-artifact pre\s*\{[^}]*font-size:\s*15px;/su);
});

test("current P0 revision isolates historical P1-P5 runs", () => {
    expect(page).toContain("function isCurrentLockRevision");
    expect(page).toContain("旧 revision 的 P1-P5 运行链已隔离");
    expect(page).toContain("检测到 {historicalRunCount} 条旧 P0 revision");
});

test("P4 is presented as the official compiler instead of a text task", () => {
    expect(page).toContain("官方编译 P4 投喂稿");
    expect(page).toContain("P4 使用官方 compile_feed.py，不调用文本模型");
    expect(page).toContain("等待官方 compile_feed.py 编译");
    expect(styles).toContain(".manju-production-stage-input.is-compiler");
});

test("P4 and P5 expose official machine-check status", () => {
    expect(page).toContain("compile_feed.py + validate_prompt.py + gate_check.py + script_feed_diff.py 已通过");
    expect(page).toContain("audit_contract.py 全流程契约审计已通过");
    expect(styles).toContain(".manju-production-audit.is-passed");
});

test("P4 validation cannot be skipped", () => {
    expect(page).not.toContain("跳过官方双机检");
    expect(page).not.toContain("skipValidation");
    expect(api).not.toContain("skipValidation");
    expect(page).toContain("script_feed_diff.py");
    expect(page).toContain("历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。");
    expect(page).toContain("未记录完整四步机检，不能作为 PASS");
    expect(api).toContain("machineCheckProofVerified?: boolean");
    expect(page).toContain('stage.machineCheckStatus === "PASS" && stage.machineCheckProofVerified === true');
});

test("P4.5 is an independent P4 gate backed by check_p45.py", () => {
    expect(page).toContain("P4.5 预演故事板");
    expect(page).toContain("生成 P4.5 预演故事板");
    expect(page).toContain("P4.5 必须通过 check_p45.py");
    expect(page).toContain("PREVIS_STORYBOARD.md");
});

test("P5 deliverables render only persisted backend rows", () => {
    expect(api).toContain("productionRunId: string");
    expect(api).toContain("fileName: string");
    expect(api).toContain("mimeType: string");
    expect(page).toContain("getManjuUserDeliverables");
    expect(page).toContain('deliverable.kind === "markdown"');
    expect(page).toContain('deliverable.kind === "board"');
    expect(page).toContain('deliverable.kind === "manifest"');
    expect(page).toContain("Markdown 源稿");
    expect(page).toContain("离线 HTML 看板");
    expect(page).toContain("交付清单");
    expect(page).toContain("item.productionRunId === detail.run.id");
    expect(page).toContain("downloadManjuDeliverable(deliverable)");
});

test("M2 compile explains the missing reference gate instead of silently hiding the button", () => {
    expect(page).toContain("const compileBlocked");
    expect(page).toContain("M2 资产交付编译被参考图门禁暂停");
    expect(page).toContain("现在没有提交模型任务，也不会扣除模型成本");
    expect(page).toContain("去登记参考图");
    expect(styles).toContain(".manju-asset-compile-block");
});

test("generated P2 compile artifacts remain manually editable before and after confirmation", () => {
    expect(page).toContain('stage.key === "asset_compile" && artifact?.content');
    expect(page).toContain('stage.key === "asset_compile" && ["AWAITING_CONFIRMATION", "LOCKED"].includes(stage.status)');
    expect(page).toContain("直接编辑当前已生成工件");
    expect(page).toContain("content={manualCorrectionContent}");
});

test("switching asset mode creates a new run instead of being blocked by the current M0 run", () => {
    expect(page).toContain("切换为 ${assetMode} 并新建");
    expect(page).toContain("会停用当前未完成的");
    expect(page).toContain('assetRun.run.status === "SUPERSEDED"');
});

test("P3 cannot bypass storyboard image delivery with an optional-image message", () => {
    expect(page).toContain("P3 必须逐段完成故事板交付。");
    expect(page).toContain("普通“无需图片”不能绕过。");
    expect(page).not.toContain("故事板图片是可选的视觉参考");
    expect(page).not.toContain("没有图片也可以确认进入 P4");
});

test("P4 canvas packaging uses only current ready P3 image lineage", () => {
    expect(page).toContain("sourceP3ArtifactId");
    expect(page).toContain('image.status === "READY"');
    expect(page).toContain('image.status !== "SUPERSEDED"');
    expect(page).toContain("image.runId === sourceContext.productionRunId");
    expect(page).toContain("image.artifactId === sourceContext.p3ArtifactId");
    expect(page).toContain('if (sourceStage === "p3_images")');
});

test("P1 never shows an empty state while the active run is still loading", () => {
    expect(page).toContain("runsQuery.isLoading || runQuery.isLoading");
    expect(page).toContain("正在读取剧本任务版");
    expect(sourceLibraryPanel).toContain("runsLoading");
    expect(sourceLibraryPanel).toContain("正在读取当前任务版");
    expect(sourceLibraryPanel).toContain('queryKey: ["manju", "script-runs", projectId]');
});

test("P0B decision artifact renders inside P3 and gates storyboard", () => {
    expect(page).toContain("decision_anchor: {");
    expect(page).toContain('aria-label="P0B-P5 生产链工作区"');
    expect(page).toContain("const productionStagesForTab");
    expect(page).toContain('const decisionAnchorLocked = detail.stages.find((item) => item.key === "decision_anchor")?.status === "LOCKED"');
    expect(page).toContain('stage.key === "director_storyboard" && !decisionAnchorLocked');
    expect(page).toContain("P0B 锁定后才能执行 P3 分镜");
    expect(page).not.toContain("manju-script-decision-gate");
});

test("roundtable review requires two explicit user decisions", () => {
    expect(api).toContain("export type ManjuRoundtableDecision");
    expect(api).toContain("export type ManjuRoundtableDecisionRecord");
    expect(api).toContain("PROCEED_CURRENT");
    expect(api).toContain("REFINE_OR_REWRITE");
    expect(api).toContain("decideManjuRoundtable");
    expect(api).toContain("acceptQualityRisk");
    expect(page).toContain("decideRoundtableMutation");
    expect(page).toContain("qualityRiskRequired");
    expect(page).toContain("USER_ACCEPTED_QUALITY_RISK");
    expect(page).toContain("接受当前稿");
    expect(page).toContain("精修并重写");
    expect(page).not.toContain("提交圆桌决策并锁定");
    expect(page).not.toContain('"AWAITING_USER_DECISION"].includes(stage.status)');
});
