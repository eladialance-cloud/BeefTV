package hostedapi

import (
	"errors"
	"log"
	"strings"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"

	"github.com/gin-gonic/gin"
)

// PricingQuoteService resolves the server-owned price for the current task
// parameters. It is intentionally read-only so the browser can show an
// estimate without being able to write a price or bypass submission billing.
type PricingQuoteService interface {
	Quote(request credits.QuoteRequest) (*credits.Quote, error)
}

type pricingQuoteRequest struct {
	LogicalModelID string            `json:"logicalModelId"`
	ChannelID      string            `json:"channelId"`
	ChannelModelID string            `json:"channelModelId"`
	ModelKey       string            `json:"modelKey"`
	Capability     string            `json:"capability"`
	BillingMode    string            `json:"billingMode"`
	Selector       map[string]string `json:"selector"`
	Quantity       int64             `json:"quantity"`
}

type pricingQuoteCandidate struct {
	mode     string
	quantity int64
}

// RegisterPricingQuoteRoutes mounts the authenticated, read-only quote used by
// creation surfaces to render the cost of their current parameter set.
func RegisterPricingQuoteRoutes(r *gin.RouterGroup, svc PricingQuoteService, resolve ActorResolver) {
	r.POST("/pricing/quote", func(c *gin.Context) {
		c.Header("X-Pricing-Quote-Version", "2")
		if _, err := resolve(c); err != nil {
			writeError(c, err)
			return
		}

		var input pricingQuoteRequest
		if err := c.ShouldBindJSON(&input); err != nil {
			writeError(c, kernel.BadAuthRequest("积分报价参数格式错误"))
			return
		}
		quote, err := resolvePricingQuote(svc, credits.QuoteRequest{
			LogicalModelID: input.LogicalModelID,
			ChannelID:      input.ChannelID,
			ChannelModelID: input.ChannelModelID,
			ModelKey:       input.ModelKey,
			Capability:     input.Capability,
			BillingMode:    input.BillingMode,
			Selector:       input.Selector,
			Quantity:       input.Quantity,
		})
		if err != nil {
			log.Printf(
				"pricing_quote_failed request_id=%s channel_id=%s channel_model_id=%s logical_model_id=%s model_key=%s capability=%s billing_mode=%s selector=%v quantity=%d error=%v",
				c.GetHeader("X-Request-ID"),
				input.ChannelID,
				input.ChannelModelID,
				input.LogicalModelID,
				input.ModelKey,
				input.Capability,
				input.BillingMode,
				input.Selector,
				input.Quantity,
				err,
			)
			writeError(c, err)
			return
		}
		writeOK(c, gin.H{"quote": quote})
	})
}

func resolvePricingQuote(svc PricingQuoteService, request credits.QuoteRequest) (*credits.Quote, error) {
	if !strings.EqualFold(strings.TrimSpace(request.BillingMode), "auto") {
		return svc.Quote(request)
	}

	candidates := []pricingQuoteCandidate{{mode: "fixed_request", quantity: 1}}
	switch request.Capability {
	case "video", "audio":
		candidates = append(candidates, pricingQuoteCandidate{mode: "per_second", quantity: request.Quantity})
	case "image":
		candidates = append(candidates, pricingQuoteCandidate{mode: "per_image", quantity: request.Quantity})
	case "text":
		candidates = append(candidates, pricingQuoteCandidate{mode: "per_1k_tokens", quantity: request.Quantity})
	}

	var notFound error
	for _, candidate := range candidates {
		quote, err := svc.Quote(credits.QuoteRequest{
			LogicalModelID: request.LogicalModelID,
			ChannelID:      request.ChannelID,
			ChannelModelID: request.ChannelModelID,
			ModelKey:       request.ModelKey,
			Capability:     request.Capability,
			BillingMode:    candidate.mode,
			Selector:       request.Selector,
			Quantity:       candidate.quantity,
			At:             request.At,
		})
		if err == nil {
			return quote, nil
		}
		var appErr *kernel.AppError
		if !errors.As(err, &appErr) || appErr.Status != 404 {
			return nil, err
		}
		notFound = err
	}
	if notFound == nil {
		notFound = kernel.NotFound("未找到有效的模型价格规则")
	}
	return nil, notFound
}
