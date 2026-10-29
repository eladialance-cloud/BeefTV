package canvas

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func TestCommitUserCanvasDocumentPreservesUntouchedFieldsAndCAS(t *testing.T) {
	svc := newCanvasHistoryTestService(t)
	created, err := svc.UpsertUserCanvasProject("owner", json.RawMessage(`{
		"id":"canvas","revision":0,"title":"initial",
		"nodes":[{"id":"n1","type":"text","title":"旧"}],
		"connections":[],
		"customKeep":"keep-me",
		"viewport":{"x":9,"y":8,"k":2}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	overlay := json.RawMessage(`{
		"id":"forged","revision":99,"remoteContentHash":"local-only",
		"title":"手工提交",
		"nodes":[{"id":"n1","type":"text","title":"新"}],
		"viewport":{"x":1,"y":1,"k":4}
	}`)
	summary, after, err := svc.CommitUserCanvasDocument("owner", "canvas", created.Revision, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Revision != created.Revision+1 {
		t.Fatalf("revision = %d", summary.Revision)
	}
	var doc map[string]any
	if err := json.Unmarshal(after, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["id"] != "canvas" || doc["title"] != "手工提交" || doc["customKeep"] != "keep-me" {
		t.Fatalf("合并丢字段: %s", after)
	}
	if doc["remoteContentHash"] != nil {
		t.Fatal("不应写入 remoteContentHash")
	}
	viewport := doc["viewport"].(map[string]any)
	if viewport["x"] != float64(0) {
		t.Fatalf("视口应由服务端保留: %s", after)
	}
	_, _, err = svc.CommitUserCanvasDocument("owner", "canvas", created.Revision, overlay)
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusConflict {
		t.Fatalf("过期 revision 必须冲突: %v", err)
	}
	_, _, err = svc.CommitUserCanvasDocument("owner", "missing", summary.Revision, overlay)
	if !errors.As(err, &appErr) || appErr.Status != http.StatusNotFound {
		t.Fatalf("缺失画布必须 404: %v", err)
	}
}

func TestCommitUserCanvasDocumentRepairsMissingAssetIDByResource(t *testing.T) {
	svc := newCanvasHistoryTestService(t)
	now := time.Now().UTC()
	resource := model.Resource{
		ID: "resource-auto-bind", UserID: "owner", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "users/owner/video/resource-auto-bind.mp4",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.repo.CreateResource(&resource); err != nil {
		t.Fatal(err)
	}
	asset := json.RawMessage(`{
		"id":"asset-auto-bind",
		"kind":"video",
		"title":"已同步视频",
		"coverUrl":"",
		"tags":[],
		"status":"confirmed",
		"source":"生成任务",
		"data":{
			"url":"/api/resources/resource-auto-bind/file",
			"storageKey":"resource:resource-auto-bind",
			"width":1280,
			"height":720,
			"bytes":100,
			"mimeType":"video/mp4"
		}
	}`)
	if _, err := svc.UpsertUserAsset("owner", asset); err != nil {
		t.Fatal(err)
	}
	created, err := svc.UpsertUserCanvasProject("owner", json.RawMessage(`{
		"id":"canvas-auto-bind",
		"revision":0,
		"title":"自动补绑",
		"nodes":[],
		"connections":[]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	overlay := json.RawMessage(`{
		"title":"自动补绑",
		"nodes":[{
			"id":"video-node",
			"type":"video",
			"title":"生成视频",
			"metadata":{
				"storageKey":"resource:resource-auto-bind",
				"content":"/api/resources/resource-auto-bind/file"
			}
		}],
		"connections":[]
	}`)

	first, after, err := svc.CommitUserCanvasDocument("owner", "canvas-auto-bind", created.Revision, overlay)
	if err != nil {
		t.Fatalf("服务端应按资源自动补绑缺失 assetId: %v", err)
	}
	var document struct {
		Nodes []struct {
			Metadata struct {
				AssetID string `json:"assetId"`
			} `json:"metadata"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(after, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Nodes) != 1 || document.Nodes[0].Metadata.AssetID != "asset-auto-bind" {
		t.Fatalf("自动补绑结果不正确: %s", after)
	}

	timelineOverlay := json.RawMessage(`{
		"timeline":{
			"clips":[{
				"id":"clip-video",
				"directMedia":{
					"id":"media-video",
					"kind":"video",
					"storageKey":"resource:resource-auto-bind",
					"url":"/api/resources/resource-auto-bind/file"
				}
			}]
		}
	}`)
	_, after, err = svc.CommitUserCanvasDocument("owner", "canvas-auto-bind", first.Revision, timelineOverlay)
	if err != nil {
		t.Fatalf("时间线媒体也应按资源自动补绑: %v", err)
	}
	var timelineDocument struct {
		Timeline struct {
			Clips []struct {
				DirectMedia struct {
					AssetID string `json:"assetId"`
				} `json:"directMedia"`
			} `json:"clips"`
		} `json:"timeline"`
	}
	if err := json.Unmarshal(after, &timelineDocument); err != nil {
		t.Fatal(err)
	}
	if len(timelineDocument.Timeline.Clips) != 1 ||
		timelineDocument.Timeline.Clips[0].DirectMedia.AssetID != "asset-auto-bind" {
		t.Fatalf("时间线自动补绑结果不正确: %s", after)
	}
}

func TestCommitUserCanvasDocumentCreatesAssetForOwnedResource(t *testing.T) {
	svc := newCanvasHistoryTestService(t)
	now := time.Now().UTC()
	resource := model.Resource{
		ID: "resource-create-asset", UserID: "owner", Kind: "video", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "users/owner/video/resource-create-asset.mp4",
		MimeType: "video/mp4", Size: 4096, Width: 1280, Height: 720, DurationMs: 5000,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := svc.repo.CreateResource(&resource); err != nil {
		t.Fatal(err)
	}
	created, err := svc.UpsertUserCanvasProject("owner", json.RawMessage(`{
		"id":"canvas-create-asset",
		"revision":0,
		"title":"自动入库",
		"nodes":[],
		"connections":[]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	overlay := json.RawMessage(`{
		"title":"自动入库",
		"nodes":[{
			"id":"video-node",
			"type":"video",
			"title":"生成视频",
			"metadata":{
				"storageKey":"resource:resource-create-asset",
				"content":"/api/resources/resource-create-asset/file"
			}
		}],
		"connections":[]
	}`)

	_, after, err := svc.CommitUserCanvasDocument("owner", "canvas-create-asset", created.Revision, overlay)
	if err != nil {
		t.Fatalf("仅有资源、素材记录缺失时应自动创建素材: %v", err)
	}
	var document struct {
		Nodes []struct {
			Metadata struct {
				AssetID string `json:"assetId"`
			} `json:"metadata"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(after, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Nodes) != 1 || strings.TrimSpace(document.Nodes[0].Metadata.AssetID) == "" {
		t.Fatalf("自动创建素材后未写回 assetId: %s", after)
	}
	asset, err := svc.UserAsset("owner", document.Nodes[0].Metadata.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(asset), "resource:resource-create-asset") {
		t.Fatalf("自动创建的素材未引用原资源: %s", asset)
	}
}
