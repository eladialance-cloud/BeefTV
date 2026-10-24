package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

var creationPromptVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

type CreationPromptPreview struct {
	DefaultPrompt string         `json:"defaultPrompt"`
	NodeRef       string         `json:"nodeRef"`
	DynastyID     string         `json:"dynastyId"`
	Settings      map[string]any `json:"settings"`
}

func (s *Service) PreviewCreationPrompt(templateID string, nodeRef string, dynastyID string, variables map[string]any) (*CreationPromptPreview, error) {
	if s == nil || s.repo == nil {
		return nil, BadAuthRequest("工作流模板服务不可用")
	}
	templateID = strings.TrimSpace(templateID)
	nodeRef = strings.TrimSpace(nodeRef)
	dynastyID = strings.TrimSpace(dynastyID)
	if templateID == "" || nodeRef == "" {
		return nil, BadAuthRequest("提示词预览缺少模板或节点")
	}
	item, err := s.repo.CreationCatalogItem(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, BadAuthRequest("工作流模板不存在或已删除")
		}
		return nil, err
	}
	if item.Kind != CreationCatalogKindWorkflow || !item.Enabled {
		return nil, BadAuthRequest("工作流模板已停用或类型不合法")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(item.PayloadJSON), &payload); err != nil {
		return nil, BadAuthRequest("工作流模板定义不可用，请联系管理员修复")
	}
	prompt, settings, err := resolveCreationPromptPayload(payload, nodeRef, dynastyID, variables)
	if err != nil {
		return nil, err
	}
	return &CreationPromptPreview{
		DefaultPrompt: prompt,
		NodeRef:       nodeRef,
		DynastyID:     dynastyID,
		Settings:      settings,
	}, nil
}

// applyCreationPrompt resolves prompt text from the private workflow catalog
// only after task admission. The browser sends a stable template/node/dynasty
// reference; prompt text is never part of the public template payload.
func (s *Service) applyCreationPrompt(request CreateTaskRequest) (CreateTaskRequest, error) {
	if s == nil || s.repo == nil || request.Input == nil {
		return request, nil
	}
	metadata, _ := request.Input["metadata"].(map[string]any)
	reference, _ := metadata["creationTemplate"].(map[string]any)
	if len(reference) == 0 {
		return request, nil
	}
	templateID := creationString(reference["id"])
	nodeRef := creationString(reference["nodeRef"])
	if templateID == "" || nodeRef == "" {
		return request, BadAuthRequest("工作流生成引用无效，请重新创建流程画布")
	}
	item, err := s.repo.CreationCatalogItem(templateID)
	if err != nil {
		return request, BadAuthRequest("工作流模板不存在或已删除，请重新加载模板")
	}
	if item.Kind != CreationCatalogKindWorkflow || !item.Enabled {
		return request, BadAuthRequest("工作流模板已停用，请重新选择可用模板")
	}
	var payload map[string]any
	if json.Unmarshal([]byte(item.PayloadJSON), &payload) != nil {
		return request, BadAuthRequest("工作流模板定义不可用，请联系管理员修复")
	}
	variables := creationTemplateVariables(reference["variables"])
	dynastyID := creationString(reference["dynastyId"])
	prompt, _, err := resolveFinalCreationPrompt(payload, nodeRef, dynastyID, variables, creationString(reference["promptOverride"]))
	if err != nil {
		return request, err
	}

	input := cloneCreationInput(request.Input)
	input["prompt"] = prompt
	publicMetadata, _ := input["metadata"].(map[string]any)
	publicReference := map[string]any{
		"id":      templateID,
		"key":     item.ItemKey,
		"nodeRef": nodeRef,
	}
	if dynastyID != "" {
		publicReference["dynastyId"] = dynastyID
	}
	publicMetadata["creationTemplate"] = publicReference
	input["metadata"] = publicMetadata
	request.Input = input
	if strings.TrimSpace(request.Prompt) == "" {
		request.Prompt = "工作流生成"
	}
	return request, nil
}

func privateWorkflowPromptTemplate(payload map[string]any, nodeRef string) string {
	node := privateWorkflowPromptNode(payload, nodeRef)
	metadata, _ := node["metadata"].(map[string]any)
	return creationString(metadata["promptTemplate"])
}

func privateWorkflowPromptNode(payload map[string]any, nodeRef string) map[string]any {
	canvas, _ := payload["canvas"].(map[string]any)
	for _, key := range []string{"dynamicNodes", "nodes"} {
		for _, rawNode := range creationAnySlice(canvas[key]) {
			node, _ := rawNode.(map[string]any)
			ref := creationString(firstCreationValue(node["ref"], node["id"]))
			if ref != nodeRef {
				continue
			}
			return node
		}
	}
	return nil
}

func resolveCreationPromptPayload(
	payload map[string]any,
	nodeRef string,
	dynastyID string,
	variables map[string]any,
) (string, map[string]any, error) {
	template := privateWorkflowPromptTemplate(payload, nodeRef)
	if strings.TrimSpace(template) == "" {
		return "", nil, BadAuthRequest("当前流程节点没有可用的服务端生成规则，请联系管理员")
	}
	resolvedVariables := creationTemplateVariables(variables)
	if customDesc := creationString(resolvedVariables["customDesc"]); customDesc != "" {
		resolvedVariables["personalStoryClause"] = "Their personal story: " + customDesc + "."
	} else {
		resolvedVariables["personalStoryClause"] = ""
	}
	settings := map[string]any{}
	dynastyID = strings.TrimSpace(dynastyID)
	if dynastyID != "" {
		dynasty, ok := privateWorkflowDynasty(payload, dynastyID)
		if !ok {
			return "", nil, BadAuthRequest("所选朝代不在当前工作流模板中，请重新选择")
		}
		resolvedVariables["dynasty"] = dynasty
		if value, ok := dynasty["videoSettings"].(map[string]any); ok {
			settings = cloneCreationMap(value)
		}
	}
	prompt := creationPromptVariablePattern.ReplaceAllStringFunc(template, func(token string) string {
		match := creationPromptVariablePattern.FindStringSubmatch(token)
		if len(match) != 2 {
			return ""
		}
		return creationPromptVariableText(resolveCreationPromptVariable(resolvedVariables, match[1]))
	})
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", nil, BadAuthRequest("当前流程节点的生成规则为空，请联系管理员")
	}
	return prompt, settings, nil
}

func resolveFinalCreationPrompt(
	payload map[string]any,
	nodeRef string,
	dynastyID string,
	variables map[string]any,
	promptOverride string,
) (string, map[string]any, error) {
	prompt, settings, err := resolveCreationPromptPayload(payload, nodeRef, dynastyID, variables)
	if err != nil {
		return "", nil, err
	}
	promptOverride = strings.TrimSpace(promptOverride)
	if promptOverride == "" {
		return prompt, settings, nil
	}
	node := privateWorkflowPromptNode(payload, nodeRef)
	metadata, _ := node["metadata"].(map[string]any)
	allowed, _ := metadata["allowProducerPromptOverride"].(bool)
	if !allowed {
		return "", nil, BadAuthRequest("当前流程节点不允许制作方覆盖提示词")
	}
	if utf8.RuneCountInString(promptOverride) > 12000 {
		return "", nil, BadAuthRequest("提示词不能超过 12000 个字符")
	}
	return promptOverride, settings, nil
}

func privateWorkflowDynasty(payload map[string]any, dynastyID string) (map[string]any, bool) {
	for _, rawDynasty := range creationAnySlice(payload["dynastyCatalog"]) {
		dynasty, _ := rawDynasty.(map[string]any)
		if creationString(dynasty["id"]) == dynastyID {
			return cloneCreationMap(dynasty), true
		}
	}
	return nil, false
}

func creationTemplateVariables(value any) map[string]any {
	record, _ := value.(map[string]any)
	if record == nil {
		return map[string]any{}
	}
	return cloneCreationMap(record)
}

func cloneCreationInput(input map[string]any) map[string]any {
	cloned := cloneCreationMap(input)
	if metadata, ok := cloned["metadata"].(map[string]any); ok {
		cloned["metadata"] = cloneCreationMap(metadata)
	}
	return cloned
}

func cloneCreationMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = cloneCreationValue(value)
	}
	return result
}

func cloneCreationValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		return cloneCreationMap(item)
	case []any:
		cloned := make([]any, len(item))
		for index, child := range item {
			cloned[index] = cloneCreationValue(child)
		}
		return cloned
	default:
		return item
	}
}

func resolveCreationPromptVariable(values map[string]any, path string) any {
	var current any = values
	for _, segment := range strings.Split(path, ".") {
		record, _ := current.(map[string]any)
		if record == nil {
			return nil
		}
		current = record[segment]
	}
	return current
}

func creationPromptVariableText(value any) string {
	switch item := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(item)
	case json.Number:
		return item.String()
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", item), "0"), ".")
	case bool:
		if item {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(item))
		for _, child := range item {
			if text := creationPromptVariableText(child); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(item))
	}
}

func firstCreationValue(values ...any) any {
	for _, value := range values {
		if value != nil && creationString(value) != "" {
			return value
		}
	}
	return ""
}

func creationAnySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func creationString(value any) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
