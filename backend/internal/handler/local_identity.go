package handler

import (
	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"strings"

	"github.com/gin-gonic/gin"
)

// currentUser resolves the stable workspace owner. Local routes never inspect
// cookies and never create a session record.
func currentUser(c *gin.Context, svc *app.Service) (*model.User, error) {
	// 托管形态：会话中间件会把登录用户放进上下文；没有登录就是未授权，绝不回退到共享工作区 owner。
	if IsHostedRequest(c) {
		if isAssistantHostRequest(c, svc) {
			turnID := strings.TrimSpace(c.GetHeader(assistantTurnHeader))
			if turnID == "" {
				turnID = strings.TrimSpace(c.Param("turnId"))
			}
			if turnID == "" {
				return &model.User{ID: "assistant-host", Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
			}
			userID, found, err := svc.AssistantTurnUserIDForHost(turnID)
			if err != nil {
				return nil, err
			}
			if found {
				return &model.User{ID: userID, Role: model.UserRoleUser, Status: model.UserStatusActive}, nil
			}
		}
		if user, ok := SessionUser(c); ok {
			return user, nil
		}
		return nil, kernel.Unauthorized("请先登录")
	}
	if _, exists := c.Get(workspaceContextKey); exists {
		scope, err := CurrentWorkspace(c)
		if err != nil {
			return nil, err
		}
		return svc.WorkspaceOwner(scope.ID)
	}
	return svc.LocalWorkspaceOwner()
}
