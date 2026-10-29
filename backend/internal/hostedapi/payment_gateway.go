package hostedapi

import (
	"io"
	"net/http"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

const paymentCallbackMaxBytes = 1 << 20

// PaymentGatewayService is the user launch plus provider callback surface.
type PaymentGatewayService interface {
	BeginPayment(userID string, orderID string) (*payment.Launch, error)
	ProcessCallback(provider string, headers http.Header, body []byte) (*model.PaymentOrder, error)
}

// RegisterPaymentRoutes mounts the browser payment launch route and the
// unauthenticated gateway callbacks. Callback responses follow each gateway's
// own success contract, so these handlers do not use the JSON error envelope.
func RegisterPaymentRoutes(r *gin.RouterGroup, svc PaymentGatewayService, resolve ActorResolver) {
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

	r.POST("/topup/orders/:id/pay", func(c *gin.Context) {
		actor, ok := user(c)
		if !ok {
			return
		}
		launch, err := svc.BeginPayment(actor.ID, c.Param("id"))
		if err != nil {
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"payment": launch})
	})

	notify := func(c *gin.Context) {
		provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, paymentCallbackMaxBytes))
		if err != nil {
			writeGatewayCallbackError(c, provider, kernel.BadAuthRequest("支付回调内容过大或读取失败"))
			return
		}
		if len(body) == 0 && c.Request.URL.RawQuery != "" {
			body = []byte(c.Request.URL.Query().Encode())
		}
		if _, err := svc.ProcessCallback(provider, c.Request.Header.Clone(), body); err != nil {
			writeGatewayCallbackError(c, provider, err)
			return
		}
		if provider == model.PaymentProviderAlipay || provider == model.PaymentProviderEPay {
			c.String(http.StatusOK, "success")
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
	}
	r.POST("/payments/:provider/notify", notify)
	r.GET("/payments/:provider/notify", notify)
}

func writeGatewayCallbackError(c *gin.Context, provider string, err error) {
	if provider == model.PaymentProviderAlipay || provider == model.PaymentProviderEPay {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	message := "回调处理失败"
	if err != nil {
		message = err.Error()
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": message})
}
