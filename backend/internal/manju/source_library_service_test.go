package manju_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestSourceLibraryStoresOrderedChaptersAndCreatesAdaptationUnit(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}

	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title:              "雨夜旧宅",
		AuthorizationScope: "仅本项目改编",
		FileName:           "雨夜旧宅.txt",
		Content: `第一章 雨夜
林澈推开旧宅的门。

第二章 旧照片
照片背面写着一个日期。`,
	})
	if err != nil {
		t.Fatalf("create source document: %v", err)
	}
	if document.Document.ChapterCount != 2 || len(document.Chapters) != 2 {
		t.Fatalf("expected two chapters: %+v", document)
	}
	if document.Chapters[0].Title != "第一章 雨夜" || document.Chapters[1].Position != 2 {
		t.Fatalf("chapter order/title not preserved: %+v", document.Chapters)
	}

	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode:             manju.ScriptModeStepwise,
		Title:            "第一章改编",
		SourceDocumentID: document.Document.ID,
		ChapterIDs:       []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatalf("create chapter-based adaptation unit: %v", err)
	}
	if run.Run.UnitTitle != "第一章改编" || run.Run.SourceDocumentID != document.Document.ID {
		t.Fatalf("unit metadata missing: %+v", run.Run)
	}
	if len(run.Run.SourceChapterIDs) != 1 || run.Run.SourceChapterIDs[0] != document.Chapters[0].ID {
		t.Fatalf("selected chapter not retained: %+v", run.Run)
	}
	if run.Source == nil || !strings.Contains(run.Source.RangeLabel, "第一章 雨夜") {
		t.Fatalf("run source should be seeded from the selected chapter: %+v", run.Source)
	}
	sourceStage := findScriptStage(t, run, manju.ScriptStageSourceRegistration)
	if sourceStage.Status != manju.ScriptStageStatusLocked || run.Run.CurrentStage != manju.ScriptStageIPDevelopment {
		t.Fatalf("chapter selection already completed source registration; it must not ask for the same source again: stage=%+v run=%+v", sourceStage, run.Run)
	}
	if run.Run.LockRevision <= 0 {
		t.Fatalf("adaptation unit must retain the current P0 revision: %+v", run.Run)
	}

	second, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode:             manju.ScriptModeStepwise,
		Title:            "第二章改编",
		SourceDocumentID: document.Document.ID,
		ChapterIDs:       []string{document.Chapters[1].ID},
		BaseRunID:        run.Run.ID,
	})
	if !errors.Is(err, manju.ErrScriptUnitInProgress) {
		t.Fatalf("next unit must wait for the active unit to finish, got second=%+v err=%v", second, err)
	}
}

func TestSourceLibraryRejectsMissingAdaptationAuthorization(t *testing.T) {
	f := newFixture(t)
	_, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "未经授权的小说", Content: "第一章 正文",
	})
	if !errors.Is(err, manju.ErrInvalidScriptSource) {
		t.Fatalf("source registration without authorization must be rejected, got %v", err)
	}
}

func TestChapterUnitAutoRepairsLegacyAwaitingSourceStage(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "旧章节单元", AuthorizationScope: "本项目改编", Content: "第一章 雨夜\n林澈推门。",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceStage := findScriptStage(t, run, manju.ScriptStageSourceRegistration)
	if err := f.db.Exec("UPDATE manju_script_stages SET status = ?, confirmed_by = ?, confirmed_at = NULL WHERE id = ?", "AWAITING_CONFIRMATION", "", sourceStage.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE manju_script_runs SET current_stage = ? WHERE id = ?", manju.ScriptStageSourceRegistration, run.Run.ID).Error; err != nil {
		t.Fatal(err)
	}
	repaired, err := f.service.ScriptRunDetail(f.ownerID, f.projectID, run.Run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findScriptStage(t, repaired, manju.ScriptStageSourceRegistration); got.Status != manju.ScriptStageStatusLocked {
		t.Fatalf("legacy chapter source stage should auto-lock: %+v", got)
	}
	if repaired.Run.CurrentStage != manju.ScriptStageIPDevelopment {
		t.Fatalf("legacy chapter unit should advance to IP development: %+v", repaired.Run)
	}
}

func TestCreatingChapterUnitAbandonsActiveRunWithoutSource(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	legacyID := "legacy-unbound"
	if err := f.db.Exec(
		"INSERT INTO manju_script_runs (id, project_id, branch, mode, status, current_stage, lock_revision, source_id, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		legacyID, f.projectID, manju.ScriptBranchAdaptation, manju.ScriptModeStepwise, "ACTIVE", manju.ScriptStageSourceRegistration, 1, "stale-source-id", f.ownerID, time.Now(), time.Now(),
	).Error; err != nil {
		t.Fatal(err)
	}
	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "新章节来源", AuthorizationScope: "本项目改编", Content: "第一章 开始\n正文。",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatalf("unbound legacy run must not block a properly sourced unit: %v", err)
	}
	if created.Run.ID == legacyID {
		t.Fatal("expected a new chapter unit")
	}
	var legacyStatus string
	if err := f.db.Table("manju_script_runs").Select("status").Where("id = ?", legacyID).Scan(&legacyStatus).Error; err != nil {
		t.Fatal(err)
	}
	if legacyStatus != "ABANDONED" {
		t.Fatalf("legacy unbound run should be abandoned, got %s", legacyStatus)
	}
}

func TestChapterUnitPromptKeepsEntireSelectedChapter(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Create(&model.ManjuProjectModelBinding{
		ID: "binding-source-long", ProjectID: f.projectID,
		TextChannelID: "channel-text", TextModelKey: "text-pro",
	}).Error; err != nil {
		t.Fatal(err)
	}
	tail := "本章结尾：林澈终于看清照片上的日期。"
	content := "第一章 长章\n" + strings.Repeat("雨声压过走廊里的脚步。", 2500) + "\n" + tail
	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "长篇小说", AuthorizationScope: "本项目改编", Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.ConfirmScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageSourceRegistration); err != nil {
		t.Fatal(err)
	}
	task := &model.Task{
		ID: "task-source-long", UserID: f.ownerID, ProjectID: f.projectID,
		Type: "canvas_text", Status: model.TaskStatusQueued, Prompt: "执行 IP 与角色开发",
	}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: task}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunScriptStage(f.ownerID, f.projectID, run.Run.ID, manju.ScriptStageIPDevelopment, manju.ScriptStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(creator.request.Prompt, tail) {
		t.Fatalf("selected chapter tail was silently truncated from the prompt")
	}
}

func TestSecondUnitCanReuseLockedIPAndStoryBible(t *testing.T) {
	f := newFixture(t)
	lock := validLock()
	lock.Source = manju.SourceAdaptation
	if _, err := f.service.SaveLock(f.ownerID, f.projectID, lock); err != nil {
		t.Fatal(err)
	}
	document, err := f.service.CreateSourceDocument(f.ownerID, f.projectID, manju.SourceDocumentInput{
		Title: "连续小说", AuthorizationScope: "本项目改编",
		Content: "第一章 开始\n故事开始。\n\n第二章 继续\n故事继续。",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第一章", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		key     string
		kind    string
		content string
	}{
		{manju.ScriptStageIPDevelopment, "IP_DEVELOPMENT.md", "# IP\n林澈是档案修复师。"},
		{manju.ScriptStageSeriesBible, "SERIES_BIBLE.md", "# 故事圣经\n1999 年雨夜旧宅。"},
	} {
		stage := findScriptStage(t, first, item.key)
		artifact := &model.ManjuScriptArtifact{
			ID: "artifact-" + item.key, ProjectID: f.projectID, RunID: first.Run.ID, StageID: stage.ID,
			Kind: item.kind, Version: 1, Content: item.content, ContentHash: "hash-" + item.key,
			CreatedBy: f.ownerID, CreatedAt: time.Now(), TaskID: "",
		}
		if err := f.db.Create(artifact).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.db.Model(&model.ManjuScriptStage{}).Where("id = ?", stage.ID).Updates(map[string]any{
			"status": model.ManjuScriptStageStatusLocked, "artifact_id": artifact.ID,
			"version": 1,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.Exec("UPDATE manju_script_runs SET status = ?, current_stage = ? WHERE id = ?", "COMPLETED", "", first.Run.ID).Error; err != nil {
		t.Fatal(err)
	}

	second, err := f.service.CreateScriptRun(f.ownerID, f.projectID, manju.ScriptRunInput{
		Mode: manju.ScriptModeStepwise, Title: "第二章", SourceDocumentID: document.Document.ID,
		ChapterIDs: []string{document.Chapters[1].ID}, BaseRunID: first.Run.ID, ReuseGlobalStages: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{manju.ScriptStageIPDevelopment, manju.ScriptStageSeriesBible} {
		stage := findScriptStage(t, second, key)
		if stage.Status != manju.ScriptStageStatusLocked || stage.ArtifactID == "" {
			t.Fatalf("reused stage %s should be locked with a copied artifact: %+v", key, stage)
		}
	}
	second, err = f.service.ConfirmScriptStage(f.ownerID, f.projectID, second.Run.ID, manju.ScriptStageSourceRegistration)
	if err != nil {
		t.Fatal(err)
	}
	if second.Run.CurrentStage != manju.ScriptStageEpisodePlanning {
		t.Fatalf("after source confirmation, reused IP/story stages should be skipped: %+v", second.Run)
	}
}
