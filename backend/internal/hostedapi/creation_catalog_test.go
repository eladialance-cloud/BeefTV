package hostedapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type fakeCreationCatalogService struct {
	publicKind  string
	publicScene string
	previewID   string
	previewNode string
	previewEra  string
}

func (f *fakeCreationCatalogService) AdminCreationCatalogPage(*model.User, string, app.AdminListQuery) (*app.CreationCatalogPage, error) {
	return &app.CreationCatalogPage{}, nil
}

func (f *fakeCreationCatalogService) CreateCreationCatalogItem(*model.User, app.CreationCatalogRequest) (*model.CreationCatalogItem, error) {
	return nil, nil
}

func (f *fakeCreationCatalogService) UpdateCreationCatalogItem(*model.User, string, app.CreationCatalogRequest) (*model.CreationCatalogItem, error) {
	return nil, nil
}

func (f *fakeCreationCatalogService) DeleteCreationCatalogItem(*model.User, string) error {
	return nil
}

func (f *fakeCreationCatalogService) PublicCreationCatalog(kind string, scene string) ([]model.CreationCatalogItem, error) {
	f.publicKind = kind
	f.publicScene = scene
	return []model.CreationCatalogItem{{
		ID:          "template-1",
		Kind:        app.CreationCatalogKindWorkflow,
		ItemKey:     "wedding-workflow",
		Name:        "婚礼视频流程",
		Description: "从故事到成片",
		CanvasType:  "node",
		PayloadJSON: `{
			"schema":"visideep.workflow-template/v2",
			"version":"2.0.0",
			"forms":{"setup":{"steps":[{"id":"dynasty","title":"选择朝代","fields":[{"key":"dynasties","type":"checkboxGroup","label":"朝代","minSelections":2,"maxSelections":5,"options":[{"label":"唐 · 大唐盛世","value":"tang"}]}]}]}},
			"promptSources":[{"key":"secret","content":"TOP SECRET PROMPT SOURCE"}],
			"dynastyCatalog":[{"id":"tang","name":"唐","portraitPromptMale":"TOP SECRET PORTRAIT","couplePrompt":"TOP SECRET COUPLE","videoPrompt":"TOP SECRET VIDEO"}],
			"canvas":{
				"nodes":[
					{"ref":"W01","type":"text","title":"故事采集","metadata":{"status":"success","content":"可公开操作说明","promptTemplate":"TOP SECRET STATIC","pluginData":{"promptSourceKeys":["secret"]}}}
				],
				"dynamicNodes":[
					{"ref":"portrait","type":"image","titleTemplate":"{{dynasty.name}} · 肖像","dependsOn":["W01"],"metadata":{"generationMode":"image","promptTemplate":"TOP SECRET DYNAMIC","model":"internal-model"}}
				],
				"connections":[]
			}
		}`,
		Enabled:   true,
		CreatedBy: "admin-secret",
	}}, nil
}

func (f *fakeCreationCatalogService) PreviewCreationPrompt(templateID string, nodeRef string, dynastyID string, _ map[string]any) (*app.CreationPromptPreview, error) {
	f.previewID = templateID
	f.previewNode = nodeRef
	f.previewEra = dynastyID
	return &app.CreationPromptPreview{
		DefaultPrompt: "Resolved video prompt",
		NodeRef:       nodeRef,
		DynastyID:     dynastyID,
		Settings:      map[string]any{"duration": float64(8), "ratio": "16:9"},
	}, nil
}

func TestPublicCreationTemplatesReturnsEnabledPayloadWithoutAdminFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeCreationCatalogService{}
	engine := gin.New()
	RegisterCreationCatalogPublicRoutes(engine.Group("/api"), service)

	request := httptest.NewRequest(http.MethodGet, "/api/creation-templates?kind=workflow&scene=general", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("公开模板接口应返回 200，得到 %d (%s)", recorder.Code, recorder.Body.String())
	}
	if service.publicKind != app.CreationCatalogKindWorkflow || service.publicScene != "general" {
		t.Fatalf("筛选参数未传给服务：kind=%q scene=%q", service.publicKind, service.publicScene)
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID      string         `json:"id"`
				ItemKey string         `json:"itemKey"`
				Name    string         `json:"name"`
				Payload map[string]any `json:"payload"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, recorder.Body.String())
	}
	if response.Code != 0 || len(response.Data.Items) != 1 {
		t.Fatalf("公开模板响应不符: %#v", response)
	}
	item := response.Data.Items[0]
	if item.ID != "template-1" || item.ItemKey != "wedding-workflow" || item.Name != "婚礼视频流程" {
		t.Fatalf("公开模板字段不符: %#v", item)
	}
	canvas, ok := item.Payload["canvas"].(map[string]any)
	if !ok {
		t.Fatalf("payload 必须返回可公开的画布对象: %#v", item.Payload)
	}
	nodes, ok := canvas["nodes"].([]any)
	if !ok || len(nodes) != 1 {
		t.Fatalf("公开画布节点缺失: %#v", canvas["nodes"])
	}
	node := nodes[0].(map[string]any)
	metadata := node["metadata"].(map[string]any)
	if _, leaked := metadata["promptTemplate"]; leaked {
		t.Fatalf("公开节点不能包含 promptTemplate: %#v", metadata)
	}
	if _, leaked := metadata["pluginData"]; leaked {
		t.Fatalf("公开节点不能包含插件私有数据: %#v", metadata)
	}
	forms := item.Payload["forms"].(map[string]any)
	setup := forms["setup"].(map[string]any)
	formSteps := setup["steps"].([]any)
	formFields := formSteps[0].(map[string]any)["fields"].([]any)
	formField := formFields[0].(map[string]any)
	if formField["minSelections"] != float64(2) || formField["maxSelections"] != float64(5) {
		t.Fatalf("公开表单必须保留朝代选择边界: %#v", formField)
	}
	bytes := recorder.Body.String()
	if bytes == "" || strings.Contains(bytes, "admin-secret") || strings.Contains(bytes, "createdBy") || strings.Contains(bytes, "payloadJson") {
		t.Fatalf("公开响应泄漏内部字段或未返回解析后的 payload: %s", bytes)
	}
	for _, secret := range []string{
		"promptSources", "dynastyCatalog", "promptTemplate", "promptSourceKeys",
		"portraitPromptMale", "portraitPromptFemale", "couplePrompt", "videoPrompt",
		"TOP SECRET", "internal-model",
	} {
		if strings.Contains(bytes, secret) {
			t.Fatalf("公开响应泄漏提示词或内部模型字段 %q: %s", secret, bytes)
		}
	}
}

func TestPreviewCreationPromptRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeCreationCatalogService{}
	engine := gin.New()
	RegisterCreationCatalogPublicRoutes(engine.Group("/api"), service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/creation-templates/template-1/prompt-preview",
		strings.NewReader(`{"nodeRef":"dynastyVideo","dynastyId":"tang","variables":{"customDesc":"西湖相识"}}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("提示词预览应返回 200，得到 %d (%s)", recorder.Code, recorder.Body.String())
	}
	if service.previewID != "template-1" || service.previewNode != "dynastyVideo" || service.previewEra != "tang" {
		t.Fatalf("预览参数未传给服务: %#v", service)
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			DefaultPrompt string         `json:"defaultPrompt"`
			NodeRef       string         `json:"nodeRef"`
			DynastyID     string         `json:"dynastyId"`
			Settings      map[string]any `json:"settings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 0 || response.Data.DefaultPrompt != "Resolved video prompt" ||
		response.Data.NodeRef != "dynastyVideo" || response.Data.DynastyID != "tang" {
		t.Fatalf("提示词预览响应不符: %#v", response)
	}
}

func TestBuiltCrossEraWeddingTemplateProjectionContainsNoPrompts(t *testing.T) {
	raw, err := os.ReadFile("../../../build/cross-era-wedding-volc.workflow.v1.json")
	if os.IsNotExist(err) {
		t.Skip("generated workflow artifact is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	payload, ok := publicCreationTemplatePayload(string(raw))
	if !ok {
		t.Fatal("generated workflow could not be projected")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"promptSources", "dynastyCatalog", "promptTemplate", "promptSourceKeys",
		"portraitPromptMale", "portraitPromptFemale", "couplePrompt", "videoPrompt",
		"doubao-seedream", "doubao-seedance",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("built template public projection leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestBuiltCrossEraWeddingTemplateAllowsProducerVideoPromptOverride(t *testing.T) {
	raw, err := os.ReadFile("../../../build/cross-era-wedding-volc.workflow.v1.json")
	if os.IsNotExist(err) {
		t.Skip("generated workflow artifact is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Canvas struct {
			DynamicNodes []struct {
				Ref      string         `json:"ref"`
				Metadata map[string]any `json:"metadata"`
			} `json:"dynamicNodes"`
		} `json:"canvas"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, node := range payload.Canvas.DynamicNodes {
		if node.Ref != "dynastyVideo" {
			continue
		}
		if allowed, _ := node.Metadata["allowProducerPromptOverride"].(bool); !allowed {
			t.Fatalf("dynastyVideo must allow producer prompt override: %#v", node.Metadata)
		}
		return
	}
	t.Fatal("dynastyVideo node not found")
}
