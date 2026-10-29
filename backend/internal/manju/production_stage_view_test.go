package manju

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestProductionStageViewMarksLegacySkippedMachineCheckNonCompliant(t *testing.T) {
	view := productionStageView(model.ManjuProductionStage{
		Key:        ProductionStageFeed,
		ArtifactID: "legacy-feed-artifact",
		InputJSON: `{
			"compiler": "V7.4.26 compile_feed.py",
			"machineCheckStatus": "SKIPPED_BY_USER",
			"machineCheckNote": "用户跳过官方双机检"
		}`,
	}, "p3-artifact")

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
	}, "p3-artifact")

	if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusLegacyNonCompliant {
		t.Fatalf("machine check status = %q", view.MachineCheckStatus)
	}
	if view.MachineCheckNote != "历史非合规状态：未通过 V7.4.26 机检，不能作为当前锁定依据。" {
		t.Fatalf("machine check note = %q", view.MachineCheckNote)
	}
}

func TestProductionStageViewRejectsIncompletePassProof(t *testing.T) {
	tests := map[string]string{
		"top-level status only": `{
			"machineCheckStatus": "PASS",
			"machineCheckWarnings": ["warning"]
		}`,
		"missing validator": `{
			"machineCheckProof": {
				"version": "V7.4.26",
				"pipeline": "compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py",
				"compiler": "V7.4.26 compile_feed.py",
				"gate": "V7.4.26 gate_check.py",
				"dialogueDiff": "V7.4.26 script_feed_diff.py",
				"dialogueBaselineArtifactId": "p3-artifact",
				"status": "PASS",
				"warnings": []
			}
		}`,
		"mismatched gate": `{
			"machineCheckProof": {
				"version": "V7.4.26",
				"pipeline": "compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py",
				"compiler": "V7.4.26 compile_feed.py",
				"validator": "V7.4.26 validate_prompt.py",
				"gate": "custom gate",
				"dialogueDiff": "V7.4.26 script_feed_diff.py",
				"dialogueBaselineArtifactId": "p3-artifact",
				"status": "PASS",
				"warnings": []
			}
		}`,
		"missing warnings": `{
			"machineCheckProof": {
				"version": "V7.4.26",
				"pipeline": "compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py",
				"compiler": "V7.4.26 compile_feed.py",
				"validator": "V7.4.26 validate_prompt.py",
				"gate": "V7.4.26 gate_check.py",
				"dialogueDiff": "V7.4.26 script_feed_diff.py",
				"dialogueBaselineArtifactId": "p3-artifact",
				"status": "PASS"
			}
		}`,
	}
	for name, inputJSON := range tests {
		t.Run(name, func(t *testing.T) {
			view := productionStageView(model.ManjuProductionStage{
				Key:        ProductionStageFeed,
				ArtifactID: "feed-artifact",
				InputJSON:  inputJSON,
			}, "p3-artifact")
			if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusLegacyNonCompliant {
				t.Fatalf("machine check status = %q", view.MachineCheckStatus)
			}
			if view.MachineCheckProofVerified {
				t.Fatal("incomplete proof must not be marked verified")
			}
		})
	}
}

func TestProductionStageViewExposesVerifiedProofAndDialogueWarnings(t *testing.T) {
	view := productionStageView(model.ManjuProductionStage{
		Key:        ProductionStageFeed,
		ArtifactID: "feed-artifact",
		InputJSON: `{
			"machineCheckProof": {
				"version": "V7.4.26",
				"pipeline": "compile_feed.py -> validate_prompt.py -> gate_check.py -> script_feed_diff.py",
				"compiler": "V7.4.26 compile_feed.py",
				"validator": "V7.4.26 validate_prompt.py",
				"gate": "V7.4.26 gate_check.py",
				"dialogueDiff": "V7.4.26 script_feed_diff.py",
				"dialogueBaselineArtifactId": "p3-artifact",
				"status": "PASS",
				"warnings": ["1. [⚠ 部分覆盖 0.70] 林澈：别开门，外面有人。"]
			}
		}`,
	}, "p3-artifact")

	if view.MachineCheckStatus != model.ManjuProductionMachineCheckStatusPass {
		t.Fatalf("machine check status = %q", view.MachineCheckStatus)
	}
	if !view.MachineCheckProofVerified {
		t.Fatal("exact proof must be marked verified")
	}
	if len(view.MachineCheckWarnings) != 1 {
		t.Fatalf("machine check warnings = %+v", view.MachineCheckWarnings)
	}
}
