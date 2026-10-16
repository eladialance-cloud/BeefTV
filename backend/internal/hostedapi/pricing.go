package hostedapi

import (
	"strings"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

// PricingAdminService is the hosted pricing control surface needed by the
// admin console. The route kind is validated by the service.
type PricingAdminService interface {
	Rules(kind string) ([]model.PricingRule, error)
	Save(actor *model.User, kind string, id string, input credits.PricingInput) (*model.PricingRule, error)
	Delete(actor *model.User, kind string, id string) error
}

// RegisterPricingAdminRoutes mounts one generic CRUD surface for the three
// pricing layers: model, channel_tier, and logical_sku.
func RegisterPricingAdminRoutes(r *gin.RouterGroup, svc PricingAdminService, resolve ActorResolver, requireAdmin AdminGuard) {
	group := r.Group("/admin/pricing")

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

	group.GET("/:kind", func(c *gin.Context) {
		if _, ok := actor(c); !ok {
			return
		}
		items, err := svc.Rules(c.Param("kind"))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.POST("/:kind", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input credits.PricingInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("价格规则参数格式错误"))
			return
		}
		item, err := svc.Save(user, c.Param("kind"), "", input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.PUT("/:kind/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		var input credits.PricingInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("价格规则参数格式错误"))
			return
		}
		item, err := svc.Save(user, c.Param("kind"), strings.TrimSpace(c.Param("id")), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})

	group.DELETE("/:kind/:id", func(c *gin.Context) {
		user, ok := actor(c)
		if !ok {
			return
		}
		if err := svc.Delete(user, c.Param("kind"), strings.TrimSpace(c.Param("id"))); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})
}
