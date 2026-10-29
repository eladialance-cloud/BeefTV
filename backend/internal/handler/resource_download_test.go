package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
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
	"gorm.io/gorm/logger"
)

func TestResourceDownloadDispositionSanitizesFileName(t *testing.T) {
	resource := &model.Resource{ID: "fallback", MimeType: "video/mp4"}
	disposition := resourceDownloadDisposition(resource, `..\evil\镜头 01.mp4`)
	if !strings.HasPrefix(disposition, "attachment") || !strings.Contains(disposition, "filename") || strings.Contains(disposition, "evil") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
}

func TestResourceDownloadUsesAttachmentDisposition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateLocalSchema(db); err != nil {
		t.Fatal(err)
	}
	svc := app.NewLocal(repository.New(db), dataDir)
	owner, err := svc.LocalWorkspaceOwner()
	if err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{
		ID:        "download",
		UserID:    owner.ID,
		Kind:      "image",
		MimeType:  "image/png",
		Provider:  "local",
		Status:    model.ResourceStatusReady,
		ObjectKey: "download.png",
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "resources", "download.png"), []byte("png-bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterUserDataRoutes(router.Group("/api"), svc)
	request := httptest.NewRequest(http.MethodGet, "/api/resources/download/file?proxy=1&download=1&filename=asset.png", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "png-bytes" {
		t.Fatalf("download response = %d %s", response.Code, response.Body.String())
	}
	if disposition := response.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "attachment") || !strings.Contains(disposition, "asset.png") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
	if response.Header().Get("Content-Security-Policy") != "sandbox allow-downloads" {
		t.Fatalf("Content-Security-Policy = %q", response.Header().Get("Content-Security-Policy"))
	}
}
