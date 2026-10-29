package manju

import "testing"

func TestCanvasImportReferencesKeepsPendingAssetNamesFromBatchRows(t *testing.T) {
	payload := `{
		"nodes": [{
			"id": "asset-table",
			"type": "batch-table",
			"title": "漫剧数字资产表",
			"metadata": {
				"manjuStage": "p2_assets",
				"batchTable": {
					"manjuAssetTable": {"version": 1},
					"rows": [{
						"id": "row-1",
						"prompt": "CHR-林澈完整角色四格提示词",
						"manjuAsset": {
							"taskId": "task-1",
							"code": "CHR-林澈",
							"kind": "character",
							"name": "林澈角色四格"
						}
					}]
				}
			}
		}]
	}`

	references := canvasImportReferences(payload, CanvasPackageStageP2Assets)
	if len(references) != 1 {
		t.Fatalf("references = %#v", references)
	}
	if references[0].Code != "CHR-林澈" || references[0].Title != "林澈角色四格" {
		t.Fatalf("reference = %#v", references[0])
	}
	if references[0].Content != "" || references[0].StorageKey != "" {
		t.Fatalf("pending reference must not invent media: %#v", references[0])
	}
	if references[0].Prompt != "CHR-林澈完整角色四格提示词" {
		t.Fatalf("reference prompt = %#v", references[0])
	}
}

func TestMergeCanvasImportReferencesWithP2TasksRestoresPromptsAndMissingRows(t *testing.T) {
	references := []CanvasImportReferenceView{{
		Code: "CHR-01", AssetCode: "CHR-01", Title: "旧角色名", Kind: "image",
	}}
	tasks := []CanvasPackageTask{
		{
			ID: "p2-task-chr", AssetCode: "CHR-01", Title: "林澈角色四格",
			Prompt: "林澈角色四格完整提示词", Category: "character",
		},
		{
			ID: "p2-task-scn", AssetCode: "SCN-老宅", Title: "老宅场景母版",
			Prompt: "老宅场景母版完整提示词", Category: "environment",
		},
	}

	merged := mergeCanvasImportReferencesWithP2Tasks(references, tasks)
	if len(merged) != 2 {
		t.Fatalf("merged references = %#v", merged)
	}
	if merged[0].Prompt != "林澈角色四格完整提示词" || merged[0].TaskID != "p2-task-chr" {
		t.Fatalf("character reference = %#v", merged[0])
	}
	if merged[1].Code != "SCN-老宅" || merged[1].Prompt != "老宅场景母版完整提示词" {
		t.Fatalf("missing scene reference = %#v", merged[1])
	}
}

func TestCanvasImportReferencesKeepsAssetIDForBoundMedia(t *testing.T) {
	payload := `{
		"nodes": [{
			"id": "asset-image",
			"type": "image",
			"title": "图1 · CHR-01",
			"metadata": {
				"manjuStage": "p2_assets",
				"manjuTaskId": "task-1",
				"manjuAssetCode": "CHR-01",
				"assetId": "generation_abc",
				"storageKey": "resource:resource-1",
				"content": "/api/resources/resource-1/file"
			}
		}]
	}`

	references := canvasImportReferences(payload, CanvasPackageStageP2Assets)
	if len(references) != 1 {
		t.Fatalf("references = %#v", references)
	}
	if references[0].AssetID != "generation_abc" {
		t.Fatalf("assetId must survive canvas import reference projection: %#v", references[0])
	}
}
