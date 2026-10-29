package hostedapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

type fakeTopupService struct {
	products []model.TopupProduct
	orders   []model.PaymentOrder
}

func (f *fakeTopupService) ListTopupProducts(includeDisabled bool) ([]model.TopupProduct, error) {
	if includeDisabled {
		return f.products, nil
	}
	var enabled []model.TopupProduct
	for index := range f.products {
		if f.products[index].Enabled {
			enabled = append(enabled, f.products[index])
		}
	}
	return enabled, nil
}

func (f *fakeTopupService) CreateTopupProduct(_ *model.User, input payment.TopupProductInput) (*model.TopupProduct, error) {
	item := model.TopupProduct{ID: "p-new", Name: input.Name, PriceFen: input.PriceFen, Credits: input.Credits, Bonus: input.Bonus, Provider: input.Provider, Enabled: true}
	f.products = append(f.products, item)
	return &item, nil
}

func (f *fakeTopupService) UpdateTopupProduct(_ *model.User, id string, input payment.TopupProductInput) (*model.TopupProduct, error) {
	for index := range f.products {
		if f.products[index].ID == id {
			f.products[index].Name = input.Name
			updated := f.products[index]
			return &updated, nil
		}
	}
	return nil, kernel.NotFound("充值商品不存在")
}

func (f *fakeTopupService) DeleteTopupProduct(_ *model.User, id string) error {
	for index := range f.products {
		if f.products[index].ID == id {
			f.products = append(f.products[:index], f.products[index+1:]...)
			return nil
		}
	}
	return kernel.NotFound("充值商品不存在")
}

func (f *fakeTopupService) CreatePaymentOrder(userID string, productID string, provider string) (*model.PaymentOrder, error) {
	if userID == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	order := model.PaymentOrder{ID: "o-1", UserID: userID, ProductID: productID, Provider: provider, Status: model.PaymentOrderPendingPayment}
	f.orders = append(f.orders, order)
	return &order, nil
}

func (f *fakeTopupService) OrdersByUser(userID string, limit int) ([]model.PaymentOrder, error) {
	var out []model.PaymentOrder
	for index := range f.orders {
		if f.orders[index].UserID == userID {
			out = append(out, f.orders[index])
		}
	}
	return out, nil
}

func (f *fakeTopupService) Orders(limit int) ([]model.PaymentOrder, error) { return f.orders, nil }

func (f *fakeTopupService) MarkPaid(_ *model.User, orderID string, providerTradeNo string) (*model.PaymentOrder, error) {
	for index := range f.orders {
		if f.orders[index].ID == orderID {
			f.orders[index].Status = model.PaymentOrderPaid
			f.orders[index].ProviderTradeNo = providerTradeNo
			updated := f.orders[index]
			return &updated, nil
		}
	}
	return nil, kernel.NotFound("支付订单不存在")
}

func (f *fakeTopupService) ClosePaymentOrder(orderID string, reason string) (*model.PaymentOrder, error) {
	for index := range f.orders {
		if f.orders[index].ID == orderID {
			f.orders[index].Status = model.PaymentOrderClosed
			updated := f.orders[index]
			return &updated, nil
		}
	}
	return nil, kernel.NotFound("支付订单不存在")
}

func (f *fakeTopupService) RefundPaymentOrder(_ *model.User, orderID string, reason string, providerRefundNo string) (*model.PaymentOrder, error) {
	for index := range f.orders {
		if f.orders[index].ID == orderID {
			now := time.Now().UTC()
			f.orders[index].Status = model.PaymentOrderRefunded
			f.orders[index].RefundedAt = &now
			f.orders[index].RefundReason = reason
			f.orders[index].RefundProviderNo = providerRefundNo
			f.orders[index].RefundReversed = true
			updated := f.orders[index]
			return &updated, nil
		}
	}
	return nil, kernel.NotFound("支付订单不存在")
}

func (f *fakeTopupService) ExpirePendingOrders(limit int) (int64, error) {
	return 0, nil
}

func (f *fakeTopupService) Reconcile() (*payment.Reconciliation, error) {
	return &payment.Reconciliation{OrdersByStatus: map[string]int64{}}, nil
}

func newTopupTestEnv(t *testing.T) (*gin.Engine, *fakeTopupService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &fakeTopupService{products: []model.TopupProduct{
		{ID: "p-1", Name: "1000 积分", PriceFen: 990, Credits: 1000, Bonus: 100, Provider: model.PaymentProviderAlipay, Enabled: true},
		{ID: "p-2", Name: "停用商品", PriceFen: 100, Credits: 100, Provider: model.PaymentProviderAlipay, Enabled: false},
	}}
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader(testCreditHeader) {
		case "admin":
			return &model.User{ID: testAdminID, Role: model.UserRoleAdmin}, nil
		case "alice":
			return &model.User{ID: testAliceID, Role: model.UserRoleUser}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	api := engine.Group("/api")
	RegisterTopupRoutes(api, service, resolve)
	RegisterTopupAdminRoutes(api, service, resolve, func(user *model.User) error {
		if user.Role != model.UserRoleAdmin {
			return kernel.Forbidden("需要管理员权限")
		}
		return nil
	})
	return engine, service
}

func TestTopupProductsHideDisabledForUsers(t *testing.T) {
	engine, _ := newTopupTestEnv(t)
	recorder := paymentRequest(t, engine, http.MethodGet, "/api/topup/products", nil, "alice")
	if recorder.Code != http.StatusOK {
		t.Fatalf("商品列表应 200，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Items []model.TopupProduct `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "p-1" {
		t.Fatalf("普通用户应只看到启用商品，得到 %+v", payload.Data.Items)
	}
}

func TestTopupProductsRequireLogin(t *testing.T) {
	engine, _ := newTopupTestEnv(t)
	recorder := paymentRequest(t, engine, http.MethodGet, "/api/topup/products", nil, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录取商品应 401，得到 %d", recorder.Code)
	}
}

func TestTopupAdminRoutesRejectNonAdmin(t *testing.T) {
	engine, _ := newTopupTestEnv(t)
	for _, path := range []string{"/api/admin/topup-products", "/api/admin/payment-orders"} {
		recorder := paymentRequest(t, engine, http.MethodGet, path, nil, "alice")
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("普通用户访问 %s 应 403，得到 %d", path, recorder.Code)
		}
	}
	recorder := paymentRequest(t, engine, http.MethodPost, "/api/admin/topup-products", map[string]any{"name": "x", "priceFen": 1, "credits": 1, "provider": "alipay"}, "alice")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("普通用户新增商品应 403，得到 %d", recorder.Code)
	}
}

func TestTopupOrderLifecycle(t *testing.T) {
	engine, _ := newTopupTestEnv(t)

	created := paymentRequest(t, engine, http.MethodPost, "/api/topup/orders", map[string]any{"productId": "p-1", "provider": "alipay"}, "alice")
	if created.Code != http.StatusOK {
		t.Fatalf("下单应 200，得到 %d: %s", created.Code, created.Body.String())
	}

	listed := paymentRequest(t, engine, http.MethodGet, "/api/topup/orders", nil, "alice")
	if listed.Code != http.StatusOK {
		t.Fatalf("订单列表应 200，得到 %d", listed.Code)
	}
	var mine struct {
		Data struct {
			Items []model.PaymentOrder `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &mine); err != nil {
		t.Fatal(err)
	}
	if len(mine.Data.Items) != 1 || mine.Data.Items[0].UserID != testAliceID {
		t.Fatalf("用户只能看到自己的订单，得到 %+v", mine.Data.Items)
	}

	paid := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-orders/o-1/mark-paid", map[string]any{"providerTradeNo": "trade-1"}, "admin")
	if paid.Code != http.StatusOK {
		t.Fatalf("管理员确认支付应 200，得到 %d: %s", paid.Code, paid.Body.String())
	}
	var result struct {
		Data struct {
			Item model.PaymentOrder `json:"item"`
		} `json:"data"`
	}
	if err := json.Unmarshal(paid.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Item.Status != model.PaymentOrderPaid {
		t.Fatalf("确认后订单应为已支付，得到 %q", result.Data.Item.Status)
	}

	closed := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-orders/o-1/close", map[string]any{"reason": "x"}, "admin")
	if closed.Code != http.StatusOK {
		t.Fatalf("关闭订单应 200，得到 %d", closed.Code)
	}
}

func TestTopupAdminPaymentOrderReconciliationRefundAndExpireRoutes(t *testing.T) {
	engine, service := newTopupTestEnv(t)
	service.orders = append(service.orders, model.PaymentOrder{ID: "o-paid", UserID: testAliceID, ProductID: "p-1", ProductName: "1000 积分", Provider: model.PaymentProviderAlipay, AmountFen: 990, Credits: 1100, Status: model.PaymentOrderPaid})

	reconciliation := paymentRequest(t, engine, http.MethodGet, "/api/admin/payment-orders/reconciliation", nil, "admin")
	if reconciliation.Code != http.StatusOK {
		t.Fatalf("对账接口应 200，得到 %d: %s", reconciliation.Code, reconciliation.Body.String())
	}

	refunded := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-orders/o-paid/refund", map[string]any{"reason": "用户申请", "providerRefundNo": "refund-1"}, "admin")
	if refunded.Code != http.StatusOK {
		t.Fatalf("退款接口应 200，得到 %d: %s", refunded.Code, refunded.Body.String())
	}

	expired := paymentRequest(t, engine, http.MethodPost, "/api/admin/payment-orders/expire", nil, "admin")
	if expired.Code != http.StatusOK {
		t.Fatalf("超时清理接口应 200，得到 %d: %s", expired.Code, expired.Body.String())
	}
}
