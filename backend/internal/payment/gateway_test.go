package payment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func testRSAKeyPair(t *testing.T) (*rsa.PrivateKey, string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return key, privatePEM, publicPEM
}

func TestBuildAlipayPagePaySignsRequest(t *testing.T) {
	key, privatePEM, publicPEM := testRSAKeyPair(t)
	config := &model.PaymentProviderConfig{
		AppID: "2021000000000000", NotifyURL: "https://example.com/api/payments/alipay/notify",
	}
	order := &model.PaymentOrder{ID: "order-1", ProductName: "1000 积分", AmountFen: 990}
	target, err := buildAlipayPagePay("https://openapi.alipay.test/gateway.do", config, secrets{PrivateKey: privatePEM}, order,
		time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	values := parsed.Query()
	if values.Get("app_id") != config.AppID || values.Get("out_trade_no") != "" {
		t.Fatalf("支付宝页面支付公共参数不对: %v", values)
	}
	if !strings.Contains(values.Get("biz_content"), `"out_trade_no":"order-1"`) ||
		!strings.Contains(values.Get("biz_content"), `"total_amount":"9.90"`) {
		t.Fatalf("支付宝业务参数不对: %s", values.Get("biz_content"))
	}
	publicKey, err := parseRSAPublicKey(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyRSASHA256(publicKey, sortedQuery(values, false), values.Get("sign")); err != nil {
		t.Fatalf("支付宝请求签名无法验证: %v", err)
	}
	if _, err := signRSASHA256(key, "test"); err != nil {
		t.Fatal(err)
	}
}

func TestParseAlipayNotificationVerifiesAndParses(t *testing.T) {
	key, privatePEM, publicPEM := testRSAKeyPair(t)
	config := &model.PaymentProviderConfig{AppID: "2021000000000000"}
	values := url.Values{
		"app_id":       []string{config.AppID},
		"notify_id":    []string{"notify-1"},
		"out_trade_no": []string{"order-1"},
		"trade_no":     []string{"trade-1"},
		"trade_status": []string{"TRADE_SUCCESS"},
		"total_amount": []string{"9.90"},
	}
	signature, err := signRSASHA256(key, sortedQuery(values, true))
	if err != nil {
		t.Fatal(err)
	}
	values.Set("sign_type", "RSA2")
	values.Set("sign", signature)
	notification, err := parseAlipayNotification(config, secrets{PlatformPublicKey: publicPEM}, []byte(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if notification.Provider != model.PaymentProviderAlipay || notification.OrderID != "order-1" ||
		notification.Status != model.PaymentOrderPaid || notification.AmountFen != 990 {
		t.Fatalf("支付宝回调解析结果不对: %+v", notification)
	}
	if privatePEM == "" {
		t.Fatal("private key fixture must not be empty")
	}
}

func TestBuildWechatNativeSignsAndReturnsCodeURL(t *testing.T) {
	key, privatePEM, publicPEM := testRSAKeyPair(t)
	client := &http.Client{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/pay/transactions/native" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		authorization := r.Header.Get("Authorization")
		if !strings.Contains(authorization, `mchid="1900000001"`) || !strings.Contains(authorization, `serial_no="serial-1"`) {
			t.Fatalf("authorization header missing merchant identity: %s", authorization)
		}
		parts := strings.Split(strings.TrimPrefix(authorization, "WECHATPAY2-SHA256-RSA2048 "), ",")
		fields := map[string]string{}
		for _, part := range parts {
			name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
			if ok {
				fields[name] = strings.Trim(value, `"`)
			}
		}
		publicKey, err := parseRSAPublicKey(publicPEM)
		if err != nil {
			t.Fatal(err)
		}
		message := "POST\n" + r.URL.RequestURI() + "\n" + fields["timestamp"] + "\n" + fields["nonce_str"] + "\n" + string(body) + "\n"
		if err := verifyRSASHA256(publicKey, message, fields["signature"]); err != nil {
			t.Fatalf("request signature invalid: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=test"}`))
	}))
	defer server.Close()
	client = server.Client()
	config := &model.PaymentProviderConfig{
		AppID: "wx-app-1", MchID: "1900000001", MchSerialNo: "serial-1",
		NotifyURL: "https://example.com/api/payments/wechat/notify",
	}
	order := &model.PaymentOrder{ID: "order-1", ProductName: "1000 积分", AmountFen: 990}
	codeURL, err := buildWechatNative(t.Context(), client, server.URL+"/v3/pay/transactions/native", config,
		secrets{APIKey: strings.Repeat("a", 32), PrivateKey: privatePEM}, order, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if codeURL != "weixin://wxpay/bizpayurl?pr=test" {
		t.Fatalf("unexpected code_url: %s", codeURL)
	}
	if _, err := signRSASHA256(key, "test"); err != nil {
		t.Fatal(err)
	}
}

func TestParseWechatNotificationVerifiesAndDecrypts(t *testing.T) {
	key, privatePEM, publicPEM := testRSAKeyPair(t)
	apiKey := strings.Repeat("b", 32)
	config := &model.PaymentProviderConfig{AppID: "wx-app-1", MchID: "1900000001"}
	transaction := wechatTransaction{
		AppID: "wx-app-1", MchID: "1900000001", OutTradeNo: "order-1",
		TransactionID: "trade-1", TradeState: "SUCCESS",
	}
	transaction.Amount.Total = 990
	plain, err := json.Marshal(transaction)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("123456789012")
	associatedData := []byte("transaction")
	block, err := aes.NewCipher([]byte(apiKey))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := aead.Seal(nil, nonce, plain, associatedData)
	envelope := wechatCallbackEnvelope{ID: "notify-1"}
	envelope.Resource.Algorithm = "AEAD_AES_256_GCM"
	envelope.Resource.Ciphertext = base64.StdEncoding.EncodeToString(ciphertext)
	envelope.Resource.Nonce = string(nonce)
	envelope.Resource.AssociatedData = string(associatedData)
	envelope.Resource.OriginalType = "transaction"
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := "1710000000"
	headerNonce := "header-nonce"
	signature, err := signRSASHA256(key, timestamp+"\n"+headerNonce+"\n"+string(body)+"\n")
	if err != nil {
		t.Fatal(err)
	}
	headers := http.Header{}
	headers.Set("Wechatpay-Timestamp", timestamp)
	headers.Set("Wechatpay-Nonce", headerNonce)
	headers.Set("Wechatpay-Signature", signature)
	notification, err := parseWechatNotification(config, secrets{APIKey: apiKey, PlatformPublicKey: publicPEM}, headers, body)
	if err != nil {
		t.Fatal(err)
	}
	if notification.Provider != model.PaymentProviderWeChat || notification.OrderID != "order-1" ||
		notification.Status != model.PaymentOrderPaid || notification.AmountFen != 990 {
		t.Fatalf("微信回调解析结果不对: %+v", notification)
	}
	if privatePEM == "" {
		t.Fatal("private key fixture must not be empty")
	}
}

func TestApplyNotificationRejectsAmountMismatchBeforeCrediting(t *testing.T) {
	service, repo, credits := newTopupService(t)
	order, err := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ApplyNotification(Notification{
		Provider:               model.PaymentProviderAlipay,
		ProviderNotificationID: "notify-amount-1",
		OrderID:                order.ID,
		Status:                 model.PaymentOrderPaid,
		AmountFen:              1,
	})
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != kernel.CodeConflict {
		t.Fatalf("金额不一致必须返回 409，得到 %v", err)
	}
	if len(credits.calls) != 0 || len(repo.notifications) != 0 || len(repo.orders) != 1 {
		t.Fatalf("金额不一致不得入账或留成功回调: credits=%d notifications=%d orders=%d",
			len(credits.calls), len(repo.notifications), len(repo.orders))
	}
}
