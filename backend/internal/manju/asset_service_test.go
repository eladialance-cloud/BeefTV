package manju_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
)

func TestCreateAssetRunRequiresCompletedLockedScript(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	scriptRun := &model.ManjuScriptRun{
		ID: "script-active", ProjectID: f.projectID, Branch: manju.ScriptBranchAdaptation,
		Mode: manju.ScriptModeStepwise, Status: model.ManjuScriptRunStatusActive,
		CurrentStage: manju.ScriptStageIPDevelopment, LockRevision: 1, CreatedBy: f.ownerID,
	}
	if err := f.db.Create(scriptRun).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuScriptStage{
		ID: "script-active-stage", RunID: scriptRun.ID, Key: manju.ScriptStageIPDevelopment,
		Title: "IP 与角色开发", Position: 1, Status: manju.ScriptStageStatusInProgress,
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRun.ID})

	if !errors.Is(err, manju.ErrAssetScriptNotReady) {
		t.Fatalf("P1 未完成时不能创建 P2，得到 %v", err)
	}
}

func TestCreateAssetRunInheritsP0AssetMode(t *testing.T) {
	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M0")

	detail, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatalf("创建 P2 失败: %v", err)
	}
	if detail.Run.AssetMode != manju.AssetModeM0 {
		t.Fatalf("P2 默认继承 P0 素材形态 M0，得到 %q", detail.Run.AssetMode)
	}
	if detail.Run.ScriptRunID != scriptRunID || detail.Run.Status != model.ManjuAssetRunStatusActive {
		t.Fatalf("P2 必须绑定锁定剧本任务: %+v", detail.Run)
	}
	if len(detail.Stages) != 3 || detail.Stages[0].Key != manju.AssetStageExtraction || detail.Stages[0].Status != manju.AssetStageStatusInProgress {
		t.Fatalf("P2 首阶段必须是实体提取: %+v", detail.Stages)
	}
	if detail.Stages[1].Status != manju.AssetStageStatusNotStarted || detail.Stages[2].Status != manju.AssetStageStatusNotStarted {
		t.Fatalf("未到门禁的后续阶段不得提前启动: %+v", detail.Stages)
	}
}

func TestSwitchingAssetModeSupersedesActiveRun(t *testing.T) {
	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M0")
	first, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatal(err)
	}
	if first.Run.AssetMode != manju.AssetModeM0 {
		t.Fatalf("首个任务应为 M0，得到 %s", first.Run.AssetMode)
	}
	second, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID, AssetMode: "M1"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Run.AssetMode != manju.AssetModeM1 || second.Run.Status != model.ManjuAssetRunStatusActive {
		t.Fatalf("切换后应建立 ACTIVE M1，得到 %+v", second.Run)
	}
	var previous model.ManjuAssetRun
	if err := f.db.First(&previous, "id = ?", first.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if previous.Status != model.ManjuAssetRunStatusSuperseded {
		t.Fatalf("旧 P2 任务必须标记 SUPERSEDED，得到 %s", previous.Status)
	}
}

func canvasTaskBlockFixture() string {
	return "\n\n<!-- MANJU_CANVAS_TASKS_JSON\n" +
		`{"version":1,"tasks":[{"code":"图1","assetCode":"CHR-林澈","title":"林澈角色四格","category":"character","prompt":"角色四格纯图板，全图共 4 格。","aspectRatio":"16:9"}]}` +
		"\n-->"
}

func TestAssetExtractionUsesOnlyLockedScriptArtifacts(t *testing.T) {

	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M1")
	if err := f.db.Create(&model.ManjuScriptStage{
		ID: "script-unlocked-stage", RunID: scriptRunID, Key: "unlocked_draft",
		Title: "未锁定草稿", Position: 9, Status: manju.ScriptStageStatusInProgress,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuScriptArtifact{
		ID: "script-unlocked-artifact", ProjectID: f.projectID, RunID: scriptRunID,
		StageID: "script-unlocked-stage", Kind: "UNLOCKED.md", Version: 1,
		Content: "UNLOCKED_SECRET_DO_NOT_USE", ContentHash: "unlocked", CreatedBy: f.ownerID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuScriptSource{
		ID: "source-raw", ProjectID: f.projectID, RunID: scriptRunID, Branch: manju.ScriptBranchAdaptation,
		Title: "小说原文", ContentText: "RAW_NOVEL_SECRET_DO_NOT_USE", ContentHash: "raw",
		AuthorizationScope: "授权", RangeLabel: "第 1 章", ExtractionStatus: "ready", CreatedBy: f.ownerID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	run, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "asset-extract-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text",
		Status: model.TaskStatusQueued, Prompt: "提取资产实体",
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: task}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageExtraction, manju.AssetStageRunInput{
		Instruction: "只从定稿剧本提取",
	}); err != nil {
		t.Fatalf("提交实体提取任务失败: %v", err)
	}
	if !strings.Contains(creator.request.Prompt, "LOCKED_SCRIPT_FACT") {
		t.Fatalf("P2 必须读取锁定剧本工件: %s", creator.request.Prompt)
	}
	if !strings.Contains(creator.request.Prompt, "高反差大光比") {
		t.Fatalf("P2 必须收到画风卡的可投喂视觉参数: %s", creator.request.Prompt)
	}
	p0Prompt := creator.request.Prompt
	if boundary := strings.Index(p0Prompt, "===== V7.4.26 严格规则执行边界 ====="); boundary >= 0 {
		p0Prompt = p0Prompt[:boundary]
	}
	if strings.Contains(p0Prompt, "徐克") || strings.Contains(p0Prompt, "sd-style-009") || strings.Contains(p0Prompt, "styleCardName") || strings.Contains(p0Prompt, "styleCardId") {
		t.Fatalf("P2 P0 交付参数不得携带画风卡名称或选择层 ID: %s", p0Prompt)
	}
	if strings.Contains(creator.request.Prompt, "UNLOCKED_SECRET_DO_NOT_USE") || strings.Contains(creator.request.Prompt, "RAW_NOVEL_SECRET_DO_NOT_USE") {
		t.Fatalf("P2 不得读取未锁定工件或原始小说正文: %s", creator.request.Prompt)
	}
	metadata, _ := creator.request.Input["metadata"].(map[string]any)
	if metadata["manjuAssetRunId"] != run.Run.ID || metadata["manjuAssetStageKey"] != manju.AssetStageExtraction {
		t.Fatalf("资产任务必须携带 P2 追踪元数据: %+v", metadata)
	}
}

func TestAssetManifestMustBeConfirmedBeforeCompile(t *testing.T) {
	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M1")
	run, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatal(err)
	}
	tasks := []*model.Task{
		{ID: "asset-task-extract", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-task-manifest", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-task-compile", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: tasks}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageExtraction, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[0].ID, "# 实体提取\n| CHR-林澈 | A |\n| SCN-旧宅 | 核心 |")
	detail, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	extraction := findAssetStage(t, detail, manju.AssetStageExtraction)
	if extraction.Status != manju.AssetStageStatusLocked || extraction.ArtifactID == "" {
		t.Fatalf("实体提取成功后必须锁定并进入清单阶段: %+v", extraction)
	}

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageManifest, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[1].ID, "# 资产清单\n| CHR-林澈 | A |\n| SCN-旧宅 | 核心 |")
	detail, err = f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	manifestStage := findAssetStage(t, detail, manju.AssetStageManifest)
	if manifestStage.Status != manju.AssetStageStatusAwaitingConfirmation || manifestStage.ArtifactID == "" {
		t.Fatalf("资产清单必须等待用户确认: %+v", manifestStage)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageCompile, manju.AssetStageRunInput{}); !errors.Is(err, manju.ErrAssetStageNotReady) {
		t.Fatalf("清单未确认不得编译交付稿，得到 %v", err)
	}

	detail, err = f.service.ConfirmAssetManifest(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findAssetStage(t, detail, manju.AssetStageCompile).Status; got != manju.AssetStageStatusInProgress {
		t.Fatalf("确认清单后编译阶段应启动，得到 %s", got)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageCompile, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	if len(creator.requests) < 3 || strings.Contains(creator.requests[2].Prompt, "MANJU_CANVAS_TASKS_JSON") {
		t.Fatalf("P2 编译任务不得要求模型额外生成画布任务块: %+v", creator.requests)
	}
	if !strings.Contains(creator.requests[2].Prompt, "MANJU_PROJECT_STATE_PATCH_JSON") {
		t.Fatalf("P2 编译任务必须要求 V7.4.26 project_state 回写块: %+v", creator.requests[2].Prompt)
	}
	validM1 := "> 素材形态：M1（编号占位）\n\n# 资产编号册\n图1 = 林澈\n\n## 素材接线单\n| 图号 | 用途 | 只控制什么 | 不控制什么 |\n| 图1 | 角色 | 脸型与服装 | 姿态与场景 |"
	validM1 += canvasTaskBlockFixture()
	succeedManjuTask(t, f, tasks[2].ID, validM1)
	detail, err = f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	compile := findAssetStage(t, detail, manju.AssetStageCompile)
	if compile.Status != manju.AssetStageStatusAwaitingConfirmation || compile.ArtifactID == "" {
		t.Fatalf("M1 完整交付必须等待最终确认: %+v", compile)
	}
	detail, err = f.service.ConfirmAssetRun(f.ownerID, f.projectID, run.Run.ID, manju.AssetConfirmInput{})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Run.Status != model.ManjuAssetRunStatusCompleted {
		t.Fatalf("最终确认后 P2 必须完成: %+v", detail.Run)
	}
}

func TestM2CompileRequiresRegisteredReferences(t *testing.T) {
	f := newFixture(t)
	scriptRunID := prepareCompletedScript(t, f, "M2")
	run, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatal(err)
	}
	tasks := []*model.Task{
		{ID: "asset-m2-extract", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-m2-manifest", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-m2-compile-ready", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: tasks}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageExtraction, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[0].ID, "# 实体提取\n| CHR-林澈 | A |")
	if _, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageManifest, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[1].ID, "# 资产清单\n| CHR-林澈 | A |")
	if _, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmAssetManifest(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageCompile, manju.AssetStageRunInput{}); !errors.Is(err, manju.ErrAssetReferencesMissing) || !strings.Contains(err.Error(), "CHR-林澈") {
		t.Fatalf("M2 未登记参考图必须在提交模型前阻断，得到 %v", err)
	}
	if len(creator.requests) != 2 {
		t.Fatalf("M2 缺参考图不得提交编译模型任务，实际请求数 %d", len(creator.requests))
	}
	detail, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if compile := findAssetStage(t, detail, manju.AssetStageCompile); compile.Status != manju.AssetStageStatusInProgress || compile.TaskID != "" {
		t.Fatalf("M2 缺参考图时编译阶段应保持可恢复状态，得到 %+v", compile)
	}

	if _, err := f.service.RegisterAssetReference(f.ownerID, f.projectID, run.Run.ID, manju.AssetReferenceInput{
		Code: "CHR-林澈", AssetType: "CHR", ResourceID: "res-chr-lin", URL: "https://assets.example/lin.png",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageCompile, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[2].ID, "> 素材形态：M2（已入库）\n\n# 资产图册\nCHR-林澈 · res-chr-lin"+canvasTaskBlockFixture())
	detail, err = f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findAssetStage(t, detail, manju.AssetStageCompile).Status; got != manju.AssetStageStatusAwaitingConfirmation {
		t.Fatalf("登记全部 M2 参考图后交付稿应等待最终确认，得到 %s", got)
	}
}

func TestAssetCompileRunsV7426AssetBlockGate(t *testing.T) {
	f := newFixture(t)
	f.service.SetSkillValidationEnabledForTest(true)
	scriptRunID := prepareCompletedScript(t, f, "M1")
	run, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{ScriptRunID: scriptRunID})
	if err != nil {
		t.Fatal(err)
	}
	tasks := []*model.Task{
		{ID: "asset-gate-extract", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-gate-manifest", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "asset-gate-compile", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: tasks}
	f.service.SetScriptTaskCreator(creator.create)

	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageExtraction, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[0].ID, "# 实体提取\n| CHR-林澈 | 主角 |")
	if _, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageManifest, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[1].ID, "# 资产清单\n| CHR-林澈 | A |\n| SCN-旧宅 | 核心 |")
	if _, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmAssetManifest(f.ownerID, f.projectID, run.Run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageCompile, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}

	missingTemplateFields := `> 素材形态：M1（编号占位）

# 资产编号册
图1 = 林澈

## 素材接线单
| 图号 | 用途 | 只控制什么 | 不控制什么 |
| 图1 | 角色 | 脸型与服装 | 姿态与场景 |

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{"characters":{"林澈":{"id":"CHR-林澈","fig_no":1}},"scenes":{"旧宅":{"id":"SCN-旧宅","fig_no":2}},"props":{}}
-->
` + canvasTaskBlockFixture()
	succeedManjuTask(t, f, tasks[2].ID, missingTemplateFields)
	detail, err := f.service.AssetRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	compile := findAssetStage(t, detail, manju.AssetStageCompile)
	if compile.Status != model.ManjuAssetStageStatusBlocked ||
		compile.ArtifactID != "" ||
		!strings.Contains(compile.Error, "check_asset_blocks.py") {
		t.Fatalf("P2 资产结构未通过 check_asset_blocks.py 时必须阻断且不得生成工件: %+v", compile)
	}
}

func prepareCompletedScript(t *testing.T, f fixture, assetMode string) string {
	t.Helper()
	lock := validLock()
	lock.AssetMode = assetMode
	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-asset-" + assetMode, ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
		ImageChannelID: "channel-image", ImageModelKey: "image-pro",
		VideoChannelID: "channel-video", VideoModelKey: "video-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	scriptRun := &model.ManjuScriptRun{
		ID: "script-ready-" + assetMode, ProjectID: f.projectID, Branch: manju.ScriptBranchAdaptation,
		Mode: manju.ScriptModeStepwise, Status: model.ManjuScriptRunStatusCompleted,
		CurrentStage: "", LockRevision: receipt.Revision, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(scriptRun).Error; err != nil {
		t.Fatal(err)
	}
	stages := []model.ManjuScriptStage{
		{
			ID: "script-ready-ip-" + assetMode, RunID: scriptRun.ID, Key: manju.ScriptStageIPDevelopment,
			Title: "IP 与角色开发", Position: 1, Status: manju.ScriptStageStatusLocked, Version: 1,
		},
		{
			ID: "script-ready-draft-" + assetMode, RunID: scriptRun.ID, Key: manju.ScriptStageScriptDraft,
			Title: "正式剧本", Position: 2, Status: manju.ScriptStageStatusLocked, Version: 1,
		},
	}
	if err := f.db.Create(&stages).Error; err != nil {
		t.Fatal(err)
	}
	artifacts := []model.ManjuScriptArtifact{
		{
			ID: "artifact-ip-" + assetMode, ProjectID: f.projectID, RunID: scriptRun.ID,
			StageID: stages[0].ID, Kind: "IP_DEVELOPMENT.md", Version: 1,
			Content: "LOCKED_IP_FACT 林澈是档案修复师。", ContentHash: "locked-ip", CreatedBy: f.ownerID,
		},
		{
			ID: "artifact-draft-" + assetMode, ProjectID: f.projectID, RunID: scriptRun.ID,
			StageID: stages[1].ID, Kind: "SCRIPT_DRAFT.md", Version: 1,
			Content: "LOCKED_SCRIPT_FACT 林澈推开旧宅的门。", ContentHash: "locked-script", CreatedBy: f.ownerID,
		},
	}
	if err := f.db.Create(&artifacts).Error; err != nil {
		t.Fatal(err)
	}
	for index := range stages {
		var stored model.ManjuScriptStage
		if err := f.db.First(&stored, "id = ?", stages[index].ID).Error; err != nil {
			t.Fatal(err)
		}
		stored.ArtifactID = artifacts[index].ID
		if err := f.db.Save(&stored).Error; err != nil {
			t.Fatal(err)
		}
	}
	return scriptRun.ID
}

func findAssetStage(t *testing.T, detail *manju.AssetRunDetail, key string) manju.AssetStageView {
	t.Helper()
	for _, stage := range detail.Stages {
		if stage.Key == key {
			return stage
		}
	}
	t.Fatalf("缺少 P2 阶段 %s: %+v", key, detail.Stages)
	return manju.AssetStageView{}
}

func succeedManjuTask(t *testing.T, f fixture, taskID, text string) {
	t.Helper()
	payload := `{"mode":"text","text":` + mustJSONString(text) + `}`
	if err := f.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成", "result_json": payload,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func mustJSONString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + replacer.Replace(value) + `"`
}

var _ localtask.CreateRequest
