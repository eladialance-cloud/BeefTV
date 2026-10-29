package manju_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestDeliveryModeBoardProducesMarkdownSourceAndHTMLBoard(t *testing.T) {
	f := newFixture(t)
	detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryBoard)
	deliverables, err := f.service.UserDeliverables(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	assertDeliverableKinds(t, deliverables, "markdown", "board", "manifest")
	assertDeliverableContent(t, deliverables, "board", "<!doctype html>")
	assertDeliverableContent(t, deliverables, "markdown", "# VisiDeep AI 漫剧全流程交付归档")

	var rows []model.ManjuDeliverable
	if err := f.db.Where("production_run_id = ?", detail.Run.ID).Order("kind ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("persisted deliverables = %d, want 3: %+v", len(rows), rows)
	}
	for _, row := range rows {
		sum := sha256.Sum256([]byte(row.Content))
		if row.ContentHash != strings.ToLower(strings.TrimSpace(row.ContentHash)) || row.ContentHash == "" {
			t.Fatalf("%s content hash is not lowercase SHA-256: %q", row.Kind, row.ContentHash)
		}
		if row.ContentHash != fmtHash(sum) {
			t.Fatalf("%s content hash does not match content", row.Kind)
		}
		if row.FileName == "" || row.MimeType == "" || row.ProjectID != f.projectID || row.LockRevision != detail.Run.LockRevision {
			t.Fatalf("deliverable metadata is incomplete: %+v", row)
		}
	}
}

func TestDeliveryModeMarkdownDoesNotPretendBoardExists(t *testing.T) {
	f := newFixture(t)
	detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryMarkdown)
	deliverables, err := f.service.UserDeliverables(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	assertDeliverableKinds(t, deliverables, "markdown", "manifest")
	assertNoDeliverableKind(t, deliverables, "board")

	var boardCount int64
	if err := f.db.Model(&model.ManjuDeliverable{}).
		Where("production_run_id = ? AND kind = ?", detail.Run.ID, "board").
		Count(&boardCount).Error; err != nil {
		t.Fatal(err)
	}
	if boardCount != 0 {
		t.Fatalf("markdown-only delivery persisted %d board rows", boardCount)
	}
}

func TestDeliveryModeBothProducesMarkdownAndBoard(t *testing.T) {
	f := newFixture(t)
	completeProjectThroughP5ForTest(t, f, manju.DeliveryBoth)
	deliverables, err := f.service.UserDeliverables(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	assertDeliverableKinds(t, deliverables, "markdown", "board", "manifest")
	assertDeliverableContent(t, deliverables, "board", "<!doctype html>")
}

func TestDeliveryRowsRemainImmutableWhenSameRunIsConfirmedAgain(t *testing.T) {
	f := newFixture(t)
	detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryBoth)
	before, err := f.service.UserDeliverables(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmProductionStage(
		f.ownerID,
		f.projectID,
		detail.Run.ID,
		manju.ProductionStageQC,
		manju.ProductionConfirmInput{Note: "重复确认"},
	); err != nil {
		t.Fatalf("completed run confirmation must be idempotent: %v", err)
	}
	after, err := f.service.UserDeliverables(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("reconfirm changed deliverable count: before=%d after=%d", len(before), len(after))
	}
	for index := range before {
		if before[index] != after[index] {
			t.Fatalf("reconfirm rewrote immutable deliverable:\nbefore=%+v\nafter=%+v", before[index], after[index])
		}
	}
}

func TestProductionDeliverableKindIsUniquePerRun(t *testing.T) {
	f := newFixture(t)
	detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryMarkdown)
	row := model.ManjuDeliverable{
		ID: "duplicate-deliverable", ProjectID: f.projectID, LockRevision: detail.Run.LockRevision,
		ProductionRunID: detail.Run.ID, Kind: "markdown", FileName: "重复.md",
		MimeType: "text/markdown", Content: "重复", ContentHash: strings.Repeat("0", 64),
	}
	if err := f.db.Create(&row).Error; err == nil {
		t.Fatal("duplicate production-run/kind deliverable must violate the unique constraint")
	}
}

func completeProjectThroughP5ForTest(t *testing.T, f fixture, deliveryMode string) *manju.ProductionRunDetail {
	t.Helper()
	assetRunID := prepareCompletedDeliveryAsset(t, f, deliveryMode)
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)
	tasks := []*model.Task{
		{ID: "delivery-p3-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "delivery-p3-image-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued},
		{ID: "delivery-p4-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "delivery-p4-video-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_video", Status: model.TaskStatusQueued},
		{ID: "delivery-p5-task", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: tasks}).create)

	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[0].ID, validStoryboardArtifactRequiringImage())
	startedImages, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(startedImages.Images) != 1 {
		t.Fatalf("expected one storyboard image task: %+v", startedImages.Images)
	}
	succeedStoryboardImageTask(t, f, tasks[1].ID, "delivery-storyboard-image")
	if _, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard, manju.ProductionConfirmInput{}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[2].ID, validFeedArtifact())
	feedStage := findProductionStage(
		t,
		mustProductionDetail(t, f, run.Run.ID),
		manju.ProductionStageFeed,
	)
	setP4MachineCheckProofForTest(t, f, run.Run.ID, lockedP3ArtifactIDForTest(t, f, run.Run.ID))
	previsArtifact := &model.ManjuProductionArtifact{
		ID: "delivery-previs-artifact", ProjectID: f.projectID, RunID: run.Run.ID, StageID: feedStage.ID,
		Kind: model.ManjuProductionArtifactPrevis, Version: 1, Content: validP45Artifact(),
		ContentHash: "delivery-previs-artifact", CreatedBy: f.ownerID, CreatedAt: time.Now(),
	}
	if err := f.db.Create(previsArtifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProductionPrevis{
		ID: "delivery-previs-row", ProjectID: f.projectID, RunID: run.Run.ID, StageID: feedStage.ID,
		ArtifactID: previsArtifact.ID, Status: model.ManjuProductionPrevisStatusReady, Version: 1,
		CreatedBy: f.ownerID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	startedVideos, err := f.service.StartProductionVideos(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(startedVideos.Videos) != 1 {
		t.Fatalf("expected one video task: %+v", startedVideos.Videos)
	}
	succeedProductionVideoTask(t, f, tasks[3].ID, "delivery-video")
	if _, err := f.service.ConfirmProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageFeed, manju.ProductionConfirmInput{}); err != nil {
		t.Fatal(err)
	}

	if _, err := f.service.RunProductionStage(f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageQC, manju.ProductionStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	succeedManjuTask(t, f, tasks[4].ID, validQCHandoffArtifact("delivery-video"))
	return mustConfirmProduction(t, f, run.Run.ID, manju.ProductionStageQC)
}

func prepareCompletedDeliveryAsset(t *testing.T, f fixture, deliveryMode string) string {
	t.Helper()
	lock := validLock()
	lock.AssetMode = manju.AssetModeM1Value
	lock.DeliveryMode = deliveryMode
	receipt, err := f.service.SaveLock(f.ownerID, f.projectID, lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "delivery-binding", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
		ImageChannelID: "channel-image", ImageModelKey: "image-pro",
		VideoChannelID: "channel-video", VideoModelKey: "video-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	scriptRun := &model.ManjuScriptRun{
		ID: "delivery-script-run", ProjectID: f.projectID, Branch: manju.ScriptBranchAdaptation,
		Mode: manju.ScriptModeStepwise, Status: model.ManjuScriptRunStatusCompleted,
		CurrentStage: "", LockRevision: receipt.Revision, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(scriptRun).Error; err != nil {
		t.Fatal(err)
	}
	stages := []model.ManjuScriptStage{
		{
			ID: "delivery-script-ip-stage", RunID: scriptRun.ID, Key: manju.ScriptStageIPDevelopment,
			Title: "IP 与角色开发", Position: 1, Status: manju.ScriptStageStatusLocked, Version: 1,
		},
		{
			ID: "delivery-script-draft-stage", RunID: scriptRun.ID, Key: manju.ScriptStageScriptDraft,
			Title: "正式剧本", Position: 2, Status: manju.ScriptStageStatusLocked, Version: 1,
		},
	}
	if err := f.db.Create(&stages).Error; err != nil {
		t.Fatal(err)
	}
	scriptArtifacts := []model.ManjuScriptArtifact{
		{
			ID: "delivery-script-ip", ProjectID: f.projectID, RunID: scriptRun.ID,
			StageID: stages[0].ID, Kind: "IP_DEVELOPMENT.md", Version: 1,
			Content: "LOCKED_IP_FACT 林澈是档案修复师。", ContentHash: "delivery-locked-ip", CreatedBy: f.ownerID,
		},
		{
			ID: "delivery-script-draft", ProjectID: f.projectID, RunID: scriptRun.ID,
			StageID: stages[1].ID, Kind: "SCRIPT_DRAFT.md", Version: 1,
			Content: "LOCKED_SCRIPT_FACT 林澈推开旧宅的门。", ContentHash: "delivery-locked-script", CreatedBy: f.ownerID,
		},
	}
	if err := f.db.Create(&scriptArtifacts).Error; err != nil {
		t.Fatal(err)
	}
	for index := range stages {
		if err := f.db.Model(&model.ManjuScriptStage{}).
			Where("id = ?", stages[index].ID).
			Update("artifact_id", scriptArtifacts[index].ID).Error; err != nil {
			t.Fatal(err)
		}
	}

	assetRun := &model.ManjuAssetRun{
		ID: "delivery-asset-run", ProjectID: f.projectID, ScriptRunID: scriptRun.ID,
		AssetMode: manju.AssetModeM1Value, Status: model.ManjuAssetRunStatusCompleted, CurrentStage: "",
		LockRevision: receipt.Revision, CreatedBy: f.ownerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(assetRun).Error; err != nil {
		t.Fatal(err)
	}
	assetStage := &model.ManjuAssetStage{
		ID: "delivery-asset-compile-stage", RunID: assetRun.ID, Key: manju.AssetStageCompile,
		Title: "资产交付编译", Position: 3, Status: model.ManjuAssetStageStatusLocked,
		Version: 1, ArtifactID: "delivery-asset-compile", CreatedAt: now, UpdatedAt: now,
	}
	if err := f.db.Create(assetStage).Error; err != nil {
		t.Fatal(err)
	}
	assetArtifact := &model.ManjuAssetArtifact{
		ID: assetStage.ArtifactID, ProjectID: f.projectID, RunID: assetRun.ID, StageID: assetStage.ID,
		Kind: model.ManjuAssetArtifactM1, Version: 1,
		Content: "LOCKED_ASSET_FACT\n图1 = CHR-林澈\nSCN-旧宅。", ContentHash: "delivery-locked-asset",
		CreatedBy: f.ownerID, CreatedAt: now,
	}
	if err := f.db.Create(assetArtifact).Error; err != nil {
		t.Fatal(err)
	}
	stateJSON, err := json.Marshal(map[string]any{
		"schema_version": "7.4.23",
		"locks": map[string]any{
			"asset_mode": manju.AssetModeM1Value,
			"model":      "2.0",
			"aspect":     "9:16",
		},
		"characters": map[string]any{
			"林澈": map[string]any{"id": "CHR-林澈", "fig_no": 1, "voice_lock": "清亮青年声"},
		},
		"scenes": map[string]any{
			"旧宅": map[string]any{
				"id": "SCN-旧宅", "fig_no": 2, "coords": "木门在画面右侧，门内为纵深走廊",
				"set_states": map[string]any{"default": "清晨薄雾"}, "media_layer": "冷白薄尘",
				"lighting": "清晨冷光从门外左上方照入",
			},
		},
		"props":    map[string]any{},
		"segments": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectStateSnapshot{
		ID: "delivery-asset-state", ProjectID: f.projectID, LockRevision: receipt.Revision,
		StageKey: "asset:asset_compile", ArtifactID: assetArtifact.ID, SchemaVersion: "7.4.23",
		StateJSON: string(stateJSON), ContentHash: "delivery-locked-asset-state",
		CreatedBy: f.ownerID, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return assetRun.ID
}

func mustProductionDetail(t *testing.T, f fixture, runID string) *manju.ProductionRunDetail {
	t.Helper()
	detail, err := f.service.ProductionRunDetail(f.ownerID, f.projectID, runID)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

func mustConfirmProduction(t *testing.T, f fixture, runID, stageKey string) *manju.ProductionRunDetail {
	t.Helper()
	detail, err := f.service.ConfirmProductionStage(
		f.ownerID,
		f.projectID,
		runID,
		stageKey,
		manju.ProductionConfirmInput{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

func assertDeliverableKinds(t *testing.T, deliverables []manju.UserDeliverableView, want ...string) {
	t.Helper()
	if len(deliverables) != len(want) {
		t.Fatalf("deliverable count = %d, want %d: %+v", len(deliverables), len(want), deliverables)
	}
	for index, kind := range want {
		if deliverables[index].Kind != kind {
			t.Fatalf("deliverable[%d].kind = %q, want %q: %+v", index, deliverables[index].Kind, kind, deliverables)
		}
	}
}

func assertDeliverableContent(t *testing.T, deliverables []manju.UserDeliverableView, kind, contains string) {
	t.Helper()
	for _, deliverable := range deliverables {
		if deliverable.Kind == kind {
			if !strings.Contains(strings.ToLower(deliverable.Content), strings.ToLower(contains)) {
				t.Fatalf("%s deliverable does not contain %q:\n%s", kind, contains, deliverable.Content)
			}
			return
		}
	}
	t.Fatalf("missing %s deliverable: %+v", kind, deliverables)
}

func assertNoDeliverableKind(t *testing.T, deliverables []manju.UserDeliverableView, kind string) {
	t.Helper()
	for _, deliverable := range deliverables {
		if deliverable.Kind == kind {
			t.Fatalf("unexpected %s deliverable: %+v", kind, deliverable)
		}
	}
}

func fmtHash(sum [sha256.Size]byte) string {
	return hex.EncodeToString(sum[:])
}
