package hostedapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/credits"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type fakePricingAdminService struct {
	rules        []model.PricingRule
	saved        *model.PricingRule
	deletedID    string
	gotKind      string
	gotID        string
	gotActorID   string
	gotInput     credits.PricingInput
	listErr      error
	saveErr      error
	deleteError  error
	quote        *credits.Quote
	quoteErr     error
	gotQuote     credits.QuoteRequest
	quotesByMode map[string]*credits.Quote
	quoteModes   []string
}

func (f *fakePricingAdminService) Rules(kind string) ([]model.PricingRule, error) {
	f.gotKind = kind
	return f.rules, f.listErr
}

func (f *fakePricingAdminService) Save(actor *model.User, kind string, id string, input credits.PricingInput) (*model.PricingRule, error) {
	f.gotActorID = actor.ID
	f.gotKind = kind
	f.gotID = id
	f.gotInput = input
	return f.saved, f.saveErr
}

func (f *fakePricingAdminService) Delete(actor *model.User, kind string, id string) error {
	f.gotActorID = actor.ID
	f.gotKind = kind
	f.gotID = id
	f.deletedID = id
	return f.deleteError
}

func (f *fakePricingAdminService) Quote(request credits.QuoteRequest) (*credits.Quote, error) {
	f.gotQuote = request
	f.quoteModes = append(f.quoteModes, request.BillingMode)
	if f.quotesByMode != nil {
		quote := f.quotesByMode[request.BillingMode]
		if quote == nil {
			return nil, kernel.NotFound("未找到有效的模型价格规则")
		}
		return quote, nil
	}
	return f.quote, f.quoteErr
}

func newPricingAPITestEnv(t *testing.T) (*gin.Engine, *fakePricingAdminService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &fakePricingAdminService{
		rules: []model.PricingRule{{ID: "rule-1", Kind: model.PricingKindModel, Name: "默认价"}},
		saved: &model.PricingRule{ID: "rule-2", Kind: model.PricingKindModel, Name: "新价格"},
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
	RegisterPricingAdminRoutes(engine.Group("/api"), service, resolve, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})
	RegisterPricingQuoteRoutes(engine.Group("/api"), service, resolve)
	return engine, service
}

func TestPricingQuoteRequiresLoginAndReturnsServerEstimate(t *testing.T) {
	engine, service := newPricingAPITestEnv(t)
	service.quote = &credits.Quote{
		RuleID: "video-1080", RuleKind: model.PricingKindLogicalSKU, BillingMode: "per_second",
		Quantity: 5, UnitPriceMicrocredits: 500_000, EstimatedCredits: 2_500_000,
	}
	body := map[string]any{
		"logicalModelId": "logical-video",
		"channelId":      "channel-1",
		"channelModelId": "video-model",
		"modelKey":       "video-model",
		"capability":     "video",
		"billingMode":    "per_second",
		"selector":       map[string]string{"quality": "1080p"},
		"quantity":       5,
	}
	anonymous := creditRequest(t, engine, http.MethodPost, "/api/pricing/quote", body, "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("未登录报价应 401，得到 %d", anonymous.Code)
	}
	response := creditRequest(t, engine, http.MethodPost, "/api/pricing/quote", body, "alice")
	if response.Code != http.StatusOK {
		t.Fatalf("用户报价应 200，得到 %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Pricing-Quote-Version") != "2" {
		t.Fatalf("报价版本头 = %q", response.Header().Get("X-Pricing-Quote-Version"))
	}
	if service.gotQuote.LogicalModelID != "logical-video" || service.gotQuote.ModelKey != "video-model" || service.gotQuote.Quantity != 5 || service.gotQuote.Selector["quality"] != "1080p" {
		t.Fatalf("quote request = %#v", service.gotQuote)
	}
	if !strings.Contains(response.Body.String(), `"estimatedCredits":2500000`) {
		t.Fatalf("quote response = %s", response.Body.String())
	}
}

func TestPricingQuoteAutoTriesFixedBeforeUsageBilling(t *testing.T) {
	engine, service := newPricingAPITestEnv(t)
	service.quotesByMode = map[string]*credits.Quote{
		"per_second": {
			RuleID: "video-seconds", RuleKind: model.PricingKindChannelTier, BillingMode: "per_second",
			Quantity: 5, UnitPriceMicrocredits: 500_000, EstimatedCredits: 2_500_000,
		},
	}
	response := creditRequest(t, engine, http.MethodPost, "/api/pricing/quote", map[string]any{
		"channelId": "channel-1", "channelModelId": "video-model",
		"capability": "video", "billingMode": "auto",
		"selector": map[string]string{"quality": "1080p"}, "quantity": 5,
	}, "alice")
	if response.Code != http.StatusOK {
		t.Fatalf("自动报价应 200，得到 %d: %s", response.Code, response.Body.String())
	}
	if len(service.quoteModes) != 2 || service.quoteModes[0] != "fixed_request" || service.quoteModes[1] != "per_second" {
		t.Fatalf("自动报价计费顺序 = %#v", service.quoteModes)
	}
	if !strings.Contains(response.Body.String(), `"estimatedCredits":2500000`) {
		t.Fatalf("quote response = %s", response.Body.String())
	}
}

func TestPricingAdminRequiresLoginAndAdmin(t *testing.T) {
	engine, _ := newPricingAPITestEnv(t)
	anonymous := creditRequest(t, engine, http.MethodGet, "/api/admin/pricing/model", nil, "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问价格后台应 401，得到 %d", anonymous.Code)
	}
	ordinary := creditRequest(t, engine, http.MethodGet, "/api/admin/pricing/model", nil, "alice")
	if ordinary.Code != http.StatusForbidden {
		t.Fatalf("普通用户访问价格后台应 403，得到 %d", ordinary.Code)
	}
}

func TestPricingAdminListsAndSavesByKind(t *testing.T) {
	engine, service := newPricingAPITestEnv(t)
	list := creditRequest(t, engine, http.MethodGet, "/api/admin/pricing/logical_sku", nil, "admin")
	if list.Code != http.StatusOK {
		t.Fatalf("管理员列价格规则应 200，得到 %d：%s", list.Code, list.Body.String())
	}
	if service.gotKind != model.PricingKindLogicalSKU {
		t.Fatalf("列表必须把 kind 传给服务，得到 %q", service.gotKind)
	}

	body := map[string]any{
		"name": "720p", "logicalModelId": "lm-video", "capability": "video",
		"billingMode": "per_second", "selector": map[string]string{"quality": "720p"},
		"unitPriceMicrocredits": 100000, "enabled": true,
	}
	saved := creditRequest(t, engine, http.MethodPost, "/api/admin/pricing/logical_sku", body, "admin")
	if saved.Code != http.StatusOK {
		t.Fatalf("管理员保存价格规则应 200，得到 %d：%s", saved.Code, saved.Body.String())
	}
	if service.gotActorID != testAdminID || service.gotKind != model.PricingKindLogicalSKU || service.gotID != "" {
		t.Fatalf("保存必须带上管理员和路径 kind，得到 actor=%q kind=%q id=%q", service.gotActorID, service.gotKind, service.gotID)
	}
	if service.gotInput.LogicalModelID != "lm-video" || service.gotInput.Selector["quality"] != "720p" {
		t.Fatalf("保存参数未正确解析: %#v", service.gotInput)
	}
}

func TestPricingAdminUpdatesAndDeletesByID(t *testing.T) {
	engine, service := newPricingAPITestEnv(t)
	update := creditRequest(t, engine, http.MethodPut, "/api/admin/pricing/channel_tier/tier-1", map[string]any{
		"channelId": "channel-1", "channelModelId": "cm-1", "capability": "image",
		"billingMode": "fixed_request", "unitPriceMicrocredits": 300000,
	}, "admin")
	if update.Code != http.StatusOK {
		t.Fatalf("管理员更新价格规则应 200，得到 %d：%s", update.Code, update.Body.String())
	}
	if service.gotKind != model.PricingKindChannelTier || service.gotID != "tier-1" {
		t.Fatalf("更新路径参数未传给服务: kind=%q id=%q", service.gotKind, service.gotID)
	}

	deleted := creditRequest(t, engine, http.MethodDelete, "/api/admin/pricing/channel_tier/tier-1", nil, "admin")
	if deleted.Code != http.StatusOK {
		t.Fatalf("管理员删除价格规则应 200，得到 %d：%s", deleted.Code, deleted.Body.String())
	}
	if service.deletedID != "tier-1" {
		t.Fatalf("删除 ID 未传给服务: %#v", service)
	}
}

func TestPricingAdminReturnsServiceError(t *testing.T) {
	engine, service := newPricingAPITestEnv(t)
	service.saveErr = kernel.BadAuthRequest("单价必须大于零")
	recorder := creditRequest(t, engine, http.MethodPost, "/api/admin/pricing/model", map[string]any{
		"logicalModelId": "lm-1", "capability": "image", "billingMode": "fixed_request",
	}, "admin")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("服务层校验失败必须返回 400，得到 %d：%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["msg"] != "单价必须大于零" {
		t.Fatalf("错误信息未透出，得到 %#v", payload)
	}
}
