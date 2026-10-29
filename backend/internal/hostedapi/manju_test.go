package hostedapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/kernel"
	manjusvc "infinite-canvas/backend/internal/manju"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	localtask "infinite-canvas/backend/internal/task"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testManjuOwnerID    = "manju-owner"
	testManjuWriterID   = "manju-writer"
	testManjuOutsiderID = "manju-outsider"
)

func newManjuHTTPEnv(t *testing.T) (*gin.Engine, *manjusvc.Service) {
	t.Helper()
	engine, service, _ := newManjuHTTPEnvWithDB(t)
	return engine, service
}

func newManjuHTTPEnvWithDB(t *testing.T) (*gin.Engine, *manjusvc.Service, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "manju-api.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(database.HostedModels()...); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Project{}, &model.ManjuProjectMember{}, &model.ManjuLockReceipt{}, &model.ManjuProjectModelBinding{},
		&model.ManjuProjectStateSnapshot{},
		&model.CanvasProject{}, &model.ManjuCanvasImport{},
		&model.ManjuScriptRun{}, &model.ManjuScriptStage{}, &model.ManjuScriptSource{}, &model.ManjuScriptArtifact{},
		&model.ManjuAssetRun{}, &model.ManjuAssetStage{}, &model.ManjuAssetArtifact{}, &model.ManjuAssetReference{},
		&model.ManjuProductionRun{}, &model.ManjuProductionStage{}, &model.ManjuProductionArtifact{}, &model.ManjuProductionPrevis{},
		&model.ManjuProductionImage{}, &model.ManjuProductionVideo{}, &model.Result{}, &model.Resource{},
		&model.ModelChannel{}, &model.ChannelModel{},
		&model.Task{},
	); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: testManjuOwnerID, Username: "owner", DisplayName: "主理人", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: testManjuWriterID, Username: "writer", DisplayName: "编剧", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: testManjuOutsiderID, Username: "outsider", DisplayName: "外部用户", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Project{
		ID: "project-secret", UserID: testManjuOwnerID, Name: "不可泄露的项目", Type: "manju", Status: model.ProjectStatusActive, Revision: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	channel := model.ModelChannel{ID: "channel-main", Scope: model.ChannelScopeSystem, Enabled: true, Name: "画布主渠道", APIFormat: "openai"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	channelModels := []model.ChannelModel{
		{ID: "cm-text", ChannelID: channel.ID, ModelKey: "text-pro", DisplayName: "文本 Pro", Capability: "text", Enabled: true},
		{ID: "cm-image", ChannelID: channel.ID, ModelKey: "image-pro", DisplayName: "图片 Pro", Capability: "image", Enabled: true},
		{ID: "cm-video", ChannelID: channel.ID, ModelKey: "video-pro", DisplayName: "视频 Pro", Capability: "video", Enabled: true},
	}
	if err := db.Create(&channelModels).Error; err != nil {
		t.Fatal(err)
	}

	service := manjusvc.New(repository.New(db))
	service.SetSkillValidationEnabledForTest(false)
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader("X-Test-Manju-User") {
		case "owner":
			return &model.User{ID: testManjuOwnerID, Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		case "writer":
			return &model.User{ID: testManjuWriterID, Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		case "outsider":
			return &model.User{ID: testManjuOutsiderID, Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	RegisterManjuRoutes(engine.Group("/api"), service, resolve)
	return engine, service, db
}

func manjuRequest(t *testing.T, engine *gin.Engine, method, path string, body any, actor string) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		request.Header.Set("X-Test-Manju-User", actor)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func confirmedManjuLockKeys(includeSeam bool) []string {
	keys := []string{
		manjusvc.LockTrack,
		manjusvc.LockSource,
		manjusvc.LockVideoModel,
		manjusvc.LockAspectRatio,
		manjusvc.LockDeliveryMode,
		manjusvc.LockStyleCard,
	}
	if includeSeam {
		keys = append(keys, manjusvc.LockSeamMode)
	}
	return keys
}

func TestManjuRoutesRequireSession(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects", nil, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问漫剧接口应 401，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuDeliverablesRouteReturnsPersistedDownloadMetadata(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	now := time.Now().UTC()
	rows := []model.ManjuDeliverable{
		{
			ID: "delivery-markdown", ProjectID: "project-secret", LockRevision: 3,
			ProductionRunID: "production-run-delivery", Kind: model.ManjuDeliverableKindMarkdown,
			FileName: "不可泄露的项目-P5最终交付.md", MimeType: "text/markdown; charset=utf-8",
			Content: "# 最终交付\n", ContentHash: strings.Repeat("a", 64), CreatedAt: now,
		},
		{
			ID: "delivery-manifest", ProjectID: "project-secret", LockRevision: 3,
			ProductionRunID: "production-run-delivery", Kind: model.ManjuDeliverableKindManifest,
			FileName: "不可泄露的项目-P5交付清单.json", MimeType: "application/json; charset=utf-8",
			Content: `{"version":"V7.4.26"}`, ContentHash: strings.Repeat("b", 64), CreatedAt: now,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	recorder := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/deliverables", nil, "owner")
	if recorder.Code != http.StatusOK {
		t.Fatalf("读取交付物应 200，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("交付物接口必须禁止缓存，得到 %q", recorder.Header().Get("Cache-Control"))
	}
	var payload struct {
		Data struct {
			Items []manjusvc.UserDeliverableView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Items) != 2 {
		t.Fatalf("deliverables = %d, want persisted markdown+manifest only: %+v", len(payload.Data.Items), payload.Data.Items)
	}
	if payload.Data.Items[0].Kind != model.ManjuDeliverableKindMarkdown ||
		payload.Data.Items[1].Kind != model.ManjuDeliverableKindManifest {
		t.Fatalf("unexpected persisted deliverable order: %+v", payload.Data.Items)
	}
	first := payload.Data.Items[0]
	if first.ProductionRunID != "production-run-delivery" ||
		first.FileName == "" ||
		first.MimeType != "text/markdown; charset=utf-8" ||
		first.ContentHash != strings.Repeat("a", 64) ||
		first.LockRevision != 3 ||
		!strings.Contains(first.Content, "最终交付") {
		t.Fatalf("deliverable download metadata is incomplete: %+v", first)
	}
}

func TestManjuTrendResolutionErrorIsBadRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	writeManjuError(context, fmt.Errorf("create run: %w", manjusvc.ErrTrendResolutionRequired))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing trend resolution must be a bad request, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuScriptArtifactRegenerationRouteRequiresSession(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(
		t,
		engine,
		http.MethodPost,
		"/api/manju/projects/project-secret/script-runs/run-1/stages/ip_development/regenerate-artifact",
		map[string]string{"instruction": "只重新生成当前工件"},
		"",
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("P1 单工件重新生成接口必须存在且要求登录，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuCanvasImportRoutesRequireSession(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	unauthorized := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/canvas-imports", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登录读取画布导入映射必须拒绝，得到 %d: %s", unauthorized.Code, unauthorized.Body.String())
	}
	owner := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/canvas-imports", nil, "owner")
	if owner.Code != http.StatusOK || !strings.Contains(owner.Body.String(), "items") {
		t.Fatalf("项目成员必须能读取画布导入映射，得到 %d: %s", owner.Code, owner.Body.String())
	}
}

func TestManjuOwnerCanDeleteProject(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)

	recorder := manjuRequest(t, engine, http.MethodDelete, "/api/manju/projects/project-secret", nil, "owner")
	if recorder.Code != http.StatusOK {
		t.Fatalf("主理人删除漫剧项目应 200，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"deleted":true`) || !strings.Contains(recorder.Body.String(), `"id":"project-secret"`) {
		t.Fatalf("删除响应必须带 deleted 和项目 ID: %s", recorder.Body.String())
	}
	var count int64
	if err := db.Model(&model.Project{}).Where("id = ?", "project-secret").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("删除后项目必须不存在，count=%d", count)
	}
}

func TestManjuDeleteProjectRequiresOwner(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	if err := db.Create(&model.ManjuProjectMember{
		ID: "writer-member", ProjectID: "project-secret", UserID: testManjuWriterID,
		Role: model.ManjuMemberRoleWriter, Status: model.ManjuMemberStatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}

	writer := manjuRequest(t, engine, http.MethodDelete, "/api/manju/projects/project-secret", nil, "writer")
	if writer.Code != http.StatusForbidden {
		t.Fatalf("项目成员不能删除漫剧项目，应 403，得到 %d: %s", writer.Code, writer.Body.String())
	}
	outsider := manjuRequest(t, engine, http.MethodDelete, "/api/manju/projects/project-secret", nil, "outsider")
	if outsider.Code != http.StatusNotFound {
		t.Fatalf("外部用户不能探测项目，应 404，得到 %d: %s", outsider.Code, outsider.Body.String())
	}
}

func TestManjuDeleteProjectRejectsActiveTasks(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	if err := db.Create(&model.Task{
		ID: "active-delete-task", UserID: testManjuOwnerID, ProjectID: "project-secret",
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}).Error; err != nil {
		t.Fatal(err)
	}

	recorder := manjuRequest(t, engine, http.MethodDelete, "/api/manju/projects/project-secret", nil, "owner")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("项目有进行中任务时必须拒绝删除，应 409，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "进行中的生成任务") {
		t.Fatalf("活动任务冲突必须给出明确中文说明: %s", recorder.Body.String())
	}
}

func TestManjuStyleCatalogRequiresSessionAndReturnsV7426Cards(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	unauthorized := manjuRequest(t, engine, http.MethodGet, "/api/manju/style-catalog", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登录读取画风目录应 401，得到 %d: %s", unauthorized.Code, unauthorized.Body.String())
	}

	recorder := manjuRequest(t, engine, http.MethodGet, "/api/manju/style-catalog", nil, "owner")
	if recorder.Code != http.StatusOK {
		t.Fatalf("登录后读取画风目录应 200，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Version string `json:"version"`
			Items   []struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
			} `json:"items"`
			Families []struct {
				ID string `json:"id"`
			} `json:"families"`
			Bases []struct {
				ID string `json:"id"`
			} `json:"bases"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Version != "V7.4.26" || len(payload.Data.Items) != 74 || len(payload.Data.Families) != 8 || len(payload.Data.Bases) != 4 {
		t.Fatalf("画风目录必须是 V7.4.26 的 74 卡/8 族/4 基底，得到 version=%s items=%d families=%d bases=%d", payload.Data.Version, len(payload.Data.Items), len(payload.Data.Families), len(payload.Data.Bases))
	}
	seen := map[string]bool{}
	for _, item := range payload.Data.Items {
		if seen[item.ID] || item.ID == "" {
			t.Fatalf("画风卡 ID 必须唯一且非空: %+v", item)
		}
		seen[item.ID] = true
		if item.Number == 67 || item.Number == 70 {
			t.Fatalf("67/70 已停用，不得回到目录: %+v", item)
		}
	}
}

func TestManjuRoutesDoNotLeakForeignProject(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret", nil, "outsider")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("外部项目必须返回 404，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("不可泄露的项目")) || bytes.Contains(recorder.Body.Bytes(), []byte("project-secret")) {
		t.Fatalf("外部项目响应泄露了项目数据: %s", recorder.Body.String())
	}
}

func TestManjuOwnerCanSaveAndReadP0Lock(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	body := map[string]any{
		"track":                 "domestic",
		"source":                "original",
		"videoModel":            "seedance-2.0",
		"aspectRatio":           "9:16",
		"deliveryMode":          "both",
		"targetDurationSeconds": 60,
		"styleBase":             "live-action",
		"styleCardId":           "sd-style-009",
		"assetMode":             "M1",
		"seamMode":              "tail-frame-freeze",
		"confirmedLocks":        confirmedManjuLockKeys(true),
	}

	saved := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/lock", body, "owner")
	if saved.Code != http.StatusOK {
		t.Fatalf("主理人保存 P0 应 200，得到 %d: %s", saved.Code, saved.Body.String())
	}
	if !strings.Contains(saved.Body.String(), "\"styleCardName\":\"徐克\"") || !strings.Contains(saved.Body.String(), "\"styleCatalogVersion\":\"V7.4.26\"") {
		t.Fatalf("P0 回执必须冻结完整画风卡元数据: %s", saved.Body.String())
	}
	var savePayload struct {
		Data struct {
			Item manjusvc.LockReceipt `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &savePayload); err != nil {
		t.Fatal(err)
	}
	if savePayload.Data.Item.Revision != 1 || savePayload.Data.Item.SegmentCount != 4 {
		t.Fatalf("P0 回执错误: %+v", savePayload.Data.Item)
	}

	read := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret", nil, "owner")
	if read.Code != http.StatusOK {
		t.Fatalf("主理人读取项目应 200，得到 %d: %s", read.Code, read.Body.String())
	}
	var readPayload struct {
		Data struct {
			Item manjusvc.ProjectDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &readPayload); err != nil {
		t.Fatal(err)
	}
	if readPayload.Data.Item.Lock == nil || readPayload.Data.Item.Lock.ContentHash == "" {
		t.Fatalf("项目详情必须返回服务端锁回执: %+v", readPayload.Data.Item.Lock)
	}
	if got := strings.Join(readPayload.Data.Item.Lock.Snapshot.ConfirmedLocks, ","); got != strings.Join(confirmedManjuLockKeys(true), ",") {
		t.Fatalf("项目详情必须返回显式锁选择凭据: %q", got)
	}
}

func TestManjuLockRejectsImplicitHardLockValues(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	body := map[string]any{
		"track":                 "domestic",
		"source":                "original",
		"videoModel":            "seedance-2.0",
		"aspectRatio":           "9:16",
		"deliveryMode":          "both",
		"targetDurationSeconds": 15,
		"styleBase":             "live-action",
		"styleCardId":           "sd-style-009",
		"assetMode":             "M1",
		"confirmedLocks": []string{
			manjusvc.LockTrack,
			manjusvc.LockSource,
			manjusvc.LockVideoModel,
			manjusvc.LockAspectRatio,
		},
	}

	recorder := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/lock", body, "owner")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("implicit P0 values must be rejected as a bad request, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func validManjuModelsBody() map[string]any {
	return map[string]any{
		"text":  map[string]string{"channelId": "channel-main", "modelKey": "text-pro"},
		"image": map[string]string{"channelId": "channel-main", "modelKey": "image-pro"},
		"video": map[string]string{"channelId": "channel-main", "modelKey": "video-pro"},
	}
}

func TestManjuOwnerCanSaveAndReadProjectModels(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	saved := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/models", validManjuModelsBody(), "owner")
	if saved.Code != http.StatusOK {
		t.Fatalf("主理人保存模型绑定应 200，得到 %d: %s", saved.Code, saved.Body.String())
	}

	read := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret", nil, "owner")
	if read.Code != http.StatusOK {
		t.Fatalf("主理人读取项目应 200，得到 %d: %s", read.Code, read.Body.String())
	}
	var payload struct {
		Data struct {
			Item manjusvc.ProjectDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Item.Models == nil ||
		payload.Data.Item.Models.Text.ModelKey != "text-pro" ||
		payload.Data.Item.Models.Image.ModelKey != "image-pro" ||
		payload.Data.Item.Models.Video.ModelKey != "video-pro" {
		t.Fatalf("项目详情必须返回三个模型槽位: %+v", payload.Data.Item.Models)
	}
}

func TestManjuProjectModelsRejectInvalidCapability(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	body := validManjuModelsBody()
	body["text"] = map[string]string{"channelId": "channel-main", "modelKey": "image-pro"}

	recorder := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/models", body, "owner")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("文本槽绑定图片模型应 400，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuWriterCannotChangeProjectModels(t *testing.T) {
	engine, service := newManjuHTTPEnv(t)
	if _, err := service.InviteMember(testManjuOwnerID, "project-secret", manjusvc.MemberInput{
		UserID: testManjuWriterID,
		Role:   model.ManjuMemberRoleWriter,
	}); err != nil {
		t.Fatal(err)
	}

	recorder := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/models", validManjuModelsBody(), "writer")

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("编剧修改模型绑定应 403，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuScriptSourceRouteDoesNotRequireRetiredP0BStage(t *testing.T) {
	engine, service := newManjuHTTPEnv(t)
	service.SetScriptTaskCreator(func(userID string, request localtask.CreateRequest) (*model.Task, error) {
		return &model.Task{
			ID: "task-script-blocked", UserID: userID, ProjectID: request.ProjectID,
			Type: request.Type, Status: model.TaskStatusQueued,
		}, nil
	})
	lockBody := map[string]any{
		"track": "domestic", "source": "adaptation", "videoModel": "seedance-2.0",
		"aspectRatio": "9:16", "deliveryMode": "both", "targetDurationSeconds": 60,
		"styleBase": "live-action", "styleCardId": "sd-style-009",
		"assetMode": "M1", "seamMode": "tail-frame-freeze",
		"confirmedLocks": confirmedManjuLockKeys(true),
	}
	if saved := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/lock", lockBody, "owner"); saved.Code != http.StatusOK {
		t.Fatalf("保存 P0 失败: %d %s", saved.Code, saved.Body.String())
	}

	created := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/script-runs", map[string]any{
		"mode": manjusvc.ScriptModeStepwise,
	}, "owner")
	if created.Code != http.StatusOK {
		t.Fatalf("创建剧本任务应 200，得到 %d: %s", created.Code, created.Body.String())
	}
	var createPayload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createPayload); err != nil {
		t.Fatal(err)
	}
	runID := createPayload.Data.Item.Run.ID
	if runID == "" || createPayload.Data.Item.Run.Branch != manjusvc.ScriptBranchAdaptation {
		t.Fatalf("P1 必须从 P0 推导小说改编支线: %+v", createPayload.Data.Item)
	}
	if len(createPayload.Data.Item.Stages) == 0 ||
		createPayload.Data.Item.Stages[0].Key != manjusvc.ScriptStageSourceRegistration {
		t.Fatalf("P0B 已移出 P1，来源登记必须保持为第一剧本阶段: %+v", createPayload.Data.Item.Stages)
	}
	blockedIP := manjuRequest(
		t, engine, http.MethodPost,
		"/api/manju/projects/project-secret/script-runs/"+runID+"/stages/"+manjusvc.ScriptStageIPDevelopment+"/run",
		map[string]any{}, "owner",
	)
	if blockedIP.Code != http.StatusConflict {
		t.Fatalf("来源未登记时 IP 阶段必须保持阻塞: %d %s", blockedIP.Code, blockedIP.Body.String())
	}

	sourcePath := "/api/manju/projects/project-secret/script-runs/" + runID + "/source"
	registered := manjuRequest(t, engine, http.MethodPost, sourcePath, map[string]any{
		"title": "雨夜旧宅", "authorizationScope": "仅本项目改编", "rangeLabel": "第 1 章",
		"content": "林澈推开旧宅的门。", "metadata": map[string]string{"sourceType": "novel"},
	}, "owner")
	if registered.Code != http.StatusOK {
		t.Fatalf("登记来源应 200，得到 %d: %s", registered.Code, registered.Body.String())
	}

	confirmPath := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/" + manjusvc.ScriptStageSourceRegistration + "/confirm"
	confirmed := manjuRequest(t, engine, http.MethodPost, confirmPath, nil, "owner")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("确认来源阶段应 200，得到 %d: %s", confirmed.Code, confirmed.Body.String())
	}

	read := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/script-runs/"+runID, nil, "owner")
	if read.Code != http.StatusOK {
		t.Fatalf("读取剧本任务应 200，得到 %d: %s", read.Code, read.Body.String())
	}
	var readPayload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &readPayload); err != nil {
		t.Fatal(err)
	}
	if readPayload.Data.Item.Source == nil || readPayload.Data.Item.Source.RangeLabel != "第 1 章" {
		t.Fatalf("读取任务必须返回来源账本: %+v", readPayload.Data.Item)
	}
	for _, stage := range readPayload.Data.Item.Stages {
		if stage.Key == "decision_anchor" {
			t.Fatalf("P0B 不得继续作为剧本阶段存在: %+v", stage)
		}
		if stage.Key == manjusvc.ScriptStageSourceRegistration && (stage.Status != manjusvc.ScriptStageStatusLocked || stage.ConfirmedBy != testManjuOwnerID) {
			t.Fatalf("来源阶段确认状态错误: %+v", stage)
		}
	}
	listed := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/script-runs", nil, "owner")
	if listed.Code != http.StatusOK {
		t.Fatalf("读取剧本任务列表应 200，得到 %d: %s", listed.Code, listed.Body.String())
	}
	var listPayload struct {
		Data struct {
			Items []manjusvc.ScriptRunView `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listPayload); err != nil {
		t.Fatal(err)
	}
	if len(listPayload.Data.Items) != 1 || listPayload.Data.Items[0].ID != runID {
		t.Fatalf("剧本任务列表必须能让页面刷新后恢复: %+v", listPayload.Data.Items)
	}
}

func prepareManjuRoundtableAwaitingDecision(t *testing.T, db *gorm.DB) string {
	t.Helper()
	return prepareManjuRoundtableAwaitingDecisionWithReview(t, db, "# 五席圆桌审核\ngrade: A\n等待用户选择。")
}

func prepareManjuRoundtableAwaitingDecisionWithReview(t *testing.T, db *gorm.DB, reviewContent string) string {
	t.Helper()
	now := time.Now()
	runID := "run-roundtable-http"
	run := &model.ManjuScriptRun{
		ID: runID, ProjectID: "project-secret", Branch: manjusvc.ScriptBranchOriginal,
		Mode: manjusvc.ScriptModeStepwise, Status: model.ManjuScriptRunStatusActive,
		CurrentStage: manjusvc.ScriptStageRoundtableReview, LockRevision: 1,
		CreatedBy: testManjuOwnerID, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(run).Error; err != nil {
		t.Fatal(err)
	}
	scriptArtifact := &model.ManjuScriptArtifact{
		ID: "http-script-v1", ProjectID: run.ProjectID, RunID: runID, StageID: "http-stage-script-draft",
		Kind: "SCRIPT_DRAFT.md", Version: 1, Content: "# 正式剧本 v1",
		ContentHash: "http-script-v1", CreatedBy: testManjuOwnerID, CreatedAt: now,
	}
	reviewArtifact := &model.ManjuScriptArtifact{
		ID: "http-roundtable-v1", ProjectID: run.ProjectID, RunID: runID, StageID: "http-stage-roundtable-review",
		Kind: "ROUNDTABLE_REVIEW.md", Version: 1, Content: reviewContent,
		ContentHash: "http-roundtable-v1", CreatedBy: testManjuOwnerID, CreatedAt: now,
	}
	if err := db.Create(&[]model.ManjuScriptArtifact{*scriptArtifact, *reviewArtifact}).Error; err != nil {
		t.Fatal(err)
	}
	stages := []model.ManjuScriptStage{
		{ID: "http-stage-source", RunID: runID, Key: manjusvc.ScriptStageSourceRegistration, Title: "来源登记", Position: 1, Status: manjusvc.ScriptStageStatusLocked, Version: 1},
		{ID: "http-stage-ip", RunID: runID, Key: manjusvc.ScriptStageIPDevelopment, Title: "IP 与角色开发", Position: 2, Status: manjusvc.ScriptStageStatusLocked, Version: 1},
		{ID: "http-stage-series", RunID: runID, Key: manjusvc.ScriptStageSeriesBible, Title: "全剧开发圣经", Position: 3, Status: manjusvc.ScriptStageStatusLocked, Version: 1},
		{ID: "http-stage-episodes", RunID: runID, Key: manjusvc.ScriptStageEpisodePlanning, Title: "分集规划", Position: 4, Status: manjusvc.ScriptStageStatusLocked, Version: 1},
		{ID: "http-stage-script-draft", RunID: runID, Key: manjusvc.ScriptStageScriptDraft, Title: "正式剧本", Position: 5, Status: manjusvc.ScriptStageStatusLocked, Version: 1, ArtifactID: scriptArtifact.ID},
		{ID: "http-stage-roundtable-review", RunID: runID, Key: manjusvc.ScriptStageRoundtableReview, Title: "五席圆桌审核", Position: 6, Status: manjusvc.ScriptStageStatusAwaitingDecision, Version: 1, ArtifactID: reviewArtifact.ID},
	}
	if err := db.Create(&stages).Error; err != nil {
		t.Fatal(err)
	}
	return runID
}

func TestManjuRoundtableDecisionRequiresExplicitChoice(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecision(t, db)
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/decide"

	recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{}, "owner")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing roundtable decision must map to 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var stages []model.ManjuScriptStage
	if err := db.Where("run_id = ?", runID).Find(&stages).Error; err != nil {
		t.Fatal(err)
	}
	for _, stage := range stages {
		if stage.Key == manjusvc.ScriptStageRoundtableReview && stage.Status != manjusvc.ScriptStageStatusAwaitingDecision {
			t.Fatalf("missing decision must not mutate review state: %+v", stage)
		}
	}
}

func TestManjuRoundtableDecisionRejectsNonAwaitingReviewAsConflict(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecision(t, db)
	if err := db.Model(&model.ManjuScriptStage{}).
		Where("run_id = ? AND key = ?", runID, manjusvc.ScriptStageRoundtableReview).
		Update("status", manjusvc.ScriptStageStatusLocked).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/decide"

	recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
		"decision": manjusvc.RoundtableProceed,
	}, "owner")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("non-awaiting roundtable decision must map to 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuRoundtableProceedRequiresRiskAcknowledgement(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecisionWithReview(
		t, db, "# 五席圆桌审核\ngrade: B\n等待用户选择。",
	)
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/decide"

	rejected := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
		"decision": manjusvc.RoundtableProceed,
	}, "owner")
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("B-grade proceed without risk acknowledgement must map to 400, got %d: %s", rejected.Code, rejected.Body.String())
	}

	accepted := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
		"decision":          manjusvc.RoundtableProceed,
		"acceptQualityRisk": true,
		"note":              "USER_ACCEPTED_QUALITY_RISK",
	}, "owner")
	if accepted.Code != http.StatusOK {
		t.Fatalf("B-grade proceed with risk acknowledgement must return 200, got %d: %s", accepted.Code, accepted.Body.String())
	}
	var payload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Item.RoundtableDecisions) != 1 {
		t.Fatalf("decision API must expose the immutable ledger row: %+v", payload.Data.Item.RoundtableDecisions)
	}
	record := payload.Data.Item.RoundtableDecisions[0]
	if record.Grade != "B" || !record.RiskAccepted || record.Decision != string(manjusvc.RoundtableProceed) {
		t.Fatalf("decision API ledger mismatch: %+v", record)
	}
	var review manjusvc.ScriptStageView
	for _, stage := range payload.Data.Item.Stages {
		if stage.Key == manjusvc.ScriptStageRoundtableReview {
			review = stage
			break
		}
	}
	if review.ReviewGrade != "B" {
		t.Fatalf("review grade must be exposed for the risk UI: %+v", review)
	}

	read := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/script-runs/"+runID, nil, "owner")
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"grade":"B"`) ||
		!strings.Contains(read.Body.String(), `"riskAccepted":true`) {
		t.Fatalf("run detail must expose the decision ledger after refresh: %d %s", read.Code, read.Body.String())
	}
}

func TestManjuRoundtableRegenerationRejectedWhileAwaitingDecision(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecision(t, db)
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/regenerate-artifact"

	recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
		"instruction": "try regenerating review",
	}, "owner")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("roundtable regeneration while awaiting decision must map to 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuRoundtableManualCorrectionRejectedWhileAwaitingDecision(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecision(t, db)
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/manual-correct"

	recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
		"content": "# 人工改写圆桌审核\ngrade: A",
		"note":    "attempt to bypass decision",
	}, "owner")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("manual roundtable correction while awaiting decision must map to 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuRoundtableGenericConfirmRejectsBypassState(t *testing.T) {
	engine, _, db := newManjuHTTPEnvWithDB(t)
	runID := prepareManjuRoundtableAwaitingDecision(t, db)
	if err := db.Model(&model.ManjuScriptStage{}).
		Where("run_id = ? AND key = ?", runID, manjusvc.ScriptStageRoundtableReview).
		Update("status", manjusvc.ScriptStageStatusAwaitingConfirm).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/confirm"

	recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{}, "owner")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("generic roundtable confirmation must map to 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuRoundtableDecisionRejectsNonAuthoritativeGrade(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{name: "missing", content: "# 五席圆桌审核\n等待用户选择。"},
		{name: "conflicting JSON and table", content: "# 五席圆桌审核\n{\"grade\":\"A\"}\n| 等级 | B |\n等待用户选择。"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			engine, _, db := newManjuHTTPEnvWithDB(t)
			runID := prepareManjuRoundtableAwaitingDecisionWithReview(t, db, testCase.content)
			path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/decide"

			recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
				"decision":          manjusvc.RoundtableProceed,
				"acceptQualityRisk": true,
			}, "owner")

			if recorder.Code != http.StatusConflict {
				t.Fatalf("non-authoritative grade must map to 409, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestManjuRoundtableDecisionReturnsDownstreamState(t *testing.T) {
	testCases := []struct {
		name            string
		decision        manjusvc.RoundtableDecision
		expectedRun     string
		expectedCurrent string
		expectedReview  string
		expectedScript  string
		expectedVersion int64
	}{
		{
			name:            "proceed current",
			decision:        manjusvc.RoundtableProceed,
			expectedRun:     model.ManjuScriptRunStatusCompleted,
			expectedReview:  manjusvc.ScriptStageStatusLocked,
			expectedScript:  manjusvc.ScriptStageStatusLocked,
			expectedVersion: 1,
		},
		{
			name:            "refine or rewrite",
			decision:        manjusvc.RoundtableRefine,
			expectedRun:     model.ManjuScriptRunStatusActive,
			expectedCurrent: manjusvc.ScriptStageScriptDraft,
			expectedReview:  manjusvc.ScriptStageStatusStale,
			expectedScript:  manjusvc.ScriptStageStatusInProgress,
			expectedVersion: 2,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			engine, _, db := newManjuHTTPEnvWithDB(t)
			runID := prepareManjuRoundtableAwaitingDecision(t, db)
			path := "/api/manju/projects/project-secret/script-runs/" + runID + "/stages/roundtable_review/decide"

			recorder := manjuRequest(t, engine, http.MethodPost, path, map[string]any{
				"decision": testCase.decision,
				"note":     "API decision regression",
			}, "owner")
			if recorder.Code != http.StatusOK {
				t.Fatalf("roundtable decision must return 200, got %d: %s", recorder.Code, recorder.Body.String())
			}
			var payload struct {
				Data struct {
					Item manjusvc.ScriptRunDetail `json:"item"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			detail := payload.Data.Item
			if detail.Run.Status != testCase.expectedRun || detail.Run.CurrentStage != testCase.expectedCurrent {
				t.Fatalf("unexpected run state: %+v", detail.Run)
			}
			var review, scriptDraft manjusvc.ScriptStageView
			for _, stage := range detail.Stages {
				switch stage.Key {
				case manjusvc.ScriptStageRoundtableReview:
					review = stage
				case manjusvc.ScriptStageScriptDraft:
					scriptDraft = stage
				}
			}
			if review.Status != testCase.expectedReview {
				t.Fatalf("unexpected review state: %+v", review)
			}
			if review.ReviewGrade != "A" {
				t.Fatalf("unexpected review grade: %+v", review)
			}
			if scriptDraft.Status != testCase.expectedScript || scriptDraft.Version != testCase.expectedVersion {
				t.Fatalf("unexpected script_draft state: %+v", scriptDraft)
			}
			if len(detail.RoundtableDecisions) != 1 {
				t.Fatalf("decision must be present in run detail: %+v", detail.RoundtableDecisions)
			}
			record := detail.RoundtableDecisions[0]
			if record.Decision != string(testCase.decision) ||
				record.Grade != "A" ||
				record.RiskAccepted ||
				record.Note != "API decision regression" {
				t.Fatalf("unexpected decision ledger row: %+v", record)
			}
		})
	}
}

func TestManjuSourceLibraryCreatesChapterAdaptationUnit(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	lockBody := map[string]any{
		"track": "domestic", "source": "adaptation", "videoModel": "seedance-2.0",
		"aspectRatio": "9:16", "deliveryMode": "both", "targetDurationSeconds": 15,
		"styleBase": "live-action", "styleCardId": "sd-style-009", "assetMode": "M1",
		"confirmedLocks": confirmedManjuLockKeys(false),
	}
	if saved := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/lock", lockBody, "owner"); saved.Code != http.StatusOK {
		t.Fatalf("保存 P0 失败: %d %s", saved.Code, saved.Body.String())
	}
	documentHTTP := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/source-documents", map[string]any{
		"title": "雨夜旧宅", "authorizationScope": "本项目改编", "fileName": "雨夜旧宅.txt",
		"content": "第一章 雨夜\n林澈推门。\n\n第二章 旧照片\n照片背面有日期。",
	}, "owner")
	if documentHTTP.Code != http.StatusOK {
		t.Fatalf("上传原著失败: %d %s", documentHTTP.Code, documentHTTP.Body.String())
	}
	var documentPayload struct {
		Data struct {
			Item manjusvc.SourceDocumentDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(documentHTTP.Body.Bytes(), &documentPayload); err != nil {
		t.Fatal(err)
	}
	document := documentPayload.Data.Item
	if document.Document.ChapterCount != 2 || len(document.Chapters) != 2 {
		t.Fatalf("原著必须拆成两章: %+v", document)
	}
	stateHTTP := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/source-library-state", nil, "owner")
	if stateHTTP.Code != http.StatusOK || !strings.Contains(stateHTTP.Body.String(), `"chapterCount":2`) {
		t.Fatalf("原著库状态必须返回章节哈希与数量: %d %s", stateHTTP.Code, stateHTTP.Body.String())
	}
	runHTTP := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/script-runs", map[string]any{
		"mode": "STEPWISE", "title": "第一章改编",
		"sourceDocumentId": document.Document.ID, "chapterIds": []string{document.Chapters[0].ID},
	}, "owner")
	if runHTTP.Code != http.StatusOK {
		t.Fatalf("创建章节改编单元失败: %d %s", runHTTP.Code, runHTTP.Body.String())
	}
	var runPayload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(runHTTP.Body.Bytes(), &runPayload); err != nil {
		t.Fatal(err)
	}
	run := runPayload.Data.Item
	if run.Run.UnitTitle != "第一章改编" || run.Source == nil || !strings.Contains(run.Source.RangeLabel, "第一章 雨夜") {
		t.Fatalf("章节改编单元没有保存来源范围: %+v", run)
	}
}

func TestManjuAssetRoutesRequireCompletedScript(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	lockBody := map[string]any{
		"track": "domestic", "source": "adaptation", "videoModel": "seedance-2.0",
		"aspectRatio": "9:16", "deliveryMode": "both", "targetDurationSeconds": 15,
		"styleBase": "live-action", "styleCardId": "sd-style-009", "assetMode": "M1",
		"confirmedLocks": confirmedManjuLockKeys(false),
	}
	if saved := manjuRequest(t, engine, http.MethodPut, "/api/manju/projects/project-secret/lock", lockBody, "owner"); saved.Code != http.StatusOK {
		t.Fatalf("准备 P0 失败: %d %s", saved.Code, saved.Body.String())
	}
	created := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/script-runs", map[string]any{"mode": manjusvc.ScriptModeStepwise}, "owner")
	if created.Code != http.StatusOK {
		t.Fatalf("创建 P1 失败: %d %s", created.Code, created.Body.String())
	}
	var payload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}

	blocked := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/asset-runs", map[string]any{
		"scriptRunId": payload.Data.Item.Run.ID,
	}, "owner")
	if blocked.Code != http.StatusConflict {
		t.Fatalf("未完成 P1 创建 P2 必须冲突，得到 %d: %s", blocked.Code, blocked.Body.String())
	}
	listed := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/asset-runs", nil, "owner")
	if listed.Code != http.StatusOK {
		t.Fatalf("P2 列表路由必须可用，得到 %d: %s", listed.Code, listed.Body.String())
	}
}

func TestManjuRetiredP0BDoesNotGateScriptStageRun(t *testing.T) {
	engine, service := newManjuHTTPEnv(t)
	lockBody := map[string]any{
		"track": "domestic", "source": "adaptation", "videoModel": "seedance-2.0",
		"aspectRatio": "9:16", "deliveryMode": "both", "targetDurationSeconds": 15,
		"styleBase": "live-action", "styleCardId": "sd-style-009",
		"assetMode":      "M1",
		"confirmedLocks": confirmedManjuLockKeys(false),
	}
	for _, request := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPut, "/api/manju/projects/project-secret/lock", lockBody},
		{http.MethodPut, "/api/manju/projects/project-secret/models", validManjuModelsBody()},
	} {
		if recorder := manjuRequest(t, engine, request.method, request.path, request.body, "owner"); recorder.Code != http.StatusOK {
			t.Fatalf("准备 P0/P1 失败: %d %s", recorder.Code, recorder.Body.String())
		}
	}

	created := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/script-runs", map[string]any{
		"mode": manjusvc.ScriptModeStepwise,
	}, "owner")
	var createPayload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createPayload); err != nil || createPayload.Data.Item.Run.ID == "" {
		t.Fatalf("创建剧本任务失败: %v %s", err, created.Body.String())
	}
	runID := createPayload.Data.Item.Run.ID
	base := "/api/manju/projects/project-secret/script-runs/" + runID
	if recorder := manjuRequest(t, engine, http.MethodPost, base+"/source", map[string]any{
		"title": "雨夜旧宅", "authorizationScope": "仅本项目改编", "rangeLabel": "第 1 章", "content": "林澈推开旧宅的门。",
	}, "owner"); recorder.Code != http.StatusOK {
		t.Fatalf("登记来源失败: %d %s", recorder.Code, recorder.Body.String())
	}
	if recorder := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ScriptStageSourceRegistration+"/confirm", nil, "owner"); recorder.Code != http.StatusOK {
		t.Fatalf("确认来源失败: %d %s", recorder.Code, recorder.Body.String())
	}

	var captured localtask.CreateRequest
	service.SetScriptTaskCreator(func(userID string, request localtask.CreateRequest) (*model.Task, error) {
		captured = request
		return &model.Task{ID: "task-http-script", UserID: userID, ProjectID: request.ProjectID, Type: request.Type, Status: model.TaskStatusQueued}, nil
	})
	run := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ScriptStageIPDevelopment+"/run", map[string]any{
		"instruction": "突出雨夜压迫感",
	}, "owner")
	if run.Code != http.StatusOK {
		t.Fatalf("启动阶段应 200，得到 %d: %s", run.Code, run.Body.String())
	}
	if captured.Type != "canvas_text" || captured.ProjectID != "project-secret" || captured.Input["mode"] != "text" {
		t.Fatalf("阶段必须提交 BeefTV 文本任务: %+v", captured)
	}
	var payload struct {
		Data struct {
			Item manjusvc.ScriptRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(run.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, stage := range payload.Data.Item.Stages {
		if stage.Key == manjusvc.ScriptStageIPDevelopment && stage.TaskID != "task-http-script" {
			t.Fatalf("阶段未绑定返回的任务: %+v", stage)
		}
	}
}

func TestManjuProductionP0BDecisionLocksBeforeStoryboard(t *testing.T) {
	engine, service, db := newManjuHTTPEnvWithDB(t)
	seedProductionReadyProject(t, db)

	created := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs", map[string]any{
		"assetRunId": "asset-production",
	}, "owner")
	if created.Code != http.StatusOK {
		t.Fatalf("创建生产链失败: %d %s", created.Code, created.Body.String())
	}
	var createdPayload struct {
		Data struct {
			Item manjusvc.ProductionRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdPayload); err != nil {
		t.Fatal(err)
	}
	run := createdPayload.Data.Item
	if len(run.Stages) != 4 || run.Stages[0].Key != manjusvc.ProductionStageDecisionAnchor {
		t.Fatalf("生产链必须以 P0B 为首阶段: %+v", run.Stages)
	}
	base := "/api/manju/projects/project-secret/production-runs/" + run.Run.ID
	blocked := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageStoryboard+"/run", map[string]any{}, "owner")
	if blocked.Code != http.StatusConflict {
		t.Fatalf("P0B 未锁定时 P3 必须冲突: %d %s", blocked.Code, blocked.Body.String())
	}

	task := &model.Task{
		ID: "task-http-production-p0b", UserID: testManjuOwnerID, ProjectID: "project-secret",
		Type: "canvas_text", Status: model.TaskStatusQueued,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	service.SetScriptTaskCreator(func(userID string, request localtask.CreateRequest) (*model.Task, error) {
		return task, nil
	})
	started := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageDecisionAnchor+"/run", map[string]any{}, "owner")
	if started.Code != http.StatusOK {
		t.Fatalf("启动生产 P0B 失败: %d %s", started.Code, started.Body.String())
	}
	result, err := json.Marshal(map[string]any{"mode": "text", "text": validProductionP0BArtifact()})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成", "result_json": string(result),
	}).Error; err != nil {
		t.Fatal(err)
	}
	detail := manjuRequest(t, engine, http.MethodGet, base, nil, "owner")
	if detail.Code != http.StatusOK {
		t.Fatalf("读取生产链失败: %d %s", detail.Code, detail.Body.String())
	}
	confirmed := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageDecisionAnchor+"/confirm", map[string]any{}, "owner")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("确认生产 P0B 失败: %d %s", confirmed.Code, confirmed.Body.String())
	}
	var confirmedPayload struct {
		Data struct {
			Item manjusvc.ProductionRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(confirmed.Body.Bytes(), &confirmedPayload); err != nil {
		t.Fatal(err)
	}
	for _, stage := range confirmedPayload.Data.Item.Stages {
		if stage.Key == manjusvc.ProductionStageDecisionAnchor && stage.Status != model.ManjuProductionStageStatusLocked {
			t.Fatalf("P0B 必须锁定: %+v", stage)
		}
		if stage.Key == manjusvc.ProductionStageStoryboard && stage.Status != model.ManjuProductionStageStatusInProgress {
			t.Fatalf("P0B 锁定后必须开放 P3: %+v", stage)
		}
	}
}

func validProductionP0BArtifact() string {
	return `# P0B 决策锚点
## 情绪锚点表

【大分镜 #B01 · 情绪锚点表】

| 节拍 # | 情绪关键词 | 服务的剧本动作 | 情境 ID | 核心组合摘要（80%定死） | 解锁雷区 |
|---|---|---|---|---|---|
| 节拍 1 | 压抑试探 | 母亲把体检单压在茶杯下 | E05 | 长焦+大光圈+空旷留白 | — |
| 节拍 2 | 察觉异样 | 女儿瞥见纸角，手指顿住 | E11 | 微距标点+静切动 | — |
| 节拍 3 | 强装平静 | 母亲端起杯子继续夹菜 | E08 | 过肩+长焦+手持微动 | — |

## 构图雷区白名单

- A1 正面居中：A类；解锁；白名单理由为仪式感；配套强对称构图、静态运镜与长焦压缩。
- A类：其余项目无解锁；未命中白名单时按默认规避。
- B类 · 永不解锁：物体穿身、角色服装突变、杂乱前景刺眼物、比例失调、重复元素无重点、空鸡蛋构图、道具状态突变。
`
}

func seedProductionReadyProject(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	receipt := model.ManjuLockReceipt{
		ID: "lock-production", ProjectID: "project-secret", Revision: 1, Status: model.ManjuLockStatusLocked,
		PayloadJSON: `{"videoModel":"seedance-2.0","aspectRatio":"9:16","assetMode":"M1"}`, ContentHash: "lock-hash",
		CreatedBy: testManjuOwnerID, CreatedAt: now,
	}
	binding := model.ManjuProjectModelBinding{
		ID: "binding-production", ProjectID: "project-secret",
		TextChannelID: "channel-main", TextModelKey: "text-pro",
		ImageChannelID: "channel-main", ImageModelKey: "image-pro",
		VideoChannelID: "channel-main", VideoModelKey: "video-pro",
		UpdatedBy: testManjuOwnerID, CreatedAt: now, UpdatedAt: now,
	}
	scriptRun := model.ManjuScriptRun{
		ID: "script-production", ProjectID: "project-secret", Branch: model.ManjuScriptBranchAdaptation,
		Mode: model.ManjuScriptModeStepwise, Status: model.ManjuScriptRunStatusCompleted,
		LockRevision: 1, CreatedBy: testManjuOwnerID, CreatedAt: now, UpdatedAt: now,
	}
	assetRun := model.ManjuAssetRun{
		ID: "asset-production", ProjectID: "project-secret", ScriptRunID: scriptRun.ID,
		AssetMode: model.ManjuAssetModeM1, Status: model.ManjuAssetRunStatusCompleted,
		LockRevision: 1, CreatedBy: testManjuOwnerID, CreatedAt: now, UpdatedAt: now,
	}
	assetArtifact := model.ManjuAssetArtifact{
		ID: "asset-artifact-compile", ProjectID: "project-secret", RunID: assetRun.ID, StageID: "asset-stage-compile",
		Kind: model.ManjuAssetArtifactM1, Version: 1, Content: "# P2 M1 资产交付\n图1 林澈", ContentHash: "asset-hash",
		CreatedBy: testManjuOwnerID, CreatedAt: now,
	}
	compileStage := model.ManjuAssetStage{
		ID: "asset-stage-compile", RunID: assetRun.ID, Key: model.ManjuAssetStageCompile,
		Title: "P2 资产编译", Position: 3, Status: model.ManjuAssetStageStatusLocked,
		Version: 1, ArtifactID: assetArtifact.ID, ConfirmedBy: testManjuOwnerID,
		ConfirmedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	for _, item := range []any{&receipt, &binding, &scriptRun, &assetRun, &assetArtifact, &compileStage} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func lockProductionP0BStageThroughAPI(t *testing.T, engine *gin.Engine, db *gorm.DB, baseURL, runID string) {
	t.Helper()
	var stage model.ManjuProductionStage
	if err := db.First(&stage, "run_id = ? AND key = ?", runID, manjusvc.ProductionStageDecisionAnchor).Error; err != nil {
		t.Fatal(err)
	}
	content := validProductionP0BArtifact()
	artifact := &model.ManjuProductionArtifact{
		ID: "p0b-http-" + runID[:8], ProjectID: "project-secret", RunID: runID, StageID: stage.ID,
		Kind: model.ManjuProductionArtifactEmotionAnchor, Version: 1, Content: content,
		ContentHash: "locked-p0b-hash", CreatedBy: testManjuOwnerID, CreatedAt: time.Now(),
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := db.Model(&model.ManjuProductionStage{}).Where("id = ?", stage.ID).Updates(map[string]any{
		"status": model.ManjuProductionStageStatusAwaitingConfirmation, "artifact_id": artifact.ID,
		"version": 1, "completed_at": now, "error": "",
	}).Error; err != nil {
		t.Fatal(err)
	}
	confirmed := manjuRequest(
		t, engine, http.MethodPost,
		baseURL+"/stages/"+manjusvc.ProductionStageDecisionAnchor+"/confirm",
		map[string]any{}, "owner",
	)
	if confirmed.Code != http.StatusOK {
		t.Fatalf("确认生产 P0B 失败: %d %s", confirmed.Code, confirmed.Body.String())
	}
}

func TestManjuProductionRoutesRequireSession(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs", map[string]any{"assetRunId": "asset-production"}, "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录创建生产链应 401，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	images := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/storyboard-images", nil, "")
	if images.Code != http.StatusUnauthorized {
		t.Fatalf("未登录启动故事板图应 401，得到 %d: %s", images.Code, images.Body.String())
	}
	videos := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/videos", nil, "")
	if videos.Code != http.StatusUnauthorized {
		t.Fatalf("未登录启动 P4 视频应 401，得到 %d: %s", videos.Code, videos.Body.String())
	}
	previs := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/previs-storyboard", nil, "")
	if previs.Code != http.StatusUnauthorized {
		t.Fatalf("未登录生成 P4.5 应 401，得到 %d: %s", previs.Code, previs.Body.String())
	}
}

func TestP4RequestRejectsObsoleteSkipValidationField(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(
		t,
		engine,
		http.MethodPost,
		"/api/manju/projects/project-secret/production-runs/run-1/stages/feed_package/run",
		map[string]any{"skipValidation": true},
		"owner",
	)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("skipValidation must be rejected, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestP4RequestRejectsTrailingJSONValue(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/manju/projects/project-secret/production-runs/run-1/stages/feed_package/run",
		strings.NewReader(`{"instruction":""}{"skipValidation":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Test-Manju-User", "owner")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON must be rejected, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestManjuProductionRoutesDoNotLeakForeignProject(t *testing.T) {
	engine, _ := newManjuHTTPEnv(t)

	recorder := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs", map[string]any{"assetRunId": "asset-production"}, "outsider")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("外部用户创建生产链必须 404，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "project-secret") || strings.Contains(recorder.Body.String(), "asset-production") {
		t.Fatalf("外部项目响应泄露了数据: %s", recorder.Body.String())
	}
	images := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/storyboard-images", nil, "outsider")
	if images.Code != http.StatusNotFound {
		t.Fatalf("外部用户启动故事板图必须 404，得到 %d: %s", images.Code, images.Body.String())
	}
	if strings.Contains(images.Body.String(), "project-secret") || strings.Contains(images.Body.String(), "run-secret") {
		t.Fatalf("外部故事板图响应泄露了数据: %s", images.Body.String())
	}
	videos := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/videos", nil, "outsider")
	if videos.Code != http.StatusNotFound {
		t.Fatalf("外部用户启动 P4 视频必须 404，得到 %d: %s", videos.Code, videos.Body.String())
	}
	if strings.Contains(videos.Body.String(), "project-secret") || strings.Contains(videos.Body.String(), "run-secret") {
		t.Fatalf("外部 P4 视频响应泄露了数据: %s", videos.Body.String())
	}
	previs := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs/run-secret/previs-storyboard", nil, "outsider")
	if previs.Code != http.StatusNotFound {
		t.Fatalf("外部用户生成 P4.5 必须 404，得到 %d: %s", previs.Code, previs.Body.String())
	}
}

func TestManjuProductionRoutesCreateRunAndAdvanceStoryboardStage(t *testing.T) {
	engine, service, db := newManjuHTTPEnvWithDB(t)
	seedProductionReadyProject(t, db)

	created := manjuRequest(t, engine, http.MethodPost, "/api/manju/projects/project-secret/production-runs", map[string]any{
		"assetRunId": "asset-production",
	}, "owner")
	if created.Code != http.StatusOK {
		t.Fatalf("创建生产链应 200，得到 %d: %s", created.Code, created.Body.String())
	}
	var createPayload struct {
		Data struct {
			Item manjusvc.ProductionRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createPayload); err != nil {
		t.Fatal(err)
	}
	run := createPayload.Data.Item.Run
	if run.ID == "" || run.AssetRunID != "asset-production" || run.CurrentStage != manjusvc.ProductionStageDecisionAnchor {
		t.Fatalf("生产链创建回执错误: %+v", run)
	}

	listed := manjuRequest(t, engine, http.MethodGet, "/api/manju/projects/project-secret/production-runs", nil, "owner")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), run.ID) {
		t.Fatalf("生产链列表必须返回刚创建的任务: %d %s", listed.Code, listed.Body.String())
	}
	base := "/api/manju/projects/project-secret/production-runs/" + run.ID
	lockProductionP0BStageThroughAPI(t, engine, db, base, run.ID)
	detail := manjuRequest(t, engine, http.MethodGet, base, nil, "owner")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), manjusvc.ProductionStageStoryboard) {
		t.Fatalf("生产链详情必须返回 P3 阶段: %d %s", detail.Code, detail.Body.String())
	}

	var captured []localtask.CreateRequest
	service.SetScriptTaskCreator(func(userID string, request localtask.CreateRequest) (*model.Task, error) {
		captured = append(captured, request)
		task := &model.Task{
			ID: fmt.Sprintf("task-production-%d", len(captured)), UserID: userID,
			ProjectID: request.ProjectID, Type: request.Type, Status: model.TaskStatusQueued,
		}
		if err := db.Create(task).Error; err != nil {
			return nil, err
		}
		return task, nil
	})
	ran := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageStoryboard+"/run", map[string]any{
		"instruction": "第 1 段多给动作相位",
	}, "owner")
	if ran.Code != http.StatusOK {
		t.Fatalf("执行 P3 应 200，得到 %d: %s", ran.Code, ran.Body.String())
	}
	if len(captured) != 1 || captured[0].Type != "canvas_text" || captured[0].Operation != "manju_production_stage" || captured[0].ProjectID != "project-secret" {
		t.Fatalf("P3 必须创建 BeefTV 文本任务: %+v", captured)
	}
	if !strings.Contains(captured[0].Prompt, "P2 已锁定且可交接的工件") || !strings.Contains(captured[0].Prompt, "第 1 段多给动作相位") {
		t.Fatalf("P3 提示词没有读取锁定 P2 和用户补充: %s", captured[0].Prompt)
	}

	var stage model.ManjuProductionStage
	if err := db.First(&stage, "run_id = ? AND key = ?", run.ID, manjusvc.ProductionStageStoryboard).Error; err != nil {
		t.Fatal(err)
	}
	storyboard := strings.Join([]string{
		"# P3 分镜与故事板",
		"## 分镜总表",
		"| 镜号 | 时长 | 景别 | 动作 |",
		"|---|---|---|---|",
		"| 1-1 | 3 | 近景 | 林澈停步 |",
		"## 故事板表",
		"| 段 | 格 | 动作相位 | 站位视线 | 资产引用 |",
		"|---|---|---|---|---|",
		"| 1 | 1 | 推门 | 画面左侧 | 图1 |",
		"## P3 交付清单",
		"| 段 | 剧情分镜格/覆盖 | 故事板图格数 | 具体说明/豁免理由 |",
		"|---|---|---|---|",
		"| 段1 | 场1 ✅ | 1格 | 测试样本 |",
		"故事板图：每段六格",
		"剧情分镜格：每场一格",
		"## 台词精简建议清单",
		"默认承接上游台词，无精简。",
	}, "\n")
	artifact := model.ManjuProductionArtifact{
		ID: "production-artifact-storyboard", ProjectID: "project-secret", RunID: run.ID, StageID: stage.ID,
		Kind: model.ManjuProductionArtifactStoryboard, Version: 1, Content: storyboard, ContentHash: "storyboard-hash",
		CreatedBy: testManjuOwnerID, CreatedAt: time.Now(),
	}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stage.Status = model.ManjuProductionStageStatusAwaitingConfirmation
	stage.ArtifactID = artifact.ID
	stage.CompletedAt = &now
	if err := db.Save(&stage).Error; err != nil {
		t.Fatal(err)
	}

	imagesStarted := manjuRequest(t, engine, http.MethodPost, base+"/storyboard-images", nil, "owner")
	if imagesStarted.Code != http.StatusOK {
		t.Fatalf("启动 P3 故事板图片任务应 200，得到 %d: %s", imagesStarted.Code, imagesStarted.Body.String())
	}
	var imagePayload struct {
		Data struct {
			Item manjusvc.ProductionRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(imagesStarted.Body.Bytes(), &imagePayload); err != nil {
		t.Fatal(err)
	}
	if len(imagePayload.Data.Item.Images) != 1 || imagePayload.Data.Item.Images[0].TaskID == "" {
		t.Fatalf("必须按故事板段创建真实图片任务: %+v", imagePayload.Data.Item.Images)
	}
	if len(captured) != 2 || captured[1].Type != "canvas_image" || captured[1].Operation != "manju_storyboard_image" {
		t.Fatalf("P3 故事板图必须使用 canvas_image 任务: %+v", captured)
	}
	imageTaskID := imagePayload.Data.Item.Images[0].TaskID
	imageResourceID := "res-http-storyboard-1"
	if err := db.Model(&model.Task{}).Where("id = ?", imageTaskID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Resource{
		ID: imageResourceID, UserID: testManjuOwnerID, Kind: "image", Status: model.ResourceStatusReady,
		MimeType: "image/png", PublicURL: "/api/resources/" + imageResourceID + "/file", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	payload, err := localtask.EncodeOutputPayload(localtask.CanonicalOutput{
		OutputIndex: 0, MediaType: "image", ResourceID: imageResourceID,
		MaterializedAssetID: localtask.MaterializedAssetID(imageTaskID, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Result{
		ID: localtask.OutputResultID(imageTaskID, 0), UserID: testManjuOwnerID, TaskID: imageTaskID,
		Kind: localtask.ResultKindGenerationOutput, Payload: payload, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	imageDetail := manjuRequest(t, engine, http.MethodGet, base, nil, "owner")
	if imageDetail.Code != http.StatusOK || !strings.Contains(imageDetail.Body.String(), imageResourceID) {
		t.Fatalf("故事板图对账后必须返回真实 resourceId: %d %s", imageDetail.Code, imageDetail.Body.String())
	}

	confirmed := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageStoryboard+"/confirm", map[string]any{
		"note": "确认 P3 交付",
	}, "owner")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("确认 P3 应 200，得到 %d: %s", confirmed.Code, confirmed.Body.String())
	}
	if !strings.Contains(confirmed.Body.String(), manjusvc.ProductionStageFeed) {
		t.Fatalf("确认 P3 后必须开放 P4: %s", confirmed.Body.String())
	}

	var feedStage model.ManjuProductionStage
	if err := db.First(&feedStage, "run_id = ? AND key = ?", run.ID, manjusvc.ProductionStageFeed).Error; err != nil {
		t.Fatal(err)
	}
	feedContent := strings.Join([]string{
		"> 素材形态：M1（编号占位 · 图号见接线单）",
		"# P4 投喂封装",
		"## 段1",
		"【画面主体】图1 林澈推门，中景侧45度。",
		"【声音】木门吱呀。",
		"【台词】无",
		"【时长】5秒",
		"【画幅】9:16",
		"## 素材接线单",
		"| 图号 | 类型 | 用途 |",
		"| --- | --- | --- |",
		"| 图1 | 场景 | 旧宅门内 |",
		"## 台词完整性",
		"无台词。",
		"## 六道闸门",
		"全部通过。",
	}, "\n")
	feedArtifact := model.ManjuProductionArtifact{
		ID: "production-artifact-feed", ProjectID: "project-secret", RunID: run.ID, StageID: feedStage.ID,
		Kind: model.ManjuProductionArtifactFeed, Version: 1, Content: feedContent, ContentHash: "feed-hash",
		CreatedBy: testManjuOwnerID, CreatedAt: time.Now(),
	}
	if err := db.Create(&feedArtifact).Error; err != nil {
		t.Fatal(err)
	}
	feedStage.Status = model.ManjuProductionStageStatusAwaitingConfirmation
	feedStage.ArtifactID = feedArtifact.ID
	feedStage.CompletedAt = &now
	if err := db.Save(&feedStage).Error; err != nil {
		t.Fatal(err)
	}
	previsArtifact := model.ManjuProductionArtifact{
		ID: "production-artifact-previs", ProjectID: "project-secret", RunID: run.ID, StageID: feedStage.ID,
		Kind: model.ManjuProductionArtifactPrevis, Version: 1,
		Content: "# 项目 · 预演故事板\n\n## 段 1\n**本段要验什么**：站位\n", ContentHash: "previs-hash",
		CreatedBy: testManjuOwnerID, CreatedAt: time.Now(),
	}
	if err := db.Create(&previsArtifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ManjuProductionPrevis{
		ID: "production-previs-row", ProjectID: "project-secret", RunID: run.ID, StageID: feedStage.ID,
		ArtifactID: previsArtifact.ID, Status: model.ManjuProductionPrevisStatusReady, Version: 1,
		CreatedBy: testManjuOwnerID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	videosStarted := manjuRequest(t, engine, http.MethodPost, base+"/videos", nil, "owner")
	if videosStarted.Code != http.StatusOK {
		t.Fatalf("启动 P4 视频任务应 200，得到 %d: %s", videosStarted.Code, videosStarted.Body.String())
	}
	var videoPayload struct {
		Data struct {
			Item manjusvc.ProductionRunDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(videosStarted.Body.Bytes(), &videoPayload); err != nil {
		t.Fatal(err)
	}
	if len(videoPayload.Data.Item.Videos) != 1 || videoPayload.Data.Item.Videos[0].TaskID == "" {
		t.Fatalf("必须按 P4 投喂段创建真实视频任务: %+v", videoPayload.Data.Item.Videos)
	}
	if len(captured) != 3 || captured[2].Type != "canvas_video" || captured[2].Operation != "manju_production_video" {
		t.Fatalf("P4 视频必须使用 canvas_video 任务: %+v", captured)
	}
	videoTaskID := videoPayload.Data.Item.Videos[0].TaskID
	videoResourceID := "res-http-video-1"
	if err := db.Model(&model.Task{}).Where("id = ?", videoTaskID).Updates(map[string]any{
		"status": model.TaskStatusSucceeded, "stage": "完成",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Resource{
		ID: videoResourceID, UserID: testManjuOwnerID, Kind: "video", Status: model.ResourceStatusReady,
		MimeType: "video/mp4", PublicURL: "/api/resources/" + videoResourceID + "/file", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	videoOutput, err := localtask.EncodeOutputPayload(localtask.CanonicalOutput{
		OutputIndex: 0, MediaType: "video", ResourceID: videoResourceID,
		MaterializedAssetID: localtask.MaterializedAssetID(videoTaskID, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Result{
		ID: localtask.OutputResultID(videoTaskID, 0), UserID: testManjuOwnerID, TaskID: videoTaskID,
		Kind: localtask.ResultKindGenerationOutput, Payload: videoOutput, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	videoDetail := manjuRequest(t, engine, http.MethodGet, base, nil, "owner")
	if videoDetail.Code != http.StatusOK || !strings.Contains(videoDetail.Body.String(), videoResourceID) {
		t.Fatalf("P4 视频对账后必须返回真实 resourceId: %d %s", videoDetail.Code, videoDetail.Body.String())
	}
	confirmedFeed := manjuRequest(t, engine, http.MethodPost, base+"/stages/"+manjusvc.ProductionStageFeed+"/confirm", map[string]any{
		"note": "确认 P4 真实视频",
	}, "owner")
	if confirmedFeed.Code != http.StatusOK || !strings.Contains(confirmedFeed.Body.String(), manjusvc.ProductionStageQC) {
		t.Fatalf("真实视频就绪后确认 P4 应开放 P5: %d %s", confirmedFeed.Code, confirmedFeed.Body.String())
	}
}
