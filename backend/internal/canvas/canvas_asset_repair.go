package canvas

import (
	"encoding/json"
	"fmt"
	"strings"

	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// reconcileCanvasMediaAssetBindings repairs resource-backed canvas media whose
// assetId is missing, stale, foreign, or points at a different Resource. The
// Asset remains the authority: a binding is changed only when an owned Asset
// references the exact same Resource.
func (s *Service) reconcileCanvasMediaAssetBindings(userID string, raw json.RawMessage, candidates []model.Asset) (json.RawMessage, bool, error) {
	references, err := MediaAssetReferences(raw)
	if err != nil {
		return nil, false, err
	}
	declaredAssetIDs := make(map[string]struct{})
	for _, reference := range references {
		if strings.TrimSpace(reference.AssetID) != "" {
			declaredAssetIDs[reference.AssetID] = struct{}{}
		}
	}

	declaredAssets := make(map[string]model.Asset, len(declaredAssetIDs)+len(candidates))
	if len(declaredAssetIDs) > 0 {
		assetsForUser, err := s.repo.AssetsForUserIDs(userID, assets.SortedIDs(declaredAssetIDs))
		if err != nil {
			return nil, false, err
		}
		for _, item := range assetsForUser {
			declaredAssets[item.ID] = item
		}
	}
	for _, item := range candidates {
		if item.UserID == userID {
			declaredAssets[item.ID] = item
		}
	}

	repairResources := make(map[string]struct{})
	for _, reference := range references {
		if reference.ResourceID == "" {
			continue
		}
		item, exists := declaredAssets[reference.AssetID]
		if !exists || !assetReferencesResource(item.PayloadJSON, reference.ResourceID) {
			repairResources[reference.ResourceID] = struct{}{}
		}
	}
	if len(repairResources) == 0 {
		return raw, false, nil
	}

	resourceAssetIDs := make(map[string]string)
	userAssets, err := s.Library().UserAssets(userID)
	if err != nil {
		return nil, false, err
	}
	for _, item := range userAssets {
		var header struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(item, &header) != nil || strings.TrimSpace(header.ID) == "" {
			continue
		}
		collectCanvasAssetResourceBindings(header.ID, string(item), repairResources, resourceAssetIDs)
	}
	for _, item := range candidates {
		if item.UserID != userID || strings.TrimSpace(item.ID) == "" {
			continue
		}
		collectCanvasAssetResourceBindings(item.ID, item.PayloadJSON, repairResources, resourceAssetIDs)
	}
	for resourceID := range repairResources {
		if resourceAssetIDs[resourceID] != "" {
			continue
		}
		assetID, err := s.createCanvasAssetForResource(userID, resourceID)
		if err != nil {
			return nil, false, err
		}
		if assetID != "" {
			resourceAssetIDs[resourceID] = assetID
		}
	}
	if len(resourceAssetIDs) == 0 {
		return raw, false, nil
	}

	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, false, fmt.Errorf("解析画布媒体绑定失败: %w", err)
	}
	changed := false
	for _, rawNode := range sliceValue(document["nodes"]) {
		node := mapValue(rawNode)
		if bindCanvasMediaMetadata(node["metadata"], resourceAssetIDs, repairResources) {
			changed = true
		}
	}
	timeline := mapValue(document["timeline"])
	for _, rawClip := range sliceValue(timeline["clips"]) {
		clip := mapValue(rawClip)
		if bindCanvasMediaMetadata(clip["directMedia"], resourceAssetIDs, repairResources) {
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	repaired, err := json.Marshal(document)
	if err != nil {
		return nil, false, err
	}
	return repaired, true, nil
}

func (s *Service) createCanvasAssetForResource(userID, resourceID string) (string, error) {
	resource, err := s.repo.ResourceForUser(userID, resourceID)
	if err != nil || resource == nil || resource.Status != model.ResourceStatusReady {
		return "", nil
	}
	kind := strings.ToLower(strings.TrimSpace(resource.Kind))
	if kind != "image" && kind != "video" && kind != "audio" {
		return "", nil
	}
	assetID := kernel.NewID()
	fileURL := assets.FileURL(resource.ID)
	mimeType := strings.TrimSpace(resource.MimeType)
	if mimeType == "" {
		switch kind {
		case "image":
			mimeType = "image/png"
		case "video":
			mimeType = "video/mp4"
		case "audio":
			mimeType = "audio/mpeg"
		}
	}
	width := resource.Width
	if width <= 0 {
		width = 1
	}
	height := resource.Height
	if height <= 0 {
		height = 1
	}
	data := map[string]any{
		"storageKey": "resource:" + resource.ID,
		"mimeType":   mimeType,
		"bytes":      resource.Size,
	}
	title := "画布素材"
	switch kind {
	case "image":
		title = "画布图片"
		data["dataUrl"] = ""
		data["width"] = width
		data["height"] = height
	case "video":
		title = "画布视频"
		data["url"] = fileURL
		data["width"] = width
		data["height"] = height
		data["durationMs"] = resource.DurationMs
	case "audio":
		title = "画布音频"
		data["url"] = fileURL
		data["durationMs"] = resource.DurationMs
	}
	coverURL := ""
	if kind == "image" {
		coverURL = fileURL
	}
	payload, err := json.Marshal(map[string]any{
		"id":       assetID,
		"kind":     kind,
		"title":    title,
		"coverUrl": coverURL,
		"tags":     []string{},
		"status":   "confirmed",
		"source":   "Canvas",
		"category": "material",
		"metadata": map[string]any{
			"resourceKey": "resource:" + resource.ID,
		},
		"data": data,
	})
	if err != nil {
		return "", err
	}
	if _, err := s.Library().UpsertUserAsset(userID, payload); err != nil {
		return "", err
	}
	return assetID, nil
}

func collectCanvasAssetResourceBindings(assetID, payload string, resourceIDs map[string]struct{}, result map[string]string) {
	matched := make(map[string]struct{})
	if err := assets.CollectOwnedDocumentReferences(payload, matched); err != nil {
		return
	}
	for resourceID := range resourceIDs {
		if _, ok := matched[resourceID]; !ok {
			continue
		}
		if _, exists := result[resourceID]; !exists {
			result[resourceID] = assetID
		}
	}
}

func assetReferencesResource(payloadJSON, resourceID string) bool {
	return assets.DocumentReferences(payloadJSON, map[string]struct{}{resourceID: {}})
}

func bindCanvasMediaMetadata(raw any, resourceAssetIDs map[string]string, repairResources map[string]struct{}) bool {
	metadata := mapValue(raw)
	if metadata == nil {
		return false
	}
	for _, key := range []string{"storageKey", "content", "dataUrl", "url"} {
		resourceID := assets.ResourceID(stringValue(metadata[key]))
		if resourceID == "" {
			continue
		}
		if _, needsRepair := repairResources[resourceID]; !needsRepair {
			continue
		}
		if assetID := resourceAssetIDs[resourceID]; assetID != "" {
			if strings.TrimSpace(stringValue(metadata["assetId"])) == assetID {
				return false
			}
			metadata["assetId"] = assetID
			return true
		}
	}
	return false
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func sliceValue(value any) []any {
	result, _ := value.([]any)
	return result
}

func stringValue(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}
