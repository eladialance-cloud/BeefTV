package manju_test

import (
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
)

func TestAssetRunInheritsBaseCatalogAndM2References(t *testing.T) {
	f := newFixture(t)
	firstScriptID := prepareCompletedScript(t, f, "M2")
	var lockRevision int64
	if err := f.db.Table("manju_script_runs").Select("lock_revision").Where("id = ?", firstScriptID).Scan(&lockRevision).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	baseAssetRunID := "asset-base"
	if err := f.db.Exec(
		"INSERT INTO manju_asset_runs (id, project_id, script_run_id, asset_mode, status, current_stage, lock_revision, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		baseAssetRunID, f.projectID, firstScriptID, "M2", "COMPLETED", "", lockRevision, f.ownerID, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	for _, stage := range []struct {
		id, key, title string
		position       int
	}{
		{"asset-base-extract", manju.AssetStageExtraction, "实体提取", 1},
		{"asset-base-manifest", manju.AssetStageManifest, "资产清单", 2},
		{"asset-base-compile", manju.AssetStageCompile, "资产交付编译", 3},
	} {
		if err := f.db.Exec(
			"INSERT INTO manju_asset_stages (id, run_id, key, title, position, status, version, artifact_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			stage.id, baseAssetRunID, stage.key, stage.title, stage.position, "LOCKED", 1, "", now, now,
		).Error; err != nil {
			t.Fatal(err)
		}
	}
	manifestContent := "# 资产清单\n| CHR-林澈 | A | 档案修复师 |"
	compileContent := "# 资产编号册\n@CHR-林澈 已锁定"
	for _, artifact := range []struct {
		id, stageID, kind, content string
	}{
		{"asset-base-manifest-artifact", "asset-base-manifest", "ASSET_MANIFEST.md", manifestContent},
		{"asset-base-compile-artifact", "asset-base-compile", "ASSET_CATALOG_M2.md", compileContent},
	} {
		if err := f.db.Exec(
			"INSERT INTO manju_asset_artifacts (id, project_id, run_id, stage_id, kind, version, content, content_hash, task_id, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			artifact.id, f.projectID, baseAssetRunID, artifact.stageID, artifact.kind, 1, artifact.content, artifact.id, "", f.ownerID, now,
		).Error; err != nil {
			t.Fatal(err)
		}
		if err := f.db.Exec("UPDATE manju_asset_stages SET artifact_id = ? WHERE id = ?", artifact.id, artifact.stageID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.Exec(
		"INSERT INTO manju_asset_references (id, project_id, run_id, code, asset_type, resource_id, url, status, notes, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"asset-base-ref", f.projectID, baseAssetRunID, "CHR-林澈", "CHR", "resource-character", "", "READY", "已确认角色图", f.ownerID, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}
	secondScriptID := "script-second-unit"
	if err := f.db.Exec(
		"INSERT INTO manju_script_runs (id, project_id, branch, mode, status, current_stage, lock_revision, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		secondScriptID, f.projectID, manju.ScriptBranchAdaptation, manju.ScriptModeStepwise, "COMPLETED", "", lockRevision, f.ownerID, now, now,
	).Error; err != nil {
		t.Fatal(err)
	}

	run, err := f.service.CreateAssetRun(f.ownerID, f.projectID, manju.AssetRunInput{
		ScriptRunID: secondScriptID, AssetMode: "M2", BaseAssetRunID: baseAssetRunID, ReuseAssets: true,
	})
	if err != nil {
		t.Fatalf("create inherited asset run: %v", err)
	}
	if run.Run.BaseAssetRunID != baseAssetRunID || !run.Run.ReuseAssets || run.Run.InheritedAssetCount == 0 {
		t.Fatalf("asset inheritance metadata missing: %+v", run.Run)
	}
	if len(run.References) != 1 || run.References[0].Code != "CHR-林澈" || run.References[0].ResourceID != "resource-character" {
		t.Fatalf("M2 references were not inherited: %+v", run.References)
	}

	task := &model.Task{ID: "asset-inherited-extract", UserID: f.ownerID, ProjectID: f.projectID, Type: "canvas_text", Status: model.TaskStatusQueued}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	creator := &recordingScriptTaskCreator{task: task}
	f.service.SetScriptTaskCreator(creator.create)
	if _, err := f.service.RunAssetStage(f.ownerID, f.projectID, run.Run.ID, manju.AssetStageExtraction, manju.AssetStageRunInput{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(creator.request.Prompt, "CHR-林澈") || !strings.Contains(creator.request.Prompt, manifestContent) {
		t.Fatalf("P2 extraction prompt must include the inherited structured asset catalog: %s", creator.request.Prompt)
	}
	if strings.Contains(creator.request.Prompt, compileContent) {
		t.Fatalf("P2 extraction prompt must not inherit a previously compiled asset delivery artifact: %s", creator.request.Prompt)
	}
}
