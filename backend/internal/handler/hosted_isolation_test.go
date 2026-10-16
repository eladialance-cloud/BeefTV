package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/database"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// hostedIsolationEngine builds a real router over the hosted schema, with a test
// middleware standing in for the hosted session middleware: it marks the request
// as hosted and publishes whatever user the X-Test-User header names. This
// exercises the actual route handlers, not a stub of them.
func hostedIsolationEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "hosted.db")+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := database.MigrateHostedSchema(db); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"user-alice", "user-bob"} {
		row := model.User{ID: id, Username: id, DisplayName: id, Role: model.UserRoleUser, Status: model.UserStatusActive}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := app.NewLocal(repository.New(db), t.TempDir())

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		MarkHostedRequest(c)
		if id := strings.TrimSpace(c.GetHeader("X-Test-User")); id != "" {
			SetSessionUser(c, &model.User{ID: id, Username: id, Role: model.UserRoleUser, Status: model.UserStatusActive})
		}
		c.Next()
	})
	RegisterCanvasAPI(engine.Group("/api"), svc)
	return engine
}

func isolationCall(t *testing.T, engine *gin.Engine, method string, path string, actor string, payload any) (int, string) {
	t.Helper()
	body := ""
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = string(encoded)
	}
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		request.Header.Set("X-Test-User", actor)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func canvasPayload(id string, title string) map[string]any {
	return map[string]any{"project": map[string]any{
		"id": id, "revision": 0, "title": title, "nodes": []any{}, "connections": []any{},
	}}
}

func TestHostedRoutesRejectRequestsWithoutSession(t *testing.T) {
	engine := hostedIsolationEngine(t)
	code, _ := isolationCall(t, engine, http.MethodGet, "/api/canvas-projects/canvas-alice", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("无会话访问画布应返回 401，得到 %d", code)
	}
}

func TestHostedUsersCannotSeeEachOthersCanvas(t *testing.T) {
	engine := hostedIsolationEngine(t)

	code, body := isolationCall(t, engine, http.MethodPut, "/api/canvas-projects/canvas-alice", "user-alice", canvasPayload("canvas-alice", "Alice 的画布"))
	if code != http.StatusOK {
		t.Fatalf("Alice 创建画布应成功，得到 %d %s", code, body)
	}
	if code, body = isolationCall(t, engine, http.MethodGet, "/api/canvas-projects/canvas-alice", "user-alice", nil); code != http.StatusOK {
		t.Fatalf("Alice 读取自己的画布应成功，得到 %d %s", code, body)
	}

	// Bob 既不该读到内容，也不该因为 403 而知道它存在——必须是 404。
	code, body = isolationCall(t, engine, http.MethodGet, "/api/canvas-projects/canvas-alice", "user-bob", nil)
	if code != http.StatusNotFound {
		t.Fatalf("Bob 读取 Alice 的画布必须返回 404，得到 %d %s", code, body)
	}
	if strings.Contains(body, "Alice 的画布") {
		t.Fatalf("拒绝响应不得泄露他人画布内容: %s", body)
	}

	// Bob 的画布列表里不能出现 Alice 的条目。
	code, body = isolationCall(t, engine, http.MethodGet, "/api/canvas-projects", "user-bob", nil)
	if code != http.StatusOK {
		t.Fatalf("Bob 列出自己的画布应成功，得到 %d %s", code, body)
	}
	if strings.Contains(body, "canvas-alice") {
		t.Fatalf("Bob 的画布列表泄露了 Alice 的画布: %s", body)
	}
}

func TestHostedUserCannotOverwriteAnotherUsersCanvas(t *testing.T) {
	engine := hostedIsolationEngine(t)

	code, body := isolationCall(t, engine, http.MethodPut, "/api/canvas-projects/canvas-alice", "user-alice", canvasPayload("canvas-alice", "Alice 的画布"))
	if code != http.StatusOK {
		t.Fatalf("Alice 创建画布应成功，得到 %d %s", code, body)
	}

	// Bob 用同一个画布 ID 覆盖写入：必须是拒绝，且 Alice 的内容原样保留。
	code, _ = isolationCall(t, engine, http.MethodPut, "/api/canvas-projects/canvas-alice", "user-bob", canvasPayload("canvas-alice", "Bob 篡改"))
	if code == http.StatusOK {
		t.Fatalf("Bob 不得覆盖 Alice 的画布，却返回了 200")
	}
	_, body = isolationCall(t, engine, http.MethodGet, "/api/canvas-projects/canvas-alice", "user-alice", nil)
	if strings.Contains(body, "Bob 篡改") {
		t.Fatalf("Alice 的画布被 Bob 篡改: %s", body)
	}
}
