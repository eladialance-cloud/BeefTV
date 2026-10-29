package manju_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestV7426AllHardGatesFailClosed(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, f fixture) error
	}{
		{"missing P0 confirmation", runSaveLockWithoutDeliveryConfirmation},
		{"missing P0B", runP1BeforeP0B},
		{"partial storyboard images", confirmP3WithOneMissingImage},
		{"missing dialogue diff", startP4WithMissingDialogue},
		{"board not materialized", readBoardBeforeCompile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(t, newFixture(t)); err == nil {
				t.Fatal("gate must fail closed")
			}
		})
	}
}

func runSaveLockWithoutDeliveryConfirmation(t *testing.T, f fixture) error {
	t.Helper()
	input := validLock()
	input.ConfirmedLocks = removeString(input.ConfirmedLocks, manju.LockDeliveryMode)

	_, err := f.service.SaveLock(f.ownerID, f.projectID, input)
	if !errors.Is(err, manju.ErrExplicitLockRequired) {
		t.Fatalf("P0 without delivery confirmation must fail with ErrExplicitLockRequired, got %v", err)
	}
	return err
}

func runP1BeforeP0B(t *testing.T, f fixture) error {
	t.Helper()
	assetRunID := prepareCompletedProductionAsset(t, f, manju.AssetModeM1Value)
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatalf("create production run: %v", err)
	}

	_, err = f.service.RunProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	)
	if !errors.Is(err, manju.ErrProductionStageNotReady) {
		t.Fatalf("P3 before locked P0B must fail with ErrProductionStageNotReady, got %v", err)
	}
	return err
}

func confirmP3WithOneMissingImage(t *testing.T, f fixture) error {
	t.Helper()
	assetRunID := prepareCompletedProductionAsset(t, f, manju.AssetModeM1Value)
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatalf("create production run: %v", err)
	}
	lockProductionP0BForTest(t, f, run.Run.ID)

	tasks := []*model.Task{
		{ID: "gate-bypass-p3-text", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued},
		{ID: "gate-bypass-storyboard-1", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued},
		{ID: "gate-bypass-storyboard-2", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_image", Status: model.TaskStatusQueued},
	}
	if err := f.db.Create(&tasks).Error; err != nil {
		t.Fatalf("create production tasks: %v", err)
	}
	f.service.SetScriptTaskCreator((&sequentialScriptTaskCreator{tasks: tasks}).create)

	if _, err := f.service.RunProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageStoryboard,
		manju.ProductionStageRunInput{},
	); err != nil {
		t.Fatalf("start P3: %v", err)
	}

	storyboard := strings.Replace(
		validMixedP3StoryboardArtifact(),
		"| 段1 | 覆盖：场1 ✅ 四宫格 | ⛔豁免（单镜过场段） | 理由：本段只有一个过门动作，动作相位已完整列入分镜总表。 |",
		"| 段1 | 覆盖：场1 ✅ 四宫格 | 4格(2×2 四宫格) | 无豁免；等待已就绪故事板图。 |",
		1,
	)
	if storyboard == validMixedP3StoryboardArtifact() {
		t.Fatal("fixture did not require a second storyboard image")
	}
	succeedManjuTask(t, f, tasks[0].ID, storyboard)

	started, err := f.service.StartProductionStoryboardImages(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatalf("start storyboard images: %v", err)
	}
	if len(started.Images) != 2 {
		t.Fatalf("partial coverage fixture must start two storyboard images, got %+v", started.Images)
	}
	succeedStoryboardImageTask(t, f, tasks[1].ID, "gate-bypass-storyboard-resource-1")

	_, err = f.service.ConfirmProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageStoryboard,
		manju.ProductionConfirmInput{},
	)
	if !errors.Is(err, manju.ErrStoryboardImagesNotReady) {
		t.Fatalf("P3 with one missing storyboard image must fail with ErrStoryboardImagesNotReady, got %v", err)
	}
	return err
}

func startP4WithMissingDialogue(t *testing.T, f fixture) error {
	t.Helper()
	run := prepareP4WithUpstreamDialogue(t, f, "林澈：别开门。")

	_, err := f.service.RunProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageFeed,
		manju.ProductionStageRunInput{},
	)
	if err == nil || !strings.Contains(err.Error(), "台词") {
		t.Fatalf("P4 with dialogue missing from the compiled feed must fail, got %v", err)
	}
	return err
}

func readBoardBeforeCompile(t *testing.T, f fixture) error {
	t.Helper()
	detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryMarkdown)
	rows, err := f.service.ProductionRunDeliverables(f.ownerID, f.projectID, detail.Run.ID)
	if err != nil {
		t.Fatalf("read persisted deliverables: %v", err)
	}
	for _, row := range rows {
		if row.Kind == "board" {
			return nil
		}
	}
	return fmt.Errorf("%w: markdown delivery must not expose a board row", manju.ErrProductionDeliverablesNotReady)
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}
