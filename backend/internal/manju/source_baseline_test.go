package manju_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestSourceLibraryStateChangesWhenNewChapterIsAdded(t *testing.T) {
	f := newFixture(t)
	first, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "前半部", AuthorizationScope: "本项目改编",
		Content: "第一章 开始\n故事开始。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Document.ChapterCount != 1 {
		t.Fatal("fixture should contain one chapter")
	}
	stateBefore, err := f.service.SourceLibraryState(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if stateBefore.ContentHash == "" || stateBefore.DocumentCount != 1 || stateBefore.ChapterCount != 1 {
		t.Fatalf("source library state incomplete: %+v", stateBefore)
	}
	if _, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "后半部", AuthorizationScope: "本项目改编",
		Content: "第二章 继续\n故事继续。",
	}); err != nil {
		t.Fatal(err)
	}
	stateAfter, err := f.service.SourceLibraryState(f.ownerID, f.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if stateAfter.ContentHash == stateBefore.ContentHash || stateAfter.ChapterCount != 2 {
		t.Fatalf("adding a chapter must change the source-set hash: before=%+v after=%+v", stateBefore, stateAfter)
	}
}

func TestReusingGlobalStagesRejectsChangedSourceLibrary(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	firstDocument, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "第一部分", AuthorizationScope: "本项目改编", Content: "第一章 开始\n故事开始。",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章", SourceDocumentID: firstDocument.Document.ID,
		ChapterIDs: []string{firstDocument.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ key, kind, content string }{
		{manju.ScriptStageIPDevelopment, "IP_DEVELOPMENT.md", "# IP\n林澈是档案修复师。"},
		{manju.ScriptStageSeriesBible, "SERIES_BIBLE.md", "# 故事圣经\n旧宅与雨夜。"},
	} {
		stage := findScriptStage(t, first, item.key)
		artifactID := "baseline-" + item.key
		if err := f.db.Exec(
			"INSERT INTO manju_script_artifacts (id, project_id, run_id, stage_id, kind, version, content, content_hash, task_id, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			artifactID, f.projectID, first.Run.ID, stage.ID, item.kind, 1, item.content, item.key, "", f.ownerID, time.Now(),
		).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.db.Exec("UPDATE manju_script_stages SET status = ?, artifact_id = ?, version = 1 WHERE id = ?", "LOCKED", artifactID, stage.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.Exec("UPDATE manju_script_runs SET status = ?, current_stage = ? WHERE id = ?", "COMPLETED", "", first.Run.ID).Error; err != nil {
		t.Fatal(err)
	}

	secondDocument, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "第二部分", AuthorizationScope: "本项目改编", Content: "第二章 继续\n故事继续。",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第二章", SourceDocumentID: secondDocument.Document.ID,
		ChapterIDs: []string{secondDocument.Chapters[0].ID}, BaseRunID: first.Run.ID, ReuseGlobalStages: true,
	})
	if !errors.Is(err, manju.ErrSourceLibraryChanged) {
		t.Fatalf("changed source library must block locked global-stage reuse, got %v", err)
	}
	if _, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第二章重做上游", SourceDocumentID: secondDocument.Document.ID,
		ChapterIDs: []string{secondDocument.Chapters[0].ID},
	}); err != nil {
		t.Fatalf("fresh regeneration should remain available: %v", err)
	}
}

func TestGlobalIPStageReadsEntireProjectSourceLibrary(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(
		"INSERT INTO manju_project_model_bindings (id, project_id, text_channel_id, text_model_key, image_channel_id, image_model_key, video_channel_id, video_model_key, updated_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"binding-global-source", f.projectID, "channel-text", "text-pro", "", "", "", "", f.ownerID, time.Now(), time.Now(),
	).Error; err != nil {
		t.Fatal(err)
	}
	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "完整小说", AuthorizationScope: "本项目改编",
		Content: "第一章 开始\n第一章正文。\n\n第二章 真相\n第二章结局秘密。",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章改编", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration); err != nil {
		t.Fatal(err)
	}
	task := &model.Task{ID: "task-global-source", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: task}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageIPDevelopment, manju.ScriptStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(creator.request.Prompt, "第二章结局秘密") {
		t.Fatalf("global IP stage must read the entire source library, not only the selected unit: %s", creator.request.Prompt)
	}
}
