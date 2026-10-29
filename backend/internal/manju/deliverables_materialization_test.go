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
		"# 最终交付\n",
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
	if len(rows) != 2 ||
		rows[0].Kind != model.ManjuDeliverableKindMarkdown ||
		rows[1].Kind != model.ManjuDeliverableKindManifest {
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
				"# 最终交付\n",
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
