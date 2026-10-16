package hostedapi

import (
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

// PaymentAdminService 是支付渠道后台所需的最小服务面，便于用假实现做 HTTP 层测试。
type PaymentAdminService interface {
	List() ([]payment.View, error)
	Create(actor *model.User, input payment.Input) (*payment.View, error)
	Update(actor *model.User, id string, input payment.Input) (*payment.View, error)
	Delete(actor *model.User, id string) error
}

// RegisterPaymentAdminRoutes 暴露支付渠道配置的后台增删改查。调用方必须只在
// server profile 装配，桌面依赖图不得包含该包。
func RegisterPaymentAdminRoutes(r *gin.RouterGroup, svc PaymentAdminService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/payment-providers")

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
		var input payment.Input
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("支付渠道参数格式错误"))
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
		var input payment.Input
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("支付渠道参数格式错误"))
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
}
