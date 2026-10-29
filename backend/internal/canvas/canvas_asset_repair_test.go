package canvas

import (
	"encoding/json"
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestReconcileCanvasMediaAssetBindingsRepairsMissingAssetForReadyResource(t *testing.T) {
	svc := newCanvasHistoryTestService(t)
	resource := model.Resource{
		ID:        "resource-canvas-stale-asset",
		UserID:    "user-1",
		Kind:      "image",
		Status:    model.ResourceStatusReady,
		Provider:  "local",
		ObjectKey: "users/user-1/image/stale.png",
	}
	if err := svc.repo.DB().Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{
		"id":"canvas-1",
		"nodes":[{
			"id":"node-1",
			"type":"image",
			"metadata":{
				"assetId":"2fWgj-wMGvBcijgMNdSeX",
				"storageKey":"resource:resource-canvas-stale-asset",
				"content":"/api/resources/resource-canvas-stale-asset/file"
			}
		}]
	}`)

	repaired, changed, err := svc.reconcileCanvasMediaAssetBindings("user-1", raw, nil)
	if err != nil {
		t.Fatalf("reconcileCanvasMediaAssetBindings: %v", err)
	}
	if !changed {
		t.Fatal("expected stale asset binding to be repaired")
	}
	var document struct {
		Nodes []struct {
			Metadata struct {
				AssetID string `json:"assetId"`
			} `json:"metadata"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(repaired, &document); err != nil {
		t.Fatal(err)
	}
	assetID := document.Nodes[0].Metadata.AssetID
	if assetID == "" || assetID == "2fWgj-wMGvBcijgMNdSeX" {
		t.Fatalf("assetId = %q, want repaired asset ID", assetID)
	}
	asset, err := svc.repo.AssetForUser("user-1", assetID)
	if err != nil {
		t.Fatalf("repaired asset not found: %v", err)
	}
	if asset.Kind != "image" {
		t.Fatalf("repaired asset kind = %q, want image", asset.Kind)
	}
}
