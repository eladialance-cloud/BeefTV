package hostedapi

import (
	"net/http"
	"path/filepath"
	"testing"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestBootstrapAdminRegistrationGatesAdminRoutes drives the whole chain the
// hosted profile ships: register with a session cookie, resolve that session on
// a later request, and gate the admin routes on the resulting role. It proves
// the bootstrap username becomes an admin while a normal sign-up does not.
func TestBootstrapAdminRegistrationGatesAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-bootstrap.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.AuthSession{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	authService := auth.New(repo, auth.Config{})
	creditService := credits.New(repo, credits.Config{})

	engine := gin.New()
	api := engine.Group("/api")
	RegisterAuthRoutes(api, authService, AuthOptions{BootstrapAdminUsername: "root"})
	resolver := SessionUserResolver(authService)
	RegisterCreditRoutes(api, creditService, resolver)
	RegisterCreditAdminRoutes(api, creditService, repo, resolver, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})

	root := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{"username": "root", "password": "root-secret-1"}, nil)
	if root.Code != http.StatusOK {
		t.Fatalf("引导管理员注册应成功，得到 %d %s", root.Code, root.Body.String())
	}
	rootCookie := sessionCookie(t, root)
	if rootCookie == nil {
		t.Fatal("注册后必须下发会话 cookie")
	}

	alice := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{"username": "alice", "password": "alice-secret-1"}, nil)
	if alice.Code != http.StatusOK {
		t.Fatalf("普通用户注册应成功，得到 %d %s", alice.Code, alice.Body.String())
	}
	aliceCookie := sessionCookie(t, alice)
	if aliceCookie == nil {
		t.Fatal("普通用户注册后必须下发会话 cookie")
	}

	// The bootstrap account can read the admin console.
	if recorder := authRequest(t, engine, http.MethodGet, "/api/admin/credits/accounts", nil, rootCookie); recorder.Code != http.StatusOK {
		t.Fatalf("引导管理员应能进后台，得到 %d %s", recorder.Code, recorder.Body.String())
	}
	// A normal sign-up is rejected, not merely hidden by the front end.
	if recorder := authRequest(t, engine, http.MethodGet, "/api/admin/credits/accounts", nil, aliceCookie); recorder.Code != http.StatusForbidden {
		t.Fatalf("普通用户进后台必须 403，得到 %d %s", recorder.Code, recorder.Body.String())
	}
	// No session at all is a 401, not a 403 or a shared workspace owner.
	if recorder := authRequest(t, engine, http.MethodGet, "/api/admin/credits/accounts", nil, nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录进后台必须 401，得到 %d %s", recorder.Code, recorder.Body.String())
	}
	// The normal user keeps the user-facing wallet surface.
	if recorder := authRequest(t, engine, http.MethodGet, "/api/credits/wallet", nil, aliceCookie); recorder.Code != http.StatusOK {
		t.Fatalf("普通用户应能读自己的钱包，得到 %d %s", recorder.Code, recorder.Body.String())
	}
}
