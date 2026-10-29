package manju_test

import (
	"strings"
	"testing"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

func TestManjuStageDraftPersistsImmutableVersions(t *testing.T) {
	f := newFixture(t)
	first := &model.ManjuStageDraft{
		ID: "draft-1", ProjectID: f.projectID, Phase: "P3", RunID: "run-1",
		StageID: "stage-1", Version: 1, Content: "第一版", ContentHash: "hash-1",
		ValidationJSON: `{"issues":["C14"]}`, Source: "MODEL_FAILED", CreatedBy: f.ownerID,
	}
	second := *first
	second.ID = "draft-2"
	second.Version = 2
	second.Content = "第二版"
	second.ContentHash = "hash-2"
	if err := f.db.Create(first).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	drafts, err := repository.New(f.db).ManjuStageDrafts("run-1", "stage-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 || drafts[0].Content != "第一版" || drafts[1].Content != "第二版" {
		t.Fatalf("draft versions must remain immutable: %+v", drafts)
	}
}

func TestManualP3CorrectionCreatesAwaitingArtifactAndAudit(t *testing.T) {
	f := newFixture(t)
	f.service.SetSkillValidationEnabledForTest(true)
	assetRunID := prepareCompletedProductionAsset(t, f, "M1")
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatal(err)
	}
	p3 := findProductionStage(t, run, manju.ProductionStageStoryboard)
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("id = ?", p3.ID).
		Updates(map[string]any{
			"status":      model.ManjuProductionStageStatusBlocked,
			"artifact_id": "",
			"error":       "C14 人工修正测试",
		}).Error; err != nil {
		t.Fatal(err)
	}
	content := validStoryboardArtifact() + `

<!-- MANJU_PROJECT_STATE_PATCH_JSON
{
  "segments": [{
    "seg_id": "段1",
    "scene": "旧宅",
    "asset_refs": {
      "scenes": ["旧宅"],
      "characters": ["林澈"],
      "props": []
    },
    "standing": ["林澈在门左侧"],
    "storyboard_panels": ["起始", "过程", "反应", "出口"],
    "shots": [{
      "no": 1,
      "start": 0,
      "end": 5,
      "spec": "中景 · 24mm · 侧45度 · 固定",
      "action": "画面主体：林澈抬手推门，肩部收紧",
      "sound": "木门吱呀声",
      "dialogue": "无"
    }],
    "sfx_summary": "木门吱呀声"
  }]
}
-->`
	if err := f.db.Create(&model.ManjuStageDraft{
		ID: "manual-p3-draft", ProjectID: f.projectID, Phase: "P3",
		RunID: run.Run.ID, StageID: p3.ID, Version: 1, Content: content,
		ContentHash: "draft-hash", ValidationJSON: `{"issues":["C14"]}`,
		Source: "MODEL_FAILED", CreatedBy: f.ownerID,
	}).Error; err != nil {
		t.Fatal(err)
	}

	updated, err := f.service.ManualCorrectProductionArtifact(
		f.ownerID, f.projectID, run.Run.ID, manju.ProductionStageStoryboard,
		manju.ManualArtifactCorrectionInput{Content: content, Note: "人工修正 C14"},
	)
	if err != nil {
		t.Fatal(err)
	}
	stage := findProductionStage(t, updated, manju.ProductionStageStoryboard)
	if stage.Status != model.ManjuProductionStageStatusAwaitingConfirmation || stage.ArtifactID == "" {
		t.Fatalf("人工修正通过后必须等待用户确认: %+v", stage)
	}
	if stage.Draft != nil {
		t.Fatalf("等待确认阶段不应继续显示失败草稿: %+v", stage.Draft)
	}
	var drafts []model.ManjuStageDraft
	if err := f.db.Where("run_id = ? AND stage_id = ?", run.Run.ID, p3.ID).Order("version ASC").Find(&drafts).Error; err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 || drafts[1].Source != "MANUAL_PASSED" || !strings.Contains(drafts[1].ValidationJSON, `"passed":true`) {
		t.Fatalf("人工修正必须保存通过版本草稿: %+v", drafts)
	}
	var corrections []model.ManjuArtifactCorrection
	if err := f.db.Where("run_id = ?", run.Run.ID).Find(&corrections).Error; err != nil {
		t.Fatal(err)
	}
	if len(corrections) != 1 || corrections[0].ResultArtifactID != stage.ArtifactID {
		t.Fatalf("人工修正必须保存审计记录: %+v", corrections)
	}
}
