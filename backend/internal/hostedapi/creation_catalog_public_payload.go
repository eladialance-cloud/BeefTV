package hostedapi

import (
	"encoding/json"
	"strings"
)

// publicCreationTemplatePayload projects the server-owned workflow definition
// into the browser-safe operation graph. Prompt sources and prompt templates
// stay in the private catalog row and are resolved only when a generation task
// is admitted on the server.
func publicCreationTemplatePayload(raw string) (map[string]any, bool) {
	var source map[string]any
	if json.Unmarshal([]byte(raw), &source) != nil || source == nil {
		return nil, false
	}
	public := map[string]any{}
	for _, key := range []string{
		"schema", "schemaVersion", "version", "id", "name", "category", "badge",
		"scene", "canvasType", "summary", "description",
	} {
		if value, ok := source[key]; ok {
			public[key] = value
		}
	}
	if setup := publicSetupForm(source["forms"]); setup != nil {
		public["forms"] = map[string]any{"setup": setup}
	}
	if steps := publicWorkflowSteps(source["steps"]); len(steps) > 0 {
		public["steps"] = steps
		public["stepCount"] = len(steps)
	}
	if canvas := publicCanvasGraph(source["canvas"]); canvas != nil {
		public["canvas"] = canvas
	} else if canvas := publicCanvasGraph(map[string]any{
		"nodes":       source["nodes"],
		"connections": source["edges"],
	}); canvas != nil {
		public["canvas"] = canvas
	}
	return public, true
}

func publicSetupForm(value any) map[string]any {
	forms, _ := value.(map[string]any)
	setup, _ := forms["setup"].(map[string]any)
	if setup == nil {
		return nil
	}
	result := map[string]any{}
	for _, key := range []string{"enabled", "title", "description"} {
		if value, ok := setup[key]; ok {
			result[key] = value
		}
	}
	steps := make([]any, 0)
	for _, rawStep := range anySlice(setup["steps"]) {
		step, _ := rawStep.(map[string]any)
		if step == nil {
			continue
		}
		fields := make([]any, 0)
		for _, rawField := range anySlice(step["fields"]) {
			field, _ := rawField.(map[string]any)
			if field == nil {
				continue
			}
			publicField := map[string]any{}
			for _, key := range []string{"key", "type", "label", "description", "placeholder", "required", "defaultValue", "options", "minSelections", "maxSelections", "uploadKind", "accept"} {
				if value, ok := field[key]; ok {
					publicField[key] = publicFormOptions(value)
				}
			}
			if len(publicField) > 0 {
				fields = append(fields, publicField)
			}
		}
		if len(fields) == 0 {
			continue
		}
		publicStep := map[string]any{"fields": fields}
		for _, key := range []string{"id", "title", "description"} {
			if value, ok := step[key]; ok {
				publicStep[key] = value
			}
		}
		steps = append(steps, publicStep)
	}
	if len(steps) == 0 {
		return nil
	}
	result["steps"] = steps
	return result
}

func publicFormOptions(value any) any {
	options, ok := value.([]any)
	if !ok {
		return value
	}
	result := make([]any, 0, len(options))
	for _, rawOption := range options {
		switch option := rawOption.(type) {
		case string, float64, bool:
			result = append(result, option)
		case map[string]any:
			public := map[string]any{}
			for _, key := range []string{"label", "value", "description"} {
				if item, ok := option[key]; ok {
					public[key] = item
				}
			}
			if len(public) > 0 {
				result = append(result, public)
			}
		}
	}
	return result
}

func publicWorkflowSteps(value any) []any {
	result := make([]any, 0)
	for _, rawStep := range anySlice(value) {
		step, _ := rawStep.(map[string]any)
		if step == nil {
			continue
		}
		public := map[string]any{}
		for _, key := range []string{"id", "title", "action", "mode", "outputs"} {
			if item, ok := step[key]; ok {
				public[key] = item
			}
		}
		if len(public) > 0 {
			result = append(result, public)
		}
	}
	return result
}

func publicCanvasGraph(value any) map[string]any {
	canvas, _ := value.(map[string]any)
	if canvas == nil {
		return nil
	}
	result := map[string]any{
		"nodes":        publicCanvasNodes(canvas["nodes"]),
		"connections":  publicCanvasConnections(canvas["connections"]),
		"dynamicNodes": publicCanvasDynamicNodes(canvas["dynamicNodes"]),
		"layout":       canvas["layout"],
	}
	if strings.TrimSpace(toString(result["layout"])) == "" {
		delete(result, "layout")
	}
	if len(anySlice(result["nodes"])) == 0 {
		return nil
	}
	return result
}

func publicCanvasNodes(value any) []any {
	result := make([]any, 0)
	for _, rawNode := range anySlice(value) {
		node, _ := rawNode.(map[string]any)
		if node == nil {
			continue
		}
		public := map[string]any{}
		for _, key := range []string{"id", "ref", "type", "title", "parentId", "x", "y", "w", "h", "width", "height", "layout"} {
			if item, ok := node[key]; ok {
				public[key] = item
			}
		}
		if metadata := publicCanvasMetadata(node["metadata"]); len(metadata) > 0 {
			public["metadata"] = metadata
		}
		if data := publicCanvasMetadata(node["data"]); len(data) > 0 {
			public["data"] = data
		}
		result = append(result, public)
	}
	return result
}

func publicCanvasDynamicNodes(value any) []any {
	result := make([]any, 0)
	for _, rawNode := range anySlice(value) {
		node, _ := rawNode.(map[string]any)
		if node == nil {
			continue
		}
		public := map[string]any{}
		for _, key := range []string{"ref", "type", "per", "createdByStep", "titleTemplate", "parentId", "width", "height", "layout", "dependsOn", "connectsTo"} {
			if item, ok := node[key]; ok {
				public[key] = item
			}
		}
		if metadata := publicCanvasMetadata(node["metadata"]); len(metadata) > 0 {
			public["metadata"] = metadata
		}
		result = append(result, public)
	}
	return result
}

func publicCanvasConnections(value any) []any {
	result := make([]any, 0)
	for _, rawConnection := range anySlice(value) {
		connection, _ := rawConnection.(map[string]any)
		if connection == nil {
			continue
		}
		public := map[string]any{}
		for _, key := range []string{"id", "from", "to", "fromNodeId", "toNodeId", "type", "label", "fromHandleId", "toHandleId", "relation"} {
			if item, ok := connection[key]; ok {
				public[key] = item
			}
		}
		result = append(result, public)
	}
	return result
}

func publicCanvasMetadata(value any) map[string]any {
	metadata, _ := value.(map[string]any)
	if metadata == nil {
		return nil
	}
	public := map[string]any{}
	for _, key := range []string{
		"status", "workflowKind", "workflowTitle", "workflowDescription",
		"content", "contentTemplate", "description", "nodeRole", "resultOrigin",
		"generationMode", "generationType", "videoEditOperation", "size", "quality",
		"count", "seconds", "vquality", "generateAudio", "watermark", "frame",
		"assetCategory", "uploadField",
	} {
		if item, ok := metadata[key]; ok {
			public[key] = item
		}
	}
	return public
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func toString(value any) string {
	text, _ := value.(string)
	return text
}
