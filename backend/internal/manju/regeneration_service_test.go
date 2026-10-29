package manju_test

import (
	"errors"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestRegenerateAssetRunReusesReferencesByDefault(t *testing.T) {
	f := newFixture(t)
	baseRunID := prepareCompletedProductionAsset(t, f, "M2")
	now := time.Now()
	if err := f.db.Create(&model.ManjuAssetReference{
		ID: "regenerate-base-reference", ProjectID: f.projectID, RunID: baseRunID,
		Code: "CHR-林澈", AssetType: "CHR", ResourceID: "resource-character",
		Status: model.ManjuAssetReferenceStatusReady, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "regenerate-p2-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)

	detail, err := f.service.RegenerateAssetRun(f.ownerID, f.projectID, baseRunID, manju.AssetRegenerationInput{})
	if err != nil {
		t.Fatalf("重新生成 P2 失败: %v", err)
	}
	if detail.Run.ID == baseRunID {
		t.Fatal("重新生成 P2 必须建立新任务，不能覆盖旧任务")
	}
	if detail.Run.Status != model.ManjuAssetRunStatusActive ||
		!detail.Run.ReuseAssets ||
		detail.Run.BaseAssetRunID != baseRunID ||
		detail.Run.InheritedAssetCount != 1 {
		t.Fatalf("P2 默认必须复用旧资产库: %+v", detail.Run)
	}
	if len(detail.References) != 1 || detail.References[0].ResourceID != "resource-character" {
		t.Fatalf("P2 默认必须继承已上传资产图: %+v", detail.References)
	}
	if extraction := findAssetStage(t, detail, manju.AssetStageExtraction); extraction.TaskID != task.ID {
		t.Fatalf("P2 重新生成必须启动新的实体提取任务: %+v", extraction)
	}
}

func TestRegenerateAssetRunCanDisableAssetReuse(t *testing.T) {
	f := newFixture(t)
	baseRunID := prepareCompletedProductionAsset(t, f, "M2")
	now := time.Now()
	if err := f.db.Create(&model.ManjuAssetReference{
		ID: "regenerate-fresh-reference", ProjectID: f.projectID, RunID: baseRunID,
		Code: "CHR-林澈", AssetType: "CHR", ResourceID: "resource-character",
		Status: model.ManjuAssetReferenceStatusReady, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "regenerate-fresh-p2-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)
	reuseAssets := false

	detail, err := f.service.RegenerateAssetRun(f.ownerID, f.projectID, baseRunID, manju.AssetRegenerationInput{
		ReuseAssets: &reuseAssets,
	})
	if err != nil {
		t.Fatalf("创建不复用资产的 P2 失败: %v", err)
	}
	if detail.Run.ReuseAssets || detail.Run.BaseAssetRunID != "" || detail.Run.InheritedAssetCount != 0 {
		t.Fatalf("关闭复用后不得继承旧资产库: %+v", detail.Run)
	}
	if len(detail.References) != 0 {
		t.Fatalf("关闭复用后不得复制参考图: %+v", detail.References)
	}
}

func TestRegenerateAssetArtifactReusesCurrentRunAndKeepsUpstream(t *testing.T) {
	f := newFixture(t)
	runID := prepareCompletedProductionAsset(t, f, "M1")
	task := &model.Task{
		ID: "regenerate-p2-artifact-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)

	detail, err := f.service.RegenerateAssetArtifact(
		f.ownerID,
		f.projectID,
		runID,
		manju.AssetStageCompile,
		manju.AssetStageRunInput{Instruction: "只重新生成资产交付编译工件"},
	)
	if err != nil {
		t.Fatalf("revive P2 artifact failed: %v", err)
	}
	if detail.Run.ID != runID || detail.Run.Status != model.ManjuAssetRunStatusActive ||
		detail.Run.CurrentStage != manju.AssetStageCompile {
		t.Fatalf("P2 artifact regeneration must stay in the current run: %+v", detail.Run)
	}
	compile := findAssetStage(t, detail, manju.AssetStageCompile)
	if compile.Status != model.ManjuAssetStageStatusInProgress || compile.ArtifactID != "" || compile.TaskID == "" {
		t.Fatalf("only P2 compile artifact must be regenerated: %+v", compile)
	}
}

func TestRegenerateScriptArtifactInvalidatesExistingP2AndP3Runs(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	assetRun, err := f.service.AssetRunDetail(f.ownerID, f.projectID, assetRunID)
	if err != nil {
		t.Fatal(err)
	}
	production, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "regenerate-p1-artifact-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)

	detail, err := f.service.RegenerateScriptArtifact(
		f.ownerID,
		f.projectID,
		assetRun.Run.ScriptRunID,
		manju.ScriptStageIPDevelopment,
		manju.ScriptStageRunInput{Instruction: "只重新生成 IP 与角色开发工件"},
	)
	if err != nil {
		t.Fatalf("revive P1 artifact failed: %v", err)
	}
	if detail.Run.ID != assetRun.Run.ScriptRunID || detail.Run.Status != model.ManjuScriptRunStatusActive {
		t.Fatalf("P1 artifact regeneration must stay in the current run: %+v", detail.Run)
	}
	target := findScriptStage(t, detail, manju.ScriptStageIPDevelopment)
	if target.Status != model.ManjuScriptStageStatusInProgress || target.ArtifactID != "" || target.TaskID == "" {
		t.Fatalf("only the target P1 artifact must be regenerated: %+v", target)
	}
	if downstream := findScriptStage(t, detail, manju.ScriptStageScriptDraft); downstream.Status != model.ManjuScriptStageStatusStale {
		t.Fatalf("later P1 artifacts must wait for the new target artifact: %+v", downstream)
	}
	var artifactCount int64
	if err := f.db.Model(&model.ManjuScriptArtifact{}).
		Where("run_id = ?", assetRun.Run.ScriptRunID).
		Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if artifactCount != 2 {
		t.Fatalf("historical P1 artifacts must remain intact, got %d", artifactCount)
	}

	var storedAssetRun model.ManjuAssetRun
	if err := f.db.First(&storedAssetRun, "id = ?", assetRunID).Error; err != nil {
		t.Fatal(err)
	}
	if storedAssetRun.Status != model.ManjuAssetRunStatusSuperseded {
		t.Fatalf("P1 artifact regeneration must retire existing P2 consumers, got %s", storedAssetRun.Status)
	}
	var storedProduction model.ManjuProductionRun
	if err := f.db.First(&storedProduction, "id = ?", production.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedProduction.Status != model.ManjuProductionRunStatusSuperseded {
		t.Fatalf("P1 artifact regeneration must retire existing P3 consumers, got %s", storedProduction.Status)
	}
}

func TestRegenerateAssetArtifactInvalidatesExistingProductionRun(t *testing.T) {
	f := newFixture(t)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	production, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "regenerate-p2-consumer-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: []*model.Task{task}}).create)

	if _, err := f.service.RegenerateAssetArtifact(
		f.ownerID,
		f.projectID,
		assetRunID,
		manju.AssetStageCompile,
		manju.AssetStageRunInput{Instruction: "只重新生成资产交付编译工件"},
	); err != nil {
		t.Fatalf("revive P2 artifact failed: %v", err)
	}

	var storedProduction model.ManjuProductionRun
	if err := f.db.First(&storedProduction, "id = ?", production.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedProduction.Status != model.ManjuProductionRunStatusSuperseded {
		t.Fatalf("P2 artifact regeneration must retire existing P3 consumers, got %s", storedProduction.Status)
	}
}

func TestRegenerateP3StartsNewRunAndPreservesHistoricalMedia(t *testing.T) {
	f := newFixture(t)
	oldRunID, oldRun := prepareRegenerableProductionRun(t, f)

	detail, err := f.service.RegenerateProductionStage(f.ownerID, f.projectID, oldRunID, manju.ProductionStageStoryboard)
	if err != nil {
		t.Fatalf("重新生成 P3 失败: %v", err)
	}
	if detail.Run.ID == oldRunID {
		t.Fatal("重新生成 P3 必须建立新的生产链版本")
	}
	if detail.Run.Status != model.ManjuProductionRunStatusActive || detail.Run.CurrentStage != manju.ProductionStageStoryboard {
		t.Fatalf("新生产链必须从 P3 重新开始: %+v", detail.Run)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageStoryboard); stage.Status != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("P3 必须重新进入可执行状态: %+v", stage)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageFeed); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("重新生成 P3 后 P4 必须失效并等待: %+v", stage)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageQC); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("重新生成 P3 后 P5 必须失效并等待: %+v", stage)
	}
	decision := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
	if decision.Status != model.ManjuProductionStageStatusLocked || decision.ArtifactID == "" {
		t.Fatalf("新的 P3 版本必须承载已锁定的 P0B: %+v", decision)
	}
	if len(detail.Artifacts) != 1 || detail.Artifacts[0].Kind != model.ManjuProductionArtifactEmotionAnchor ||
		len(detail.Images) != 0 || len(detail.Videos) != 0 {
		t.Fatalf("新的 P3 版本只能继承 P0B，不得混入旧 P3/P4 工件或媒体: %+v", detail)
	}

	var oldRunAfter model.ManjuProductionRun
	if err := f.db.First(&oldRunAfter, "id = ?", oldRunID).Error; err != nil {
		t.Fatal(err)
	}
	if oldRunAfter.Status != "SUPERSEDED" {
		t.Fatalf("旧生产链必须保留并标记 SUPERSEDED，得到 %s", oldRunAfter.Status)
	}
	if countModels[model.ManjuProductionArtifact](t, f, oldRunID) != 4 ||
		countModels[model.ManjuProductionImage](t, f, oldRunID) != 1 ||
		countModels[model.ManjuProductionVideo](t, f, oldRunID) != 1 {
		t.Fatalf("旧生产链的工件、图片和视频必须完整保留")
	}

	oldDetail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, oldRunID)
	if err != nil {
		t.Fatalf("读取旧生产链失败: %v", err)
	}
	if len(oldDetail.Artifacts) != 4 || len(oldDetail.Images) != 1 || len(oldDetail.Videos) != 1 {
		t.Fatalf("旧生产链历史读取不完整: %+v", oldDetail)
	}
	if got := findProductionStage(t, oldDetail, manju.ProductionStageStoryboard).Status; got != model.ManjuProductionStageStatusLocked {
		t.Fatalf("旧 P3 锁定状态被破坏: %s", got)
	}
	if oldRun.Run.Status != model.ManjuProductionRunStatusActive {
		t.Fatalf("测试准备错误，旧生产链应为 ACTIVE: %+v", oldRun)
	}
}

func TestRegenerateP4ClonesLockedP3AndInvalidatesP5(t *testing.T) {
	f := newFixture(t)
	oldRunID, _ := prepareRegenerableProductionRun(t, f)

	detail, err := f.service.RegenerateProductionStage(f.ownerID, f.projectID, oldRunID, manju.ProductionStageFeed)
	if err != nil {
		t.Fatalf("重新生成 P4 失败: %v", err)
	}
	if detail.Run.ID == oldRunID || detail.Run.Status != model.ManjuProductionRunStatusActive || detail.Run.CurrentStage != manju.ProductionStageFeed {
		t.Fatalf("新生产链必须从 P4 重新开始: %+v", detail.Run)
	}
	decision := findProductionStage(t, detail, manju.ProductionStageDecisionAnchor)
	if decision.Status != model.ManjuProductionStageStatusLocked || decision.ArtifactID == "" {
		t.Fatalf("P4 重新生成必须承载已锁定的 P0B: %+v", decision)
	}
	if artifact := findProductionArtifact(t, detail, decision.ArtifactID); artifact.Kind != model.ManjuProductionArtifactEmotionAnchor {
		t.Fatalf("P0B 锁定工件必须复制到新生产链: %+v", artifact)
	}
	p3 := findProductionStage(t, detail, manju.ProductionStageStoryboard)
	if p3.Status != model.ManjuProductionStageStatusLocked || p3.ArtifactID == "" {
		t.Fatalf("P4 重新生成必须承载已锁定的 P3: %+v", p3)
	}
	if artifact := findProductionArtifact(t, detail, p3.ArtifactID); artifact.Content == "" || artifact.Version != 1 {
		t.Fatalf("P3 锁定工件必须复制到新生产链: %+v", artifact)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageFeed); stage.Status != model.ManjuProductionStageStatusInProgress {
		t.Fatalf("P4 必须重新进入可执行状态: %+v", stage)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageQC); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("重新生成 P4 后 P5 必须失效并等待: %+v", stage)
	}
	if len(detail.Images) != 1 || detail.Images[0].ResourceID != "resource-regenerate-storyboard" {
		t.Fatalf("已锁定故事板图必须复制引用到新生产链: %+v", detail.Images)
	}
	if len(detail.Videos) != 0 {
		t.Fatalf("新生产链不得混入旧 P4 视频: %+v", detail.Videos)
	}
	if countModels[model.ManjuProductionVideo](t, f, oldRunID) != 1 {
		t.Fatal("旧 P4 视频必须保留")
	}
}

func TestRegenerateP3ArtifactReusesCurrentProductionRun(t *testing.T) {
	f := newFixture(t)
	runID, _ := prepareRegenerableProductionRun(t, f)
	task := &model.Task{
		ID: "regenerate-p3-artifact-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)

	detail, err := f.service.RegenerateProductionArtifact(
		f.ownerID,
		f.projectID,
		runID,
		manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{Instruction: "只重新生成 P3 工件"},
	)
	if err != nil {
		t.Fatalf("revive P3 artifact failed: %v", err)
	}
	if detail.Run.ID != runID {
		t.Fatalf("artifact regeneration must not create a new run: got=%s want=%s", detail.Run.ID, runID)
	}
	if detail.Run.Status != model.ManjuProductionRunStatusActive || detail.Run.CurrentStage != manju.ProductionStageStoryboard {
		t.Fatalf("run must remain active on P3: %+v", detail.Run)
	}
	p3 := findProductionStage(t, detail, manju.ProductionStageStoryboard)
	if p3.Status != model.ManjuProductionStageStatusInProgress || p3.ArtifactID != "" || p3.TaskID == "" {
		t.Fatalf("P3 artifact must be the only target regenerated: %+v", p3)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageFeed); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("P4 must wait for the regenerated P3 artifact: %+v", stage)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageQC); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("P5 must wait for the regenerated P3 artifact: %+v", stage)
	}
}

func TestRegenerateP4ArtifactReusesCurrentProductionRunAndLockedP3(t *testing.T) {
	f := newFixture(t)
	runID, _ := prepareRegenerableProductionRun(t, f)
	task := &model.Task{
		ID: "regenerate-p4-artifact-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)

	detail, err := f.service.RegenerateProductionArtifact(
		f.ownerID,
		f.projectID,
		runID,
		manju.ProductionStageFeed,
		manju.ProductionStageRunInput{},
	)
	if err != nil {
		t.Fatalf("revive P4 artifact failed: %v", err)
	}
	if detail.Run.ID != runID || detail.Run.CurrentStage != manju.ProductionStageFeed {
		t.Fatalf("P4 artifact regeneration must stay in the current run: %+v", detail.Run)
	}
	p3 := findProductionStage(t, detail, manju.ProductionStageStoryboard)
	if p3.Status != model.ManjuProductionStageStatusLocked || p3.ArtifactID == "" {
		t.Fatalf("P4 artifact regeneration must preserve locked P3: %+v", p3)
	}
	p4 := findProductionStage(t, detail, manju.ProductionStageFeed)
	if p4.Status != model.ManjuProductionStageStatusInProgress || p4.ArtifactID != "" {
		t.Fatalf("P4 artifact must be regenerated in place: %+v", p4)
	}
	if stage := findProductionStage(t, detail, manju.ProductionStageQC); stage.Status != model.ManjuProductionStageStatusNotStarted {
		t.Fatalf("P5 must wait for the regenerated P4 artifact: %+v", stage)
	}
}

func TestRegenerateProductionStageRejectsLiveTask(t *testing.T) {
	f := newFixture(t)
	oldRunID, _ := prepareRegenerableProductionRun(t, f)
	if err := f.db.Create(&model.Task{
		ID: "p4-live-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND key = ?", oldRunID, manju.ProductionStageFeed).
		Update("task_id", "p4-live-task").Error; err != nil {
		t.Fatal(err)
	}

	_, err := f.service.RegenerateProductionStage(f.ownerID, f.projectID, oldRunID, manju.ProductionStageFeed)
	if !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("阶段任务仍在执行时不得重新生成，得到 %v", err)
	}
}

func prepareRegenerableProductionRun(t *testing.T, f fixture) (string, *manju.ProductionRunDetail) {
	t.Helper()
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	now := time.Now()
	storyboardArtifact := &model.ManjuProductionArtifact{
		ID: "artifact-regenerate-p3", ProjectID: f.projectID, RunID: run.Run.ID,
		StageID: findProductionStage(t, run, manju.ProductionStageStoryboard).ID,
		Kind:    model.ManjuProductionArtifactStoryboard, Version: 1,
		Content: validStoryboardArtifact(), ContentHash: "hash-regenerate-p3",
		CreatedBy: f.ownerID, CreatedAt: now,
	}
	feedArtifact := &model.ManjuProductionArtifact{
		ID: "artifact-regenerate-p4", ProjectID: f.projectID, RunID: run.Run.ID,
		StageID: findProductionStage(t, run, manju.ProductionStageFeed).ID,
		Kind:    model.ManjuProductionArtifactFeed, Version: 1,
		Content: validFeedArtifact(), ContentHash: "hash-regenerate-p4",
		CreatedBy: f.ownerID, CreatedAt: now,
	}
	qcArtifact := &model.ManjuProductionArtifact{
		ID: "artifact-regenerate-p5", ProjectID: f.projectID, RunID: run.Run.ID,
		StageID: findProductionStage(t, run, manju.ProductionStageQC).ID,
		Kind:    model.ManjuProductionArtifactQC, Version: 1,
		Content: validQCHandoffArtifact("resource-regenerate-video"), ContentHash: "hash-regenerate-p5",
		CreatedBy: f.ownerID, CreatedAt: now,
	}
	if err := f.db.Create(&[]model.ManjuProductionArtifact{*storyboardArtifact, *feedArtifact, *qcArtifact}).Error; err != nil {
		t.Fatal(err)
	}
	p3Stage := findProductionStage(t, run, manju.ProductionStageStoryboard)
	p4Stage := findProductionStage(t, run, manju.ProductionStageFeed)
	p5Stage := findProductionStage(t, run, manju.ProductionStageQC)
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", p3Stage.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusLocked, "version": 1, "artifact_id": storyboardArtifact.ID,
		"confirmed_by": f.ownerID, "confirmed_at": now, "completed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", p4Stage.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusLocked, "version": 1, "artifact_id": feedArtifact.ID,
		"confirmed_by": f.ownerID, "confirmed_at": now, "completed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).Where("id = ?", p5Stage.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusAwaitingConfirmation, "version": 1, "artifact_id": qcArtifact.ID,
		"completed_at": now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&model.ManjuProductionRun{}).Where("id = ?", run.Run.ID).Update("current_stage", manju.ProductionStageQC).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProductionImage{
		ID: "image-regenerate-p3", ProjectID: f.projectID, RunID: run.Run.ID, StageID: p3Stage.ID,
		Position: 1, SegmentCode: "段1", ShotLabel: "1-4", Prompt: "storyboard",
		Attempt: 1, Status: model.ManjuProductionImageStatusReady,
		ResourceID: "resource-regenerate-storyboard", CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProductionVideo{
		ID: "video-regenerate-p4", ProjectID: f.projectID, RunID: run.Run.ID, StageID: p4Stage.ID,
		Position: 1, SegmentCode: "段1", DurationSeconds: 5, Prompt: "video",
		Attempt: 1, Status: model.ManjuProductionVideoStatusReady,
		ResourceID: "resource-regenerate-video", CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return run.Run.ID, detail
}

func countModels[T any](t *testing.T, f fixture, runID string) int64 {
	t.Helper()
	var count int64
	var item T
	if err := f.db.Model(&item).Where("run_id = ?", runID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func findProductionArtifact(t *testing.T, detail *manju.ProductionRunDetail, artifactID string) manju.ProductionArtifactView {
	t.Helper()
	for _, artifact := range detail.Artifacts {
		if artifact.ID == artifactID {
			return artifact
		}
	}
	t.Fatalf("缺少生产工件 %s: %+v", artifactID, detail.Artifacts)
	return manju.ProductionArtifactView{}
}
