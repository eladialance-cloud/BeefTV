package payment

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

// buildEpayLaunch creates the standard EasyPay page-payment URL. EasyPay
// implementations share the same MD5 field-signing convention even when their
// domain and UI differ.
func buildEpayLaunch(_ string, config *model.PaymentProviderConfig, credentials secrets, order *model.PaymentOrder, _ time.Time) (*Launch, error) {
	if config == nil || order == nil {
		return nil, kernel.BadAuthRequest("易支付订单或渠道配置为空")
	}
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		return nil, kernel.BadAuthRequest("易支付网关地址不能为空")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, kernel.BadAuthRequest("易支付网关地址格式错误")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/submit.php"
	}
	payType := strings.ToLower(strings.TrimSpace(config.PayType))
	if payType == "" {
		payType = "alipay"
	}
	notifyURL := strings.TrimSpace(config.NotifyURL)
	if notifyURL == "" {
		return nil, kernel.BadAuthRequest("易支付回调地址 NotifyURL 不能为空")
	}
	returnURL := strings.TrimSpace(config.ReturnURL)
	if returnURL == "" {
		if publicBase := strings.TrimRight(strings.TrimSpace(os.Getenv("CANVAS_PUBLIC_BASE_URL")), "/"); publicBase != "" {
			returnURL = publicBase
		}
	}
	if strings.TrimSpace(credentials.APIKey) == "" {
		return nil, kernel.BadAuthRequest("易支付商户密钥不能为空")
	}
	if strings.TrimSpace(config.AppID) == "" {
		return nil, kernel.BadAuthRequest("易支付商户 PID 不能为空")
	}

	values := url.Values{
		"pid":          {config.AppID},
		"type":         {payType},
		"out_trade_no": {order.ID},
		"notify_url":   {notifyURL},
		"name":         {order.ProductName},
		"money":        {formatFenToYuan(order.AmountFen)},
	}
	if returnURL != "" {
		values.Set("return_url", returnURL)
	}
	values.Set("sign", epaySign(values, credentials.APIKey))
	values.Set("sign_type", "MD5")
	parsed.RawQuery = values.Encode()
	return &Launch{Provider: model.PaymentProviderEPay, Mode: "redirect", URL: parsed.String()}, nil
}

func parseEpayNotification(config *model.PaymentProviderConfig, credentials secrets, body []byte) (Notification, error) {
	if config == nil {
		return Notification{}, errors.New("易支付渠道配置为空")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return Notification{}, fmt.Errorf("易支付回调参数无效: %w", err)
	}
	if strings.TrimSpace(credentials.APIKey) == "" {
		return Notification{}, errors.New("易支付商户密钥为空")
	}
	provided := strings.TrimSpace(values.Get("sign"))
	if provided == "" || !strings.EqualFold(provided, epaySign(values, credentials.APIKey)) {
		return Notification{}, errors.New("易支付回调签名校验失败")
	}
	expectedPID := strings.TrimSpace(config.AppID)
	if expectedPID == "" {
		expectedPID = strings.TrimSpace(config.MchID)
	}
	if expectedPID != "" && strings.TrimSpace(values.Get("pid")) != expectedPID {
		return Notification{}, errors.New("易支付回调商户 PID 不匹配")
	}
	orderID := strings.TrimSpace(values.Get("out_trade_no"))
	tradeNo := strings.TrimSpace(values.Get("trade_no"))
	if orderID == "" || tradeNo == "" {
		return Notification{}, errors.New("易支付回调缺少订单号或交易号")
	}
	amountFen, err := parseYuanToFen(values.Get("money"))
	if err != nil {
		return Notification{}, fmt.Errorf("易支付回调金额无效: %w", err)
	}
	status := model.PaymentOrderPendingPayment
	switch strings.ToUpper(strings.TrimSpace(values.Get("trade_status"))) {
	case "TRADE_SUCCESS", "SUCCESS", "PAID":
		status = model.PaymentOrderPaid
	}
	payload, _ := json.Marshal(values)
	return Notification{
		Provider:               model.PaymentProviderEPay,
		ProviderNotificationID: tradeNo,
		ProviderTradeNo:        tradeNo,
		OrderID:                orderID,
		Status:                 status,
		ProviderStatus:         strings.TrimSpace(values.Get("trade_status")),
		AmountFen:              amountFen,
		PayloadJSON:            string(payload),
	}, nil
}

// epaySign follows EasyPay's MD5 rule: sort non-empty fields by key, join as
// key=value with ampersands, append the merchant key directly, then MD5.
func epaySign(values url.Values, key string) string {
	keys := make([]string, 0, len(values))
	for name, entries := range values {
		if name == "sign" || name == "sign_type" {
			continue
		}
		if len(entries) == 0 || strings.TrimSpace(entries[0]) == "" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, name := range keys {
		parts = append(parts, name+"="+values.Get(name))
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return hex.EncodeToString(sum[:])
}
