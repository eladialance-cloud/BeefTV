package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/app"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// InviteCodeAdminService is the hosted admin surface for registration policy
// and invite-code operations. The service owns validation and audit fields.
type InviteCodeAdminService interface {
	RegistrationPolicy() (*app.RegistrationPolicy, error)
	UpdateRegistrationPolicy(actor *model.User, policy app.RegistrationPolicy) (*app.RegistrationPolicy, error)
	AdminInviteCodePage(actor *model.User, query app.AdminListQuery) (*app.InviteCodePage, error)
	GenerateInviteCodes(actor *model.User, req app.InviteCodeBatchRequest) ([]model.InviteCode, error)
	SetInviteCodeStatus(actor *model.User, id string, enabled bool) (*model.InviteCode, error)
	DeleteInviteCode(actor *model.User, id string) error
}

// RegisterInviteCodeAdminRoutes mounts registration policy and invite-code
// management for the admin console.
func RegisterInviteCodeAdminRoutes(r *gin.RouterGroup, svc InviteCodeAdminService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin")
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

	group.GET("/registration/policy", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		policy, err := svc.RegistrationPolicy()
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"policy": policy})
	})

	group.PUT("/registration/policy", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input app.RegistrationPolicy
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("注册策略参数格式错误"))
			return
		}
		policy, err := svc.UpdateRegistrationPolicy(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"policy": policy})
	})

	group.GET("/invite-codes", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		page, err := queryInt(c, "page", 1)
		if err != nil {
			writeError(c, err)
			return
		}
		limit, err := queryInt(c, "pageSize", 50)
		if err != nil {
			writeError(c, err)
			return
		}
		result, err := svc.AdminInviteCodePage(user, app.AdminListQuery{
			Keyword: strings.TrimSpace(c.Query("keyword")),
			Status:  strings.TrimSpace(c.Query("status")),
			Page:    page,
			Limit:   limit,
		})
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, result)
	})

	group.POST("/invite-codes", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input app.InviteCodeBatchRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("邀请码参数格式错误"))
			return
		}
		items, err := svc.GenerateInviteCodes(user, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items, "count": len(items)})
	})

	group.POST("/invite-codes/:id/enable", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.SetInviteCodeStatus(user, strings.TrimSpace(c.Param("id")), true)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.POST("/invite-codes/:id/disable", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		item, err := svc.SetInviteCodeStatus(user, strings.TrimSpace(c.Param("id")), false)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/invite-codes/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.DeleteInviteCode(user, strings.TrimSpace(c.Param("id"))); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})
}
