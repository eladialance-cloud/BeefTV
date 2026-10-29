package payment

import (
	"net/url"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
)

func TestEpaySignAndCallbackVerification(t *testing.T) {
	values := url.Values{
		"pid":          {"1001"},
		"type":         {"alipay"},
		"out_trade_no": {"ORDER1"},
		"notify_url":   {"https://example.com/notify"},
		"return_url":   {"https://example.com/return"},
		"name":         {"积分"},
		"money":        {"9.90"},
	}
	if got := epaySign(values, "secret"); got != "82153cfd3f56b6725f6ec3ddd6a972c7" {
		t.Fatalf("epaySign() = %q", got)
	}

	values.Set("trade_no", "TRADE1")
	values.Set("trade_status", "TRADE_SUCCESS")
	values.Set("sign", epaySign(values, "secret"))
	values.Set("sign_type", "MD5")
	config := &model.PaymentProviderConfig{Provider: model.PaymentProviderEPay, AppID: "1001"}
	notification, err := parseEpayNotification(config, secrets{APIKey: "secret"}, []byte(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if notification.OrderID != "ORDER1" || notification.ProviderTradeNo != "TRADE1" || notification.AmountFen != 990 || notification.Status != model.PaymentOrderPaid {
		t.Fatalf("notification = %#v", notification)
	}

	values.Set("money", "1.00")
	if _, err := parseEpayNotification(config, secrets{APIKey: "secret"}, []byte(values.Encode())); err == nil {
		t.Fatal("tampered amount must fail signature verification")
	}
}

func TestBuildEpayLaunchUsesConfiguredGatewayAndPayType(t *testing.T) {
	config := &model.PaymentProviderConfig{
		Provider:  model.PaymentProviderEPay,
		AppID:     "1001",
		Endpoint:  "https://pay.example.com/submit.php",
		NotifyURL: "https://api.example.com/api/payments/epay/notify",
		ReturnURL: "https://app.example.com/wallet",
		PayType:   "wxpay",
	}
	order := &model.PaymentOrder{ID: "ORDER1", ProductName: "1000 积分", AmountFen: 990}
	launch, err := buildEpayLaunch("", config, secrets{APIKey: "secret"}, order, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if launch.Mode != "redirect" || launch.URL == "" {
		t.Fatalf("launch = %#v", launch)
	}
	parsed, err := url.Parse(launch.URL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "pay.example.com" || parsed.Path != "/submit.php" || parsed.Query().Get("type") != "wxpay" || parsed.Query().Get("out_trade_no") != "ORDER1" {
		t.Fatalf("launch URL = %s", launch.URL)
	}
	if epaySign(parsed.Query(), "secret") != parsed.Query().Get("sign") {
		t.Fatal("launch signature is invalid")
	}
}
