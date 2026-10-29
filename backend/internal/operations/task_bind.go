package operations

import (
	"encoding/json"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/canvas"
	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
	"infinite-canvas/backend/internal/taskbinding"
)

func opCanvasTaskBind(ctx *Context, params json.RawMessage) (any, error) {
	var args struct {
		CanvasID    string `json:"canvasId"`
		TaskID      string `json:"taskId"`
		NodeID      string `json:"nodeId"`
		OutputIndex int    `json:"outputIndex"`
	}
	if err := decodeParams(params, &args); err != nil {
		return nil, err
	}
	receipt, err := taskbinding.Bind(&bindPorts{ctx: ctx}, ctx.UserID, taskbinding.Request{
		CanvasID:    args.CanvasID,
		TaskID:      args.TaskID,
		NodeID:      args.NodeID,
		OutputIndex: args.OutputIndex,
	})
	if err != nil {
		return nil, mapDomainError(err)
	}
	projected, err := projectCanvasTaskBindOutcome(ctx, args.CanvasID, args.NodeID, receiptMap(receipt))
	if err != nil {
		return nil, err
	}
	return sanitizeForClient(projected), nil
}

func opCanvasTaskRecover(ctx *Context, params json.RawMessage) (any, error) {
	var args struct {
		CanvasID    string `json:"canvasId"`
		TaskID      string `json:"taskId"`
		OutputIndex int    `json:"outputIndex"`
		Title       string `json:"title"`
		Position    *struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"position"`
	}
	if err := decodeParams(params, &args); err != nil {
		return nil, err
	}
	args.CanvasID = strings.TrimSpace(args.CanvasID)
	args.TaskID = strings.TrimSpace(args.TaskID)
	task, err := ctx.Domain.WorkspaceTask(ctx.UserID, args.TaskID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	if task == nil {
		return nil, NotFound("task_not_found", "任务不存在")
	}
	raw, err := ctx.Domain.UserCanvasProject(ctx.UserID, args.CanvasID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, AsError(err)
	}

	nodeID := ""
	for _, rawNode := range canvasNodes(doc) {
		node, _ := rawNode.(map[string]any)
		if nodeMetadataTaskID(node) == args.TaskID && recoveryNodeTypeMatchesTask(node, task.Type) {
			nodeID, _ = node["id"].(string)
			break
		}
	}
	createMissing := strings.TrimSpace(nodeID) == ""
	if createMissing {
		nodeID = "recover-" + localtask.StableEntityID("recovery-node", args.TaskID, strconv.Itoa(args.OutputIndex))
	}
	title := strings.TrimSpace(args.Title)
	if title == "" {
		title = recoveryNodeTitle(task)
	}
	positionX, positionY := 120.0, 120.0
	if args.Position != nil {
		positionX, positionY = args.Position.X, args.Position.Y
	}
	receipt, err := taskbinding.Bind(&bindPorts{ctx: ctx}, ctx.UserID, taskbinding.Request{
		CanvasID: args.CanvasID, TaskID: args.TaskID, NodeID: nodeID, OutputIndex: args.OutputIndex,
		RecoveryMode: true, CreateMissingNode: createMissing, NodeTitle: title,
		NodePositionX: positionX, NodePositionY: positionY,
	})
	if err != nil {
		return nil, mapDomainError(err)
	}
	projected, err := projectCanvasTaskBindOutcome(ctx, args.CanvasID, nodeID, receiptMap(receipt))
	if err != nil {
		return nil, err
	}
	return sanitizeForClient(projected), nil
}

func recoveryNodeTitle(task *model.Task) string {
	if task == nil {
		return "恢复结果"
	}
	label := "媒体"
	switch strings.TrimSpace(task.Type) {
	case "canvas_image":
		label = "图片"
	case "canvas_video":
		label = "视频"
	case "canvas_text", "text_replay":
		label = "文本"
	}
	if modelName := strings.TrimSpace(task.Model); modelName != "" {
		return modelName + " " + label + "恢复"
	}
	return label + "恢复"
}

func projectCanvasTaskBindReplay(ctx *Context, params json.RawMessage, stored any) (any, error) {
	var args struct {
		CanvasID string `json:"canvasId"`
		NodeID   string `json:"nodeId"`
		TaskID   string `json:"taskId"`
	}
	if json.Unmarshal(params, &args) != nil {
		return stored, nil
	}
	historical := cloneReceiptMap(stored)
	projected, err := projectCanvasTaskBindOutcome(ctx, args.CanvasID, args.NodeID, historical)
	if err != nil {
		return nil, err
	}
	return sanitizeForClient(projected), nil
}

func projectCanvasTaskBindOutcome(ctx *Context, canvasID, nodeID string, stored map[string]any) (map[string]any, error) {
	historical := historicalBindReceipt(stored)
	projected := map[string]any{
		"applied":       stored["applied"],
		"canvasId":      stored["canvasId"],
		"nodeId":        stored["nodeId"],
		"taskId":        stored["taskId"],
		"outputIndex":   stored["outputIndex"],
		"effectKey":     stored["effectKey"],
		"mediaType":     stored["mediaType"],
		"alreadyBound":  stored["alreadyBound"],
		"historical":    historical,
		"bindingStatus": "deleted",
	}
	if ctx == nil || ctx.Domain == nil {
		return projected, nil
	}
	raw, err := ctx.Domain.UserCanvasProject(ctx.UserID, strings.TrimSpace(canvasID))
	if err != nil {
		if isNotFoundError(err) {
			return projected, nil
		}
		return nil, mapDomainError(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, AsError(err)
	}
	projected["revision"] = canvasRevision(doc)
	projected["canvas"] = doc
	node := findDocNode(doc, strings.TrimSpace(nodeID))
	if node == nil {
		return projected, nil
	}
	projected["node"] = node
	historicalTaskID, _ := stored["taskId"].(string)
	if currentTaskID := nodeMetadataTaskID(node); currentTaskID != "" && strings.TrimSpace(historicalTaskID) != "" && currentTaskID != strings.TrimSpace(historicalTaskID) {
		projected["bindingStatus"] = "replaced"
		return projected, nil
	}
	projected["bindingStatus"] = "bound"
	copyCurrentGenerationFields(projected, node)
	return projected, nil
}

func historicalBindReceipt(stored map[string]any) map[string]any {
	out := cloneReceiptMap(stored)
	delete(out, "canvas")
	delete(out, "historical")
	delete(out, "bindingStatus")
	return out
}

func cloneReceiptMap(stored any) map[string]any {
	if stored == nil {
		return map[string]any{}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		if typed, ok := stored.(map[string]any); ok {
			out := make(map[string]any, len(typed))
			for key, value := range typed {
				out[key] = value
			}
			return out
		}
		return map[string]any{}
	}
	var out map[string]any
	if json.Unmarshal(encoded, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func nodeMetadataTaskID(node map[string]any) string {
	if node == nil {
		return ""
	}
	meta, _ := node["metadata"].(map[string]any)
	if meta == nil {
		return ""
	}
	value, _ := meta["taskId"].(string)
	return strings.TrimSpace(value)
}

func copyCurrentGenerationFields(projected map[string]any, node map[string]any) {
	if projected == nil || node == nil {
		return
	}
	meta, _ := node["metadata"].(map[string]any)
	if meta == nil {
		return
	}
	for _, key := range []string{"content", "storageKey", "assetId"} {
		if value, _ := meta[key].(string); strings.TrimSpace(value) != "" {
			projected[key] = value
		}
	}
}

func receiptMap(receipt taskbinding.Receipt) map[string]any {
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return map[string]any{"applied": receipt.Applied, "canvasId": receipt.CanvasID, "nodeId": receipt.NodeID, "taskId": receipt.TaskID, "revision": receipt.Revision}
	}
	var out map[string]any
	if json.Unmarshal(encoded, &out) != nil {
		return map[string]any{"applied": receipt.Applied, "revision": receipt.Revision}
	}
	return out
}

type bindPorts struct {
	ctx *Context
}

func (p *bindPorts) Task(userID, taskID string) (*model.Task, error) {
	return p.ctx.Domain.WorkspaceTask(userID, taskID)
}

func (p *bindPorts) GenerationOutputs(taskID string) ([]localtask.CanonicalOutput, error) {
	return p.ctx.Domain.GenerationOutputs(taskID)
}

func (p *bindPorts) OwnedReadyResource(userID, resourceID string) (*model.Resource, error) {
	return p.ctx.Domain.OwnedReadyResource(userID, resourceID)
}

func (p *bindPorts) OwnedAsset(userID, assetID string) (*model.Asset, error) {
	return p.ctx.Domain.OwnedAsset(userID, assetID)
}

func (p *bindPorts) CanBindRecoveredNode(userID string, task model.Task, nodeID string, allowCreate bool) (bool, error) {
	originalNodeID := strings.TrimSpace(localtask.TargetBindingFromInput(task.InputJSON).NodeID)
	nodeID = strings.TrimSpace(nodeID)
	if originalNodeID == "" || nodeID == "" || nodeID == originalNodeID {
		return false, nil
	}
	raw, err := p.ctx.Domain.UserCanvasProject(userID, strings.TrimSpace(task.ProjectID))
	if err != nil {
		return false, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, err
	}
	node := findDocNode(doc, nodeID)
	if node != nil {
		return recoveryNodeTypeMatchesTask(node, task.Type) && nodeMetadataTaskID(node) == strings.TrimSpace(task.ID), nil
	}
	if allowCreate {
		return true, nil
	}
	return findDocNode(doc, originalNodeID) == nil, nil
}

func recoveryNodeTypeMatchesTask(node map[string]any, taskType string) bool {
	nodeType, _ := node["type"].(string)
	switch strings.TrimSpace(taskType) {
	case "canvas_image":
		return nodeType == "image"
	case "canvas_video":
		return nodeType == "video"
	case "canvas_text", "text_replay":
		return nodeType == "text"
	default:
		return false
	}
}

func (p *bindPorts) BindExistingNode(userID string, patch taskbinding.NodePatch) (taskbinding.NodeBindResult, error) {
	result, err := p.ctx.Domain.BindExistingCanvasNode(userID, canvas.TaskOutputBind{
		CanvasID:          patch.CanvasID,
		NodeID:            patch.NodeID,
		TaskID:            patch.TaskID,
		OutputIndex:       patch.OutputIndex,
		EffectKey:         patch.EffectKey,
		MediaType:         patch.MediaType,
		AssetID:           patch.AssetID,
		ResourceID:        patch.ResourceID,
		StorageKey:        patch.StorageKey,
		Content:           patch.Content,
		MimeType:          patch.MimeType,
		Bytes:             patch.Bytes,
		Width:             patch.Width,
		Height:            patch.Height,
		DurationMs:        patch.DurationMs,
		Storyboard:        patch.Storyboard,
		Metadata:          patch.Metadata,
		CreateMissingNode: patch.CreateMissingNode,
		RecoveryTitle:     patch.NodeTitle,
		RecoveryX:         patch.NodePositionX,
		RecoveryY:         patch.NodePositionY,
	})
	if err != nil {
		return taskbinding.NodeBindResult{}, err
	}
	return taskbinding.NodeBindResult{Revision: result.Revision, Node: result.Node}, nil
}

func bindEffectKey(params json.RawMessage) string {
	var args struct {
		TaskID      string `json:"taskId"`
		NodeID      string `json:"nodeId"`
		OutputIndex int    `json:"outputIndex"`
	}
	if json.Unmarshal(params, &args) != nil {
		return ""
	}
	taskID := strings.TrimSpace(args.TaskID)
	nodeID := strings.TrimSpace(args.NodeID)
	if taskID == "" || nodeID == "" {
		return ""
	}
	return localtask.AttachNodeEffectKey(taskID, nodeID, args.OutputIndex)
}

func recoverNodeEffectKey(params json.RawMessage) string {
	var args struct {
		TaskID      string `json:"taskId"`
		OutputIndex int    `json:"outputIndex"`
	}
	if json.Unmarshal(params, &args) != nil || strings.TrimSpace(args.TaskID) == "" {
		return ""
	}
	return localtask.RecoverNodeEffectKey(args.TaskID, args.OutputIndex)
}
