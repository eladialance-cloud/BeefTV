package hostedapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/payment"

	"github.com/gin-gonic/gin"
)

type fakePaymentGateway struct {
	launch      *payment.Launch
	order       *model.PaymentOrder
	beginErr    error
	callbackErr error
	gotUserID   string
	gotOrderID  string
	gotProvider string
	gotBody     []byte
	gotHeader   http.Header
}

func (f *fakePaymentGateway) BeginPayment(userID string, orderID string) (*payment.Launch, error) {
	f.gotUserID = userID
	f.gotOrderID = orderID
	return f.launch, f.beginErr
}

func (f *fakePaymentGateway) ProcessCallback(provider string, headers http.Header, body []byte) (*model.PaymentOrder, error) {
	f.gotProvider = provider
	f.gotHeader = headers.Clone()
	f.gotBody = append([]byte(nil), body...)
	return f.order, f.callbackErr
}

func newPaymentGatewayTestEnv(t *testing.T) (*gin.Engine, *fakePaymentGateway) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	service := &fakePaymentGateway{
		launch: &payment.Launch{Provider: model.PaymentProviderAlipay, Mode: "redirect", URL: "https://pay.example/alipay"},
		order:  &model.PaymentOrder{ID: "o-1", Status: model.PaymentOrderPaid},
	}
	resolve := func(c *gin.Context) (*model.User, error) {
		switch c.GetHeader(testCreditHeader) {
		case "alice":
			return &model.User{ID: testAliceID, Role: model.UserRoleUser}, nil
		default:
			return nil, kernel.Unauthorized("请先登录")
		}
	}
	engine := gin.New()
	RegisterPaymentRoutes(engine.Group("/api"), service, resolve)
	return engine, service
}

func TestPaymentLaunchRequiresLogin(t *testing.T) {
	engine, _ := newPaymentGatewayTestEnv(t)
	recorder := paymentRequest(t, engine, http.MethodPost, "/api/topup/orders/o-1/pay", nil, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("未登录发起支付应 401，得到 %d", recorder.Code)
	}
}

func TestPaymentLaunchUsesSessionUser(t *testing.T) {
	engine, service := newPaymentGatewayTestEnv(t)
	recorder := paymentRequest(t, engine, http.MethodPost, "/api/topup/orders/o-1/pay", nil, "alice")
	if recorder.Code != http.StatusOK {
		t.Fatalf("发起支付应 200，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if service.gotUserID != testAliceID || service.gotOrderID != "o-1" {
		t.Fatalf("发起支付必须使用会话用户，得到 user=%q order=%q", service.gotUserID, service.gotOrderID)
	}
}

func TestPaymentCallbackPassesRawBodyAndReturnsProviderContract(t *testing.T) {
	engine, service := newPaymentGatewayTestEnv(t)
	body := []byte("trade_status=TRADE_SUCCESS&out_trade_no=o-1")
	request := httptest.NewRequest(http.MethodPost, "/api/payments/alipay/notify", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "success" {
		t.Fatalf("支付宝成功回调必须返回 success，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
	if service.gotProvider != model.PaymentProviderAlipay || !bytes.Equal(service.gotBody, body) {
		t.Fatalf("回调原始请求体必须原样交给支付服务，provider=%q body=%q", service.gotProvider, service.gotBody)
	}
}

func TestWechatCallbackFailureUsesJsonContract(t *testing.T) {
	engine, service := newPaymentGatewayTestEnv(t)
	service.callbackErr = kernel.BadAuthRequest("签名错误")
	request := httptest.NewRequest(http.MethodPost, "/api/payments/wechat/notify", bytes.NewReader([]byte(`{"id":"n-1"}`)))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !bytes.Contains(recorder.Body.Bytes(), []byte(`"code":"FAIL"`)) {
		t.Fatalf("微信失败回调必须返回 JSON FAIL，得到 %d: %s", recorder.Code, recorder.Body.String())
	}
}
