package app

import (
	"testing"

	"infinite-canvas/backend/internal/canvas"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/operations"
)

func TestTrustedEmptyImageBindRequiresOwnedSucceededOriginalTaskNode(t *testing.T) {
	base := model.Task{
		ID: "task-image", UserID: "owner", ProjectID: "canvas-1", Type: "canvas_image",
		Status:    model.TaskStatusSucceeded,
		InputJSON: `{"metadata":{"source":"canvas","nodeId":"node-image"}}`,
	}
	patch := canvas.TaskOutputBind{TaskID: "task-image", CanvasID: "canvas-1", NodeID: "node-image"}
	if !trustedEmptyImageBind(&base, "owner", patch) {
		t.Fatal("trusted original empty image node was rejected")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*model.Task, *canvas.TaskOutputBind, *string)
	}{
		{name: "foreign owner", mutate: func(_ *model.Task, _ *canvas.TaskOutputBind, userID *string) { *userID = "other" }},
		{name: "foreign canvas", mutate: func(task *model.Task, _ *canvas.TaskOutputBind, _ *string) { task.ProjectID = "canvas-2" }},
		{name: "wrong task type", mutate: func(task *model.Task, _ *canvas.TaskOutputBind, _ *string) { task.Type = "canvas_video" }},
		{name: "unfinished task", mutate: func(task *model.Task, _ *canvas.TaskOutputBind, _ *string) { task.Status = model.TaskStatusRunning }},
		{name: "wrong node", mutate: func(_ *model.Task, patch *canvas.TaskOutputBind, _ *string) { patch.NodeID = "node-other" }},
		{name: "missing target", mutate: func(task *model.Task, _ *canvas.TaskOutputBind, _ *string) { task.InputJSON = `{}` }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := base
			candidate := patch
			userID := "owner"
			tc.mutate(&task, &candidate, &userID)
			if trustedEmptyImageBind(&task, userID, candidate) {
				t.Fatal("untrusted empty image bind was accepted")
			}
		})
	}
}

func TestCanvasBindRecoverySkipsMissingLegacyProject(t *testing.T) {
	if !skippableCanvasBindError(operations.NotFound("not_found", "资源不存在")) {
		t.Fatal("one missing legacy project must not pause the whole recovery batch")
	}
}

func TestTrustedEmptyVideoBindRequiresOwnedSucceededOriginalTaskNode(t *testing.T) {
	task := model.Task{
		ID: "task-video", UserID: "owner", ProjectID: "canvas-1", Type: "canvas_video",
		Status:    model.TaskStatusSucceeded,
		InputJSON: `{"metadata":{"source":"canvas","nodeId":"node-video"}}`,
	}
	patch := canvas.TaskOutputBind{TaskID: "task-video", CanvasID: "canvas-1", NodeID: "node-video"}
	if !trustedEmptyVideoBind(&task, "owner", patch) {
		t.Fatal("trusted original empty video node was rejected")
	}
	task.Status = model.TaskStatusRunning
	if trustedEmptyVideoBind(&task, "owner", patch) {
		t.Fatal("unfinished video task was accepted")
	}
}
