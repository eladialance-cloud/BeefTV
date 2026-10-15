package hostedapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
	"infinite-canvas/backend/internal/storageconfig"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type okProber struct{}

func (okProber) Probe(context.Context, storageconfig.Target) error { return nil }

func newAdminRouter(t *testing.T, actor *model.User) (*gin.Engine, *storageconfig.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:hostedapi-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.StorageLocation{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	svc := storageconfig.New(repo, t.TempDir(), okProber{})
	router := gin.New()
	group := router.Group("/api")
	RegisterStorageAdminRoutes(group, svc,
		func(*gin.Context) (*model.User, error) { return actor, nil },
		func(user *model.User) error {
			if user.Role != model.UserRoleAdmin {
				return app.Forbidden("需要管理员权限")
			}
			return nil
		})
	return router, svc
}

func doJSON(t *testing.T, router *gin.Engine, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

const createBody = `{"name":"primary","provider":"s3","endpoint":"https://oss-example.aliyuncs.com","region":"cn-hangzhou","bucket":"beeftv","accessKeyId":"AKIDEXAMPLE","secretAccessKey":"super-secret-value"}`

func TestAdminStorageRoutesRejectNonAdmin(t *testing.T) {
	router, _ := newAdminRouter(t, &model.User{ID: "user-1", Role: model.UserRoleUser})
	recorder := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations", createBody)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("非管理员应被拒绝，得到 %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminStorageRoutesNeverReturnPlaintextSecret(t *testing.T) {
	router, _ := newAdminRouter(t, &model.User{ID: "admin-1", Role: model.UserRoleAdmin})
	created := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations", createBody)
	if created.Code != http.StatusOK {
		t.Fatalf("新增失败: %d %s", created.Code, created.Body.String())
	}
	if strings.Contains(created.Body.String(), "super-secret-value") {
		t.Fatal("响应不得包含明文密钥")
	}
	var payload struct {
		Data struct {
			Item struct {
				ID                    string `json:"id"`
				SecretAccessKeyMasked string `json:"secretAccessKeyMasked"`
				IsDefault             bool   `json:"isDefault"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if payload.Data.Item.ID == "" || payload.Data.Item.SecretAccessKeyMasked == "" {
		t.Fatalf("响应缺少必要字段: %s", created.Body.String())
	}

	listed := doJSON(t, router, http.MethodGet, "/api/admin/storage-locations", "")
	if listed.Code != http.StatusOK || strings.Contains(listed.Body.String(), "super-secret-value") {
		t.Fatalf("列表必须掩码密钥，得到 %d %s", listed.Code, listed.Body.String())
	}
}

func TestAdminStorageRoutesSetDefaultRequiresProbe(t *testing.T) {
	router, _ := newAdminRouter(t, &model.User{ID: "admin-1", Role: model.UserRoleAdmin})
	created := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations", createBody)
	var payload struct {
		Data struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	id := payload.Data.Item.ID
	if id == "" {
		t.Fatalf("未取到配置 ID: %s", created.Body.String())
	}

	before := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations/"+id+"/default", "")
	if before.Code != http.StatusConflict {
		t.Fatalf("未测试的配置设默认应返回 409，得到 %d %s", before.Code, before.Body.String())
	}

	tested := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations/"+id+"/test", "")
	if tested.Code != http.StatusOK {
		t.Fatalf("连通性测试应通过: %d %s", tested.Code, tested.Body.String())
	}
	after := doJSON(t, router, http.MethodPost, "/api/admin/storage-locations/"+id+"/default", "")
	if after.Code != http.StatusOK {
		t.Fatalf("测试通过后设默认应成功: %d %s", after.Code, after.Body.String())
	}
}
