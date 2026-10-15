// Package hostedapi carries HTTP surfaces that only exist in the server/hosted
// profile. Keeping them out of the desktop handler graph is deliberate: the
// desktop composition root must stay free of SaaS-only routes.
package hostedapi

import (
	"errors"
	"net/http"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/storageconfig"

	"github.com/gin-gonic/gin"
)

// ActorResolver resolves the acting user for an admin request.
type ActorResolver func(c *gin.Context) (*model.User, error)

// AdminGuard rejects actors that may not use the admin backend.
type AdminGuard func(user *model.User) error

// WorkspaceOwnerResolver 把当前工作区属主作为操作人。托管形态接入会话认证后，
// 只需替换这个解析器，路由与服务层不用改。
func WorkspaceOwnerResolver(svc *app.Service, workspaceID string) ActorResolver {
	return func(*gin.Context) (*model.User, error) { return svc.WorkspaceOwner(workspaceID) }
}

// RegisterStorageAdminRoutes 暴露对象存储配置的后台增删改查、连通性测试与设默认。
// 调用方必须只在 server profile 装配。
func RegisterStorageAdminRoutes(r *gin.RouterGroup, svc *storageconfig.Service, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/storage-locations")

	actor := func(c *gin.Context) (*model.User, bool) {
		user, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if err := requireAdmin(user); err != nil {
			writeError(c, err)
			return nil, false
		}
		return user, true
	}

	group.GET("", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		items, err := svc.List()
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.POST("", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input storageconfig.Input
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("对象存储配置参数格式错误"))
			return
		}
		view, err := svc.Create(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": view})
	})

	group.PUT("/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input storageconfig.Input
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("对象存储配置参数格式错误"))
			return
		}
		view, err := svc.Update(user, c.Param("id"), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": view})
	})

	group.DELETE("/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.Delete(user, c.Param("id")); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})

	group.POST("/:id/test", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		view, err := svc.TestConnection(c.Request.Context(), user, c.Param("id"))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": view})
	})

	group.POST("/:id/default", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		view, err := svc.SetDefault(user, c.Param("id"))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": view})
	})
}

func writeOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": app.CodeOK, "data": data, "msg": "ok"})
}

// writeError 复用 kernel 的结构化错误，但绝不对未分类错误回显原文。
func writeError(c *gin.Context, err error) {
	var appErr *kernel.AppError
	if errors.As(err, &appErr) && appErr.Status >= http.StatusBadRequest && appErr.Status <= 599 {
		code := appErr.Code
		if code == 0 {
			code = appErr.Status
		}
		reason := appErr.Reason
		if reason == "" {
			reason = kernel.ReasonForStatus(appErr.Status)
		}
		message := appErr.Message
		if appErr.Status >= http.StatusInternalServerError {
			message = "系统处理失败，请稍后重试"
		}
		if message == "" {
			message = http.StatusText(appErr.Status)
		}
		c.JSON(appErr.Status, gin.H{"code": code, "data": nil, "msg": message, "reason": reason})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"code": app.CodeInternal, "data": nil, "msg": "系统处理失败，请稍后重试", "reason": kernel.ReasonInternal,
	})
}
