package manju

import (
	"errors"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestBuildProductionDeliveryRowsMarkdownNeverBuildsBoard(t *testing.T) {
	run := model.ManjuProductionRun{ID: "run-markdown", ProjectID: "project", LockRevision: 7}
	boardCalls := 0
	rows, err := buildProductionDeliveryRows(
		"测试项目",
		run,
		DeliveryMarkdown,
		"# 用户交付\n\n## 资产提示词\nA\n\n## 分镜提示词\nB\n",
		"# 完整内部档案\n",
		"# 测试项目 · 预演故事板\n",
		func(string) (string, error) {
			boardCalls++
			return "<!DOCTYPE html>", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if boardCalls != 0 {
		t.Fatalf("markdown-only delivery called the board compiler %d times", boardCalls)
	}
	if len(rows) != 3 ||
		rows[0].Kind != model.ManjuDeliverableKindMarkdown ||
		rows[1].Kind != "previs" ||
		rows[2].Kind != model.ManjuDeliverableKindManifest {
		t.Fatalf("markdown-only rows = %+v", rows)
	}
}

func TestBuildProductionDeliveryRowsFailsClosedOnBoardErrorsAndEmptyOutput(t *testing.T) {
	run := model.ManjuProductionRun{ID: "run-board", ProjectID: "project", LockRevision: 8}
	for _, tc := range []struct {
		name  string
		build func(string) (string, error)
	}{
		{name: "script error", build: func(string) (string, error) { return "", errors.New("python failed") }},
		{name: "empty output", build: func(string) (string, error) { return " \n", nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := buildProductionDeliveryRows(
				"测试项目",
				run,
				DeliveryBoard,
				"# 用户交付\n",
				"# 完整内部档案\n",
				"# 测试项目 · 预演故事板\n",
				tc.build,
			)
			if err == nil {
				t.Fatalf("board delivery must fail closed, rows=%+v", rows)
			}
			if tc.name == "script error" && !strings.Contains(err.Error(), "python failed") {
				t.Fatalf("board error was not preserved: %v", err)
			}
		})
	}
}

func TestProductionBoardArchiveIncludesLockedQCArtifactOnce(t *testing.T) {
	const qcArtifactID = "qc-artifact"
	archive := buildProductionBoardArchive(
		nil,
		nil,
		[]model.ManjuProductionArtifact{{
			ID: qcArtifactID, Kind: model.ManjuProductionArtifactQC, Version: 1,
			Content: "# P5 质检与交付\nQC_MARKER_ONCE\n",
		}},
		qcArtifactID,
		"# P5 质检与交付\nQC_MARKER_ONCE\n",
	)
	if got := strings.Count(archive, "QC_MARKER_ONCE"); got != 1 {
		t.Fatalf("locked QC artifact included %d times, want once:\n%s", got, archive)
	}
}
