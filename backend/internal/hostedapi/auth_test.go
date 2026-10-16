package hostedapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/auth"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newAuthTestAPI wires the real auth service over a throwaway SQLite database.
// It exercises the GORM adapter as well as the HTTP surface; the connection is
// closed in cleanup so Windows temp-dir removal does not trip on open handles.
func newAuthTestEnv(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.AuthSession{}); err != nil {
		t.Fatal(err)
	}
	service := auth.New(repository.New(db), auth.Config{})
	engine := gin.New()
	RegisterAuthRoutes(engine.Group("/api"), service, AuthOptions{})
	return engine, service
}

// newAuthTestAPI is the convenience wrapper for tests that only need the routes.
func newAuthTestAPI(t *testing.T) *gin.Engine {
	t.Helper()
	engine, _ := newAuthTestEnv(t)
	return engine
}

func authRequest(t *testing.T, engine *gin.Engine, method string, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = strings.NewReader(string(encoded))
	} else {
		reader = strings.NewReader("")
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == SessionCookieName && cookie.Value != "" {
			return cookie
		}
	}
	return nil
}

func decodeAuthUser(t *testing.T, recorder *httptest.ResponseRecorder) *model.User {
	t.Helper()
	var envelope struct {
		Data struct {
			User model.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, recorder.Body.String())
	}
	return &envelope.Data.User
}

func TestAuthMeRequiresSession(t *testing.T) {
	engine := newAuthTestAPI(t)
	recorder := authRequest(t, engine, http.MethodGet, "/api/auth/me", nil, nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问 /api/auth/me 应返回 401，得到 %d (%s)", recorder.Code, recorder.Body.String())
	}
}

func TestRegisterIssuesSessionAndNeverEchoesSecrets(t *testing.T) {
	engine := newAuthTestAPI(t)
	recorder := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "alice", "displayName": "Alice", "password": "password-1",
	}, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("注册应成功，得到 %d (%s)", recorder.Code, recorder.Body.String())
	}
	cookie := sessionCookie(t, recorder)
	if cookie == nil {
		t.Fatal("注册成功后必须下发会话 cookie")
	}
	body := recorder.Body.String()
	for _, leaked := range []string{"password-1", "passwordHash", "PasswordHash", "tokenHash"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("注册响应泄露了敏感字段 %q: %s", leaked, body)
		}
	}
	if !cookie.HttpOnly {
		t.Fatal("会话 cookie 必须是 HttpOnly")
	}

	me := authRequest(t, engine, http.MethodGet, "/api/auth/me", nil, cookie)
	if me.Code != http.StatusOK {
		t.Fatalf("携带会话访问 /api/auth/me 应返回 200，得到 %d (%s)", me.Code, me.Body.String())
	}
	if user := decodeAuthUser(t, me); user.Username != "alice" || user.Role != model.UserRoleUser {
		t.Fatalf("会话解析用户不符: %#v", user)
	}
}

func TestRegisterRejectsDuplicateAndWeakPassword(t *testing.T) {
	engine := newAuthTestAPI(t)
	first := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "bob", "password": "password-1",
	}, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("首次注册应成功，得到 %d (%s)", first.Code, first.Body.String())
	}

	duplicate := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "bob", "password": "password-2",
	}, nil)
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("重复登录名应返回 400，得到 %d (%s)", duplicate.Code, duplicate.Body.String())
	}

	weak := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "carol", "password": "short",
	}, nil)
	if weak.Code != http.StatusBadRequest {
		t.Fatalf("弱密码应返回 400，得到 %d (%s)", weak.Code, weak.Body.String())
	}
}

func TestLoginRejectsWrongPasswordAndAcceptsCorrectOne(t *testing.T) {
	engine := newAuthTestAPI(t)
	registered := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "dave", "password": "password-1",
	}, nil)
	if registered.Code != http.StatusOK {
		t.Fatalf("注册应成功，得到 %d", registered.Code)
	}

	wrong := authRequest(t, engine, http.MethodPost, "/api/login", map[string]any{
		"username": "dave", "password": "wrong-password",
	}, nil)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("错误密码应返回 401，得到 %d (%s)", wrong.Code, wrong.Body.String())
	}
	if sessionCookie(t, wrong) != nil {
		t.Fatal("登录失败不得下发会话 cookie")
	}

	right := authRequest(t, engine, http.MethodPost, "/api/login", map[string]any{
		"username": "dave", "password": "password-1",
	}, nil)
	if right.Code != http.StatusOK {
		t.Fatalf("正确密码应返回 200，得到 %d (%s)", right.Code, right.Body.String())
	}
	if sessionCookie(t, right) == nil {
		t.Fatal("登录成功必须下发会话 cookie")
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	engine := newAuthTestAPI(t)
	registered := authRequest(t, engine, http.MethodPost, "/api/register", map[string]any{
		"username": "erin", "password": "password-1",
	}, nil)
	cookie := sessionCookie(t, registered)
	if cookie == nil {
		t.Fatal("注册后应拿到会话 cookie")
	}

	logout := authRequest(t, engine, http.MethodPost, "/api/logout", nil, cookie)
	if logout.Code != http.StatusOK {
		t.Fatalf("登出应返回 200，得到 %d (%s)", logout.Code, logout.Body.String())
	}

	after := authRequest(t, engine, http.MethodGet, "/api/auth/me", nil, cookie)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("登出后会话必须失效，得到 %d (%s)", after.Code, after.Body.String())
	}
}
