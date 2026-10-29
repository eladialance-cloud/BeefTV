package manju

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestProductionStageViewMarksLegacySkippedMachineCheckNonCompliant(t *testing.T) {
	view := productionStageView(model.ManjuProductionStage{
		Key: ProductionStageFeed,
		InputJSON: `{
			"compiler": "V7.4.26 compile_feed.py",
			"machineCheckStatus": "SKIPPED_BY_USER",
			"machineCheckNote": "用户跳过官方双机检"
		}`,
	})

	if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusLegacyNonCompliant {
		t.Fatalf("machine check status = %q", view.MachineCheckStatus)
	}
	if view.MachineCheckNote != "历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。" {
		t.Fatalf("machine check note = %q", view.MachineCheckNote)
	}
}

func TestProductionStageViewDoesNotTreatMissingMachineCheckAsPass(t *testing.T) {
	view := productionStageView(model.ManjuProductionStage{
		Key:        ProductionStageFeed,
		ArtifactID: "legacy-p4-artifact",
		InputJSON:  `{"instruction":"历史 P4 任务"}`,
	})

	if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusLegacyNonCompliant {
		t.Fatalf("machine check status = %q", view.MachineCheckStatus)
	}
	if view.MachineCheckNote != "历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。" {
		t.Fatalf("machine check note = %q", view.MachineCheckNote)
	}
}

func TestProductionStageViewExposesDialogueWarnings(t *testing.T) {
	view := productionStageView(model.ManjuProductionStage{
		Key:        ProductionStageFeed,
		ArtifactID: "feed-artifact",
		InputJSON: `{
			"machineCheckStatus": "PASS",
			"machineCheckWarnings": [
				"1. [⚠ 部分覆盖 0.70] 林澈：别开门，外面有人。"
			]
		}`,
	})

	if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusPass {
		t.Fatalf("machine check status = %q", view.MachineCheckStatus)
	}
	if len(view.MachineCheckWarnings) != 1 {
		t.Fatalf("machine check warnings = %+v", view.MachineCheckWarnings)
	}
}
