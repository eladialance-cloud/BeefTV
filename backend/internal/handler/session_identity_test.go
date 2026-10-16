package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func newIdentityContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return context
}

func TestCurrentUserPrefersHostedSessionUser(t *testing.T) {
	context := newIdentityContext()
	MarkHostedRequest(context)
	SetSessionUser(context, &model.User{
		ID: "user-1", Username: "alice", Role: model.UserRoleUser, Status: model.UserStatusActive,
	})

	user, err := currentUser(context, &app.Service{})
	if err != nil {
		t.Fatalf("托管请求带会话应解析出用户，得到 %v", err)
	}
	if user == nil || user.ID != "user-1" || user.Username != "alice" {
		t.Fatalf("解析出的用户不符: %#v", user)
	}
}

func TestCurrentUserRejectsHostedRequestWithoutSession(t *testing.T) {
	context := newIdentityContext()
	MarkHostedRequest(context)

	user, err := currentUser(context, &app.Service{})
	if user != nil {
		t.Fatalf("托管请求没有会话时不得返回用户: %#v", user)
	}
	var appErr *kernel.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusUnauthorized {
		t.Fatalf("托管请求没有会话应返回 401，得到 %v", err)
	}
}

// 桌面路径的关键不变量：没有 hosted 标记时，即使上下文里恰好放了会话用户，
// currentUser 也只能走本地工作区 owner，绝不采纳会话用户。
func TestCurrentUserIgnoresSessionUserWithoutHostedMarker(t *testing.T) {
	context := newIdentityContext()
	SetSessionUser(context, &model.User{ID: "user-1", Username: "alice"})

	user, err := currentUser(context, &app.Service{})
	if user != nil && user.ID == "user-1" {
		t.Fatal("桌面路径不得返回会话用户")
	}
	if err == nil {
		t.Fatalf("空服务应报本地工作区未初始化，得到 %#v", user)
	}
}

func TestSessionUserIgnoresNilAndWrongType(t *testing.T) {
	context := newIdentityContext()
	SetSessionUser(context, nil)
	if _, ok := SessionUser(context); ok {
		t.Fatal("nil 不得发布为会话用户")
	}
	context.Set(sessionUserContextKey, "not-a-user")
	if _, ok := SessionUser(context); ok {
		t.Fatal("类型不符不得发布为会话用户")
	}
}

func TestIsHostedRequestTracksTheMarker(t *testing.T) {
	context := newIdentityContext()
	if IsHostedRequest(context) {
		t.Fatal("未标记的请求不应被视为托管请求")
	}
	MarkHostedRequest(context)
	if !IsHostedRequest(context) {
		t.Fatal("标记后应被视为托管请求")
	}
}
