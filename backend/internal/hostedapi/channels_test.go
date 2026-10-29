package hostedapi

import (
	"context"
	"net/http"
	"testing"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type fakeChannelAdminService struct {
	ChannelAdminService
	testResult   *app.AdminChannelModelTestResult
	testErr      error
	gotActorID   string
	gotChannelID string
	gotRequest   app.ChannelModelRequest
}

func (f *fakeChannelAdminService) TestAdminChannelModel(_ context.Context, actor *model.User, channelID string, req app.ChannelModelRequest) (*app.AdminChannelModelTestResult, error) {
	f.gotActorID = actor.ID
	f.gotChannelID = channelID
	f.gotRequest = req
	return f.testResult, f.testErr
}

func newChannelTestEnv(t *testing.T) (*gin.Engine, *fakeChannelAdminService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &fakeChannelAdminService{
		testResult: &app.AdminChannelModelTestResult{DurationMs: 321},
	}
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader(testCreditHeader) {
		case "admin":
			return &model.User{ID: testAdminID, Username: "admin", Role: model.UserRoleAdmin, Status: model.UserStatusActive}, nil
		case "alice":
			return &model.User{ID: testAliceID, Username: "alice", Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	RegisterChannelAdminRoutes(engine.Group("/api"), service, resolve, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})
	return engine, service
}

func TestChannelModelTestRequiresAdmin(t *testing.T) {
	engine, _ := newChannelTestEnv(t)
	anonymous := creditRequest(t, engine, http.MethodPost, "/api/admin/channels/ch-1/models/test", map[string]any{
		"modelKey": "text-1", "capability": "text", "protocol": "chat-completion",
	}, "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("未登录测试模型应 401，得到 %d", anonymous.Code)
	}
	ordinary := creditRequest(t, engine, http.MethodPost, "/api/admin/channels/ch-1/models/test", map[string]any{
		"modelKey": "text-1", "capability": "text", "protocol": "chat-completion",
	}, "alice")
	if ordinary.Code != http.StatusForbidden {
		t.Fatalf("普通用户测试模型应 403，得到 %d", ordinary.Code)
	}
}

func TestChannelModelTestPassesModelContract(t *testing.T) {
	engine, service := newChannelTestEnv(t)
	recorder := creditRequest(t, engine, http.MethodPost, "/api/admin/channels/ch-1/models/test", map[string]any{
		"modelKey": "text-1", "providerModelKey": "upstream-text", "displayName": "Text 1",
		"capability": "text", "protocol": "chat-completion", "enabled": true,
	}, "admin")
	if recorder.Code != http.StatusOK {
		t.Fatalf("管理员测试模型应 200，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	if service.gotActorID != testAdminID || service.gotChannelID != "ch-1" {
		t.Fatalf("测试必须透传管理员和渠道，得到 actor=%q channel=%q", service.gotActorID, service.gotChannelID)
	}
	if service.gotRequest.ModelKey != "text-1" || service.gotRequest.ProviderModelKey != "upstream-text" || service.gotRequest.Capability != "text" || service.gotRequest.Protocol != "chat-completion" {
		t.Fatalf("测试请求未正确解析: %#v", service.gotRequest)
	}
}
