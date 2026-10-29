package hostedapi

import (
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

// TopupService 是充值接口所需的最小服务面。
type TopupService interface {
	ListTopupProducts(includeDisabled bool) ([]model.TopupProduct, error)
	CreateTopupProduct(actor *model.User, input payment.TopupProductInput) (*model.TopupProduct, error)
	UpdateTopupProduct(actor *model.User, id string, input payment.TopupProductInput) (*model.TopupProduct, error)
	DeleteTopupProduct(actor *model.User, id string) error
	CreatePaymentOrder(userID string, productID string, provider string) (*model.PaymentOrder, error)
	OrdersByUser(userID string, limit int) ([]model.PaymentOrder, error)
	Orders(limit int) ([]model.PaymentOrder, error)
	MarkPaid(actor *model.User, orderID string, providerTradeNo string) (*model.PaymentOrder, error)
	ClosePaymentOrder(orderID string, reason string) (*model.PaymentOrder, error)
	RefundPaymentOrder(actor *model.User, orderID string, reason string, providerRefundNo string) (*model.PaymentOrder, error)
	ExpirePendingOrders(limit int) (int64, error)
	Reconcile() (*payment.Reconciliation, error)
}

// RegisterTopupRoutes mounts the user-facing recharge surface: browse SKUs and
// start an order. The actual gateway hand-off is wired in a later change; an
// order created here already carries the snapshot and idempotency key.
func RegisterTopupRoutes(r *gin.RouterGroup, svc TopupService, resolve ActorResolver) {
	group := r.Group("/topup")

	user := func(c *gin.Context) (*model.User, bool) {
		actor, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if actor == nil || strings.TrimSpace(actor.ID) == "" {
			writeError(c, kernel.Unauthorized("请先登录"))
			return nil, false
		}
		return actor, true
	}

	group.GET("/products", func(c *gin.Context) {
		if _, ok := user(c); !ok {
			return
		}
		items, err := svc.ListTopupProducts(false)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.GET("/orders", func(c *gin.Context) {
		actor, ok := user(c)
		if !ok {
			return
		}
		items, err := svc.OrdersByUser(actor.ID, parseLimit(c.Query("limit"), 50))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})

	group.POST("/orders", func(c *gin.Context) {
		actor, ok := user(c)
		if !ok {
			return
		}
		var input payment.CreateOrderInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("下单参数格式错误"))
			return
		}
		order, err := svc.CreatePaymentOrder(actor.ID, input.ProductID, input.Provider)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": order})
	})
}

// RegisterTopupAdminRoutes mounts the admin console: manage recharge SKUs, list
// orders, confirm an offline payment, and close unpaid orders.
func RegisterTopupAdminRoutes(r *gin.RouterGroup, svc TopupService, resolve ActorResolver, requireAdmin AdminGuard) {
	admin := func(c *gin.Context) (*model.User, bool) {
		actor, err := resolve(c)
		if err != nil {
			writeError(c, err)
			return nil, false
		}
		if err := requireAdmin(actor); err != nil {
			writeError(c, err)
			return nil, false
		}
		return actor, true
	}

	products := r.Group("/admin/topup-products")
	products.GET("", func(c *gin.Context) {
		if _, ok := admin(c); !ok {
			return
		}
		items, err := svc.ListTopupProducts(true)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})
	products.POST("", func(c *gin.Context) {
		actor, ok := admin(c)
		if !ok {
			return
		}
		var input payment.TopupProductInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("充值商品参数格式错误"))
			return
		}
		item, err := svc.CreateTopupProduct(actor, input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
	products.PUT("/:id", func(c *gin.Context) {
		actor, ok := admin(c)
		if !ok {
			return
		}
		var input payment.TopupProductInput
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("充值商品参数格式错误"))
			return
		}
		item, err := svc.UpdateTopupProduct(actor, c.Param("id"), input)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
	products.DELETE("/:id", func(c *gin.Context) {
		actor, ok := admin(c)
		if !ok {
			return
		}
		if err := svc.DeleteTopupProduct(actor, c.Param("id")); err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"deleted": true})
	})

	orders := r.Group("/admin/payment-orders")
	orders.GET("", func(c *gin.Context) {
		if _, ok := admin(c); !ok {
			return
		}
		items, err := svc.Orders(parseLimit(c.Query("limit"), 50))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"items": items})
	})
	orders.GET("/reconciliation", func(c *gin.Context) {
		if _, ok := admin(c); !ok {
			return
		}
		snapshot, err := svc.Reconcile()
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": snapshot})
	})
	orders.POST("/expire", func(c *gin.Context) {
		if _, ok := admin(c); !ok {
			return
		}
		closed, err := svc.ExpirePendingOrders(200)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"closed": closed})
	})
	orders.POST("/:id/refund", func(c *gin.Context) {
		actor, ok := admin(c)
		if !ok {
			return
		}
		var input struct {
			Reason           string `json:"reason"`
			ProviderRefundNo string `json:"providerRefundNo"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("退款参数格式错误"))
			return
		}
		item, err := svc.RefundPaymentOrder(actor, c.Param("id"), input.Reason, input.ProviderRefundNo)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
	orders.POST("/:id/mark-paid", func(c *gin.Context) {
		actor, ok := admin(c)
		if !ok {
			return
		}
		var input struct {
			ProviderTradeNo string `json:"providerTradeNo"`
		}
		_ = c.ShouldBindJSON(&input)
		item, err := svc.MarkPaid(actor, c.Param("id"), input.ProviderTradeNo)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
	orders.POST("/:id/close", func(c *gin.Context) {
		if _, ok := admin(c); !ok {
			return
		}
		var input struct {
			Reason string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&input)
		item, err := svc.ClosePaymentOrder(c.Param("id"), input.Reason)
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"item": item})
	})
}

func parseLimit(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	if value > 200 {
		return 200
	}
	return value
}
