package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestHostedPublicResourceSignedURLServesBytesAndRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_PUBLIC_BASE_URL", "https://127.0.0.1")
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "public-resource.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.MigrateLocalSchema(db); err != nil {
		t.Fatal(err)
	}
	svc := app.NewLocal(repository.New(db), dataDir)
	owner, err := svc.LocalWorkspaceOwner()
	if err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{
		ID: "audio-public", UserID: owner.ID, Kind: "audio", MimeType: "audio/wav",
		Provider: "local", Status: model.ResourceStatusReady, ObjectKey: "voice.wav", Size: 12,
	}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "resources", "voice.wav"), []byte("hello-audio!"), 0o644); err != nil {
		t.Fatal(err)
	}
	signed, err := svc.DirectResourceURL(owner.ID, resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	RegisterHostedPublicResourceRoutes(router.Group("/api"), svc)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil))
	if response.Code != http.StatusOK || response.Body.String() != "hello-audio!" {
		t.Fatalf("signed resource = %d %q", response.Code, response.Body.String())
	}

	rangeRequest := httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil)
	rangeRequest.Header.Set("Range", "bytes=0-4")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, rangeRequest)
	if response.Code != http.StatusPartialContent || response.Body.String() != "hello" {
		t.Fatalf("signed range = %d %q", response.Code, response.Body.String())
	}

	query := parsed.Query()
	query.Set("signature", "invalid")
	parsed.RawQuery = query.Encode()
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("invalid signature = %d %s", response.Code, response.Body.String())
	}
}
