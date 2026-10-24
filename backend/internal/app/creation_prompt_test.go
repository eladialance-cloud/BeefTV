package app

import (
	"encoding/json"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/model"
	localtask "infinite-canvas/backend/internal/task"
)

func TestPreviewCreationPromptResolvesVideoPromptAndSettings(t *testing.T) {
	payload := map[string]any{
		"dynastyCatalog": []any{map[string]any{
			"id":          "tang",
			"videoPrompt": "0-2s: lift the veil. 2-5s: share the cup. 5-8s: orbit.",
			"videoSettings": map[string]any{
				"duration": float64(8),
				"ratio":    "16:9",
			},
		}},
		"canvas": map[string]any{"dynamicNodes": []any{map[string]any{
			"ref": "dynastyVideo",
			"metadata": map[string]any{
				"promptTemplate":              "{{dynasty.videoPrompt}} Story: {{customDesc}}.",
				"allowProducerPromptOverride": true,
			},
		}}},
	}

	prompt, settings, err := resolveCreationPromptPayload(payload, "dynastyVideo", "tang", map[string]any{
		"customDesc": "西湖相识",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "0-2s: lift the veil. 2-5s: share the cup. 5-8s: orbit. Story: 西湖相识." {
		t.Fatalf("prompt = %q", prompt)
	}
	if settings["duration"] != float64(8) || settings["ratio"] != "16:9" {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestApplyCreationPromptResolvesServerPrivatePromptAndRedactsReference(t *testing.T) {
	svc, db := newChannelModelTestService(t)
	if err := db.AutoMigrate(&model.CreationCatalogItem{}); err != nil {
		t.Fatal(err)
	}
	item := model.CreationCatalogItem{
		ID:      "template-cross-era",
		Kind:    CreationCatalogKindWorkflow,
		ItemKey: "cross-era-wedding",
		Name:    "跨时空婚礼电影",
		Enabled: true,
		PayloadJSON: `{
			"dynastyCatalog":[{
				"id":"tang",
				"name":"唐",
				"portraitPromptMale":"Tang groom in a grand wedding hall"
			}],
			"canvas":{"dynamicNodes":[{
				"ref":"dynastyPortraitMale",
				"metadata":{
					"promptTemplate":"{{dynasty.portraitPromptMale}}. Keep exact face. Story: {{customDesc}}.",
					"promptSourceKeys":["private-source"]
				}
			}]}
		}`,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	request := CreateTaskRequest{
		Type:   "canvas_image",
		Prompt: "唐 · 新郎肖像",
		Input: map[string]any{
			"mode":   "image",
			"prompt": "",
			"metadata": map[string]any{
				"sourceNodeId": "portrait-node",
				"creationTemplate": map[string]any{
					"id":        item.ID,
					"key":       item.ItemKey,
					"nodeRef":   "dynastyPortraitMale",
					"dynastyId": "tang",
					"variables": map[string]any{
						"customDesc": "西湖相识",
						"dynasties":  []any{"tang"},
					},
				},
			},
		},
	}
	resolved, err := svc.applyCreationPrompt(request)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Prompt != "唐 · 新郎肖像" {
		t.Fatalf("public task prompt = %q", resolved.Prompt)
	}
	hiddenPrompt := stringValue(resolved.Input["prompt"])
	if hiddenPrompt != "Tang groom in a grand wedding hall. Keep exact face. Story: 西湖相识." {
		t.Fatalf("resolved hidden prompt = %q", hiddenPrompt)
	}
	raw, err := json.Marshal(resolved.Input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "variables") {
		t.Fatalf("private template variables must be redacted from durable task metadata: %s", raw)
	}
	if !strings.Contains(string(raw), `"nodeRef":"dynastyPortraitMale"`) {
		t.Fatalf("safe prompt reference missing: %s", raw)
	}
}

func TestResolveFinalCreationPromptUsesProducerOverrideOnAllowedNode(t *testing.T) {
	payload := map[string]any{
		"dynastyCatalog": []any{map[string]any{
			"id":          "tang",
			"videoPrompt": "DEFAULT VIDEO PROMPT",
		}},
		"canvas": map[string]any{"dynamicNodes": []any{map[string]any{
			"ref": "dynastyVideo",
			"metadata": map[string]any{
				"promptTemplate":              "{{dynasty.videoPrompt}}",
				"allowProducerPromptOverride": true,
			},
		}}},
	}
	prompt, _, err := resolveFinalCreationPrompt(
		payload,
		"dynastyVideo",
		"tang",
		map[string]any{},
		"PRODUCER EDITED VIDEO PROMPT",
	)
	if err != nil {
		t.Fatal(err)
	}
	if prompt != "PRODUCER EDITED VIDEO PROMPT" {
		t.Fatalf("hidden prompt = %q", prompt)
	}
}

func TestResolveFinalCreationPromptRejectsOverrideOnNodeWithoutPermission(t *testing.T) {
	payload := map[string]any{
		"dynastyCatalog": []any{map[string]any{
			"id":                 "tang",
			"portraitPromptMale": "DEFAULT PORTRAIT",
		}},
		"canvas": map[string]any{"dynamicNodes": []any{map[string]any{
			"ref":      "dynastyPortraitMale",
			"metadata": map[string]any{"promptTemplate": "{{dynasty.portraitPromptMale}}"},
		}}},
	}
	_, _, err := resolveFinalCreationPrompt(
		payload,
		"dynastyPortraitMale",
		"tang",
		map[string]any{},
		"SHOULD NOT BE ALLOWED",
	)
	if err == nil || !strings.Contains(err.Error(), "不允许制作方覆盖提示词") {
		t.Fatalf("override permission error = %v", err)
	}
}

func TestApplyCreationPromptRejectsUnknownPrivateNode(t *testing.T) {
	svc, db := newChannelModelTestService(t)
	if err := db.AutoMigrate(&model.CreationCatalogItem{}); err != nil {
		t.Fatal(err)
	}
	item := model.CreationCatalogItem{
		ID:          "template-cross-era",
		Kind:        CreationCatalogKindWorkflow,
		ItemKey:     "cross-era-wedding",
		Name:        "跨时空婚礼电影",
		Enabled:     true,
		PayloadJSON: `{"canvas":{"dynamicNodes":[{"ref":"known","metadata":{"promptTemplate":"PRIVATE"}}]}}`,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.applyCreationPrompt(CreateTaskRequest{
		Type: "canvas_image",
		Input: map[string]any{
			"mode": "image",
			"metadata": map[string]any{
				"creationTemplate": map[string]any{
					"id":      item.ID,
					"nodeRef": "unknown",
				},
			},
		},
	})
	if err == nil || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("unknown prompt reference error = %v", err)
	}
}

func TestCreateLocalTaskResolvesCreationPromptBeforeAdmission(t *testing.T) {
	svc, db := newChannelModelTestService(t)
	if err := db.AutoMigrate(&model.CreationCatalogItem{}); err != nil {
		t.Fatal(err)
	}
	item := model.CreationCatalogItem{
		ID:          "template-private-route",
		Kind:        CreationCatalogKindWorkflow,
		ItemKey:     "private-route",
		Name:        "私有流程",
		Enabled:     true,
		PayloadJSON: `{"canvas":{"dynamicNodes":[{"ref":"portrait","metadata":{"promptTemplate":"PRIVATE ROUTE PROMPT"}}]}}`,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.CreateLocalTask("user", localtask.CreateRequest{
		Type:   "invalid_private_prompt_probe",
		Prompt: "",
		Input: map[string]any{
			"mode":   "image",
			"prompt": "",
			"metadata": map[string]any{
				"creationTemplate": map[string]any{
					"id":      item.ID,
					"key":     item.ItemKey,
					"nodeRef": "portrait",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("invalid task type unexpectedly succeeded")
	}
	if strings.Contains(err.Error(), localtask.PromptRequiredMessage) {
		t.Fatalf("standalone task admission bypassed private prompt resolution: %v", err)
	}
	if !strings.Contains(err.Error(), "不支持的任务类型") {
		t.Fatalf("task did not reach type validation after prompt resolution: %v", err)
	}
}
