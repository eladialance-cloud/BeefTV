package manju_test

import (
	"errors"
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
		{"P3 before locked P0B", runP3BeforeP0B},
		{"partial storyboard images", confirmP3WithOneMissingImage},
		{"missing P4 proof", confirmP4WithIncompleteMachineCheckProof},
		{"missing dialogue diff", startP4WithMissingDialogue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(t, newFixture(t)); err == nil {
				t.Fatal("gate must fail closed")
			}
		})
	}
	t.Run("board delivery materialization boundaries", exerciseBoardDeliveryMaterializationBoundaries)
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

func runP3BeforeP0B(t *testing.T, f fixture) error {
	t.Helper()
	assetRunID := prepareCompletedProductionAsset(t, f, manju.AssetModeM1Value)
	run, err := f.service.CreateProductionRun(f.ownerID, f.projectID, manju.ProductionRunInput{AssetRunID: assetRunID})
	if err != nil {
		t.Fatalf("create production run: %v", err)
	}

	if err := f.db.Model(&model.ManjuProductionRun{}).
		Where("id = ?", run.Run.ID).
		Update("current_stage", manju.ProductionStageStoryboard).Error; err != nil {
		t.Fatalf("move active production run to P3: %v", err)
	}
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageStoryboard).
		Update("status", model.ManjuProductionStageStatusInProgress).Error; err != nil {
		t.Fatalf("activate P3 stage: %v", err)
	}
	task := &model.Task{
		ID: "gate-bypass-p0b-task", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatalf("create task that must not be requested: %v", err)
	}
	creator := &sequentialScriptTaskCreator{tasks: []*model.Task{task}}
	f.service.SetScriptTaskCreator(creator.create)
	var activeRun model.ManjuProductionRun
	if err := f.db.First(&activeRun, "id = ?", run.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if activeRun.Status != model.ManjuProductionRunStatusActive ||
		activeRun.CurrentStage != manju.ProductionStageStoryboard {
		t.Fatalf("fixture must reach an active P3 current stage: %+v", activeRun)
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
	if err.Error() != manju.ErrProductionStageNotReady.Error() {
		t.Fatalf("P3 must fail at ensureProductionStageGate, got message %q", err)
	}
	if len(creator.requests) != 0 {
		t.Fatalf("P3 task must not be created while P0B is unlocked: %+v", creator.requests)
	}
	var decision model.ManjuProductionStage
	if err := f.db.First(&decision, "run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageDecisionAnchor).Error; err != nil {
		t.Fatal(err)
	}
	if decision.Status == model.ManjuProductionStageStatusLocked || decision.ArtifactID != "" {
		t.Fatalf("fixture must keep P0B unlocked without an artifact: %+v", decision)
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

func confirmP4WithIncompleteMachineCheckProof(t *testing.T, f fixture) error {
	t.Helper()
	run := prepareP4AwaitingConfirmation(t, f)
	apparentPass := `{"machineCheckProof":{"version":"V7.4.26","status":"PASS"},"machineCheckNote":"历史 PASS"}`
	if err := f.db.Model(&model.ManjuProductionStage{}).
		Where("run_id = ? AND key = ?", run.Run.ID, manju.ProductionStageFeed).
		Update("input_json", apparentPass).Error; err != nil {
		t.Fatalf("write incomplete apparent PASS proof: %v", err)
	}

	_, err := f.service.ConfirmProductionStage(
		f.ownerID,
		f.projectID,
		run.Run.ID,
		manju.ProductionStageFeed,
		manju.ProductionConfirmInput{Note: "尝试使用残缺 PASS 凭证"},
	)
	if !errors.Is(err, manju.ErrProductionMachineCheckProofInvalid) {
		t.Fatalf("P4 confirmation with incomplete PASS proof must fail with ErrProductionMachineCheckProofInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "pipeline") {
		t.Fatalf("P4 must reject the missing pipeline proof step, got %v", err)
	}
	detail, detailErr := f.service.ProductionRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if detailErr != nil {
		t.Fatalf("read P4 after rejected confirmation: %v", detailErr)
	}
	if got := findProductionStage(t, detail, manju.ProductionStageFeed).Status; got != model.ManjuProductionStageStatusAwaitingConfirmation {
		t.Fatalf("rejected P4 proof must leave the stage unlocked, got %s", got)
	}
	return err
}

func exerciseBoardDeliveryMaterializationBoundaries(t *testing.T) {
	t.Helper()

	t.Run("no board before P5 confirmation", func(t *testing.T) {
		f := newFixture(t)
		run := prepareP4AwaitingConfirmation(t, f)
		rows, err := f.service.ProductionRunDeliverables(f.ownerID, f.projectID, run.Run.ID)
		if err != nil {
			t.Fatalf("read pre-P5 deliverables: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("pre-P5 run exposed deliverables before materialization: %+v", rows)
		}
	})

	t.Run("markdown mode has no board", func(t *testing.T) {
		f := newFixture(t)
		detail := completeProjectThroughP5ForTest(t, f, manju.DeliveryMarkdown)
		rows, err := f.service.ProductionRunDeliverables(f.ownerID, f.projectID, detail.Run.ID)
		if err != nil {
			t.Fatalf("read markdown deliverables: %v", err)
		}
		assertGateDeliverableKindCount(t, rows, "markdown", 1)
		assertGateDeliverableKindCount(t, rows, "board", 0)
	})

	for _, mode := range []string{manju.DeliveryBoard, manju.DeliveryBoth} {
		t.Run(mode+" mode materializes one idempotent board", func(t *testing.T) {
			f := newFixture(t)
			detail := completeProjectThroughP5ForTest(t, f, mode)
			before, err := f.service.ProductionRunDeliverables(f.ownerID, f.projectID, detail.Run.ID)
			if err != nil {
				t.Fatalf("read initial %s deliverables: %v", mode, err)
			}
			assertGateDeliverableKindCount(t, before, "markdown", 1)
			assertGateDeliverableKindCount(t, before, "board", 1)
			board := gateDeliverablesByKind(before, "board")
			if !strings.HasPrefix(strings.ToLower(board[0].Content), "<!doctype html>") {
				t.Fatalf("materialized board is not official HTML")
			}

			if _, err := f.service.ConfirmProductionStage(
				f.ownerID,
				f.projectID,
				detail.Run.ID,
				manju.ProductionStageQC,
				manju.ProductionConfirmInput{Note: "重复确认交付"},
			); err != nil {
				t.Fatalf("repeat P5 confirmation must be idempotent: %v", err)
			}
			after, err := f.service.ProductionRunDeliverables(f.ownerID, f.projectID, detail.Run.ID)
			if err != nil {
				t.Fatalf("read repeated %s deliverables: %v", mode, err)
			}
			if len(after) != len(before) {
				t.Fatalf("repeat confirmation changed %s deliverable count: before=%d after=%d", mode, len(before), len(after))
			}
			for index := range before {
				if before[index] != after[index] {
					t.Fatalf("repeat confirmation rewrote %s deliverable:\nbefore=%+v\nafter=%+v", mode, before[index], after[index])
				}
			}
			assertGateDeliverableKindCount(t, after, "board", 1)
		})
	}
}

func assertGateDeliverableKindCount(
	t *testing.T,
	rows []manju.UserDeliverableView,
	kind string,
	want int,
) {
	t.Helper()
	if got := len(gateDeliverablesByKind(rows, kind)); got != want {
		t.Fatalf("%s deliverable count = %d, want %d: %+v", kind, got, want, rows)
	}
}

func gateDeliverablesByKind(rows []manju.UserDeliverableView, kind string) []manju.UserDeliverableView {
	result := make([]manju.UserDeliverableView, 0, len(rows))
	for _, row := range rows {
		if row.Kind == kind {
			result = append(result, row)
		}
	}
	return result
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
