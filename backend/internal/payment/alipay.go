package payment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func buildAlipayPagePay(endpoint string, config *model.PaymentProviderConfig, credentials secrets, order *model.PaymentOrder, now time.Time) (string, error) {
	if config == nil || strings.TrimSpace(config.AppID) == "" {
		return "", invalidGatewayConfig("缺少支付宝 AppID")
	}
	if strings.TrimSpace(credentials.PrivateKey) == "" {
		return "", invalidGatewayConfig("缺少支付宝应用私钥")
	}
	if strings.TrimSpace(config.NotifyURL) == "" {
		return "", invalidGatewayConfig("缺少支付宝回调地址")
	}
	if order == nil || order.AmountFen <= 0 {
		return "", kernel.BadAuthRequest("支付订单金额无效")
	}
	privateKey, err := parseRSAPrivateKey(credentials.PrivateKey)
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "支付宝应用私钥格式无效", err)
	}
	bizContent, err := json.Marshal(map[string]string{
		"out_trade_no": order.ID,
		"total_amount": formatFenToYuan(order.AmountFen),
		"subject":      order.ProductName,
		"product_code": "FAST_INSTANT_TRADE_PAY",
	})
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Set("app_id", config.AppID)
	params.Set("method", "alipay.trade.page.pay")
	params.Set("format", "JSON")
	params.Set("charset", "utf-8")
	params.Set("sign_type", "RSA2")
	params.Set("timestamp", now.In(time.FixedZone("CST", 8*60*60)).Format("2006-01-02 15:04:05"))
	params.Set("version", "1.0")
	params.Set("notify_url", config.NotifyURL)
	params.Set("biz_content", string(bizContent))
	signature, err := signRSASHA256(privateKey, sortedQuery(params, false))
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "支付宝请求签名失败", err)
	}
	params.Set("sign", signature)
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultAlipayEndpoint
	}
	return endpoint + "?" + params.Encode(), nil
}

func parseAlipayNotification(config *model.PaymentProviderConfig, credentials secrets, body []byte) (Notification, error) {
	if config == nil {
		return Notification{}, errors.New("alipay config is nil")
	}
	if strings.TrimSpace(credentials.PlatformPublicKey) == "" {
		return Notification{}, invalidGatewayConfig("缺少支付宝平台公钥")
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return Notification{}, kernel.BadAuthRequest("支付宝回调格式无效")
	}
	appID := strings.TrimSpace(values.Get("app_id"))
	if appID == "" || appID != strings.TrimSpace(config.AppID) {
		return Notification{}, errors.New("alipay app_id mismatch")
	}
	publicKey, err := parseRSAPublicKey(credentials.PlatformPublicKey)
	if err != nil {
		return Notification{}, err
	}
	if err := verifyRSASHA256(publicKey, sortedQuery(values, true), values.Get("sign")); err != nil {
		return Notification{}, kernel.BadAuthRequest("支付宝回调验签失败")
	}
	orderID := strings.TrimSpace(values.Get("out_trade_no"))
	if orderID == "" {
		return Notification{}, kernel.BadAuthRequest("支付宝回调缺少订单号")
	}
	amountFen, err := parseYuanToFen(values.Get("total_amount"))
	if err != nil {
		return Notification{}, kernel.BadAuthRequest("支付宝回调金额格式无效")
	}
	status := model.PaymentOrderPendingPayment
	tradeStatus := strings.ToUpper(strings.TrimSpace(values.Get("trade_status")))
	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		status = model.PaymentOrderPaid
	}
	notificationID := strings.TrimSpace(values.Get("notify_id"))
	if notificationID == "" {
		notificationID = strings.TrimSpace(values.Get("trade_no"))
	}
	if notificationID == "" {
		notificationID = orderID + ":" + tradeStatus
	}
	payload, _ := json.Marshal(values)
	return Notification{
		Provider:               model.PaymentProviderAlipay,
		ProviderNotificationID: notificationID,
		ProviderTradeNo:        strings.TrimSpace(values.Get("trade_no")),
		OrderID:                orderID,
		Status:                 status,
		AmountFen:              amountFen,
		PayloadJSON:            string(payload),
	}, nil
}

func sortedQuery(values url.Values, callback bool) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "sign" || callback && key == "sign_type" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		for _, value := range values[key] {
			pairs = append(pairs, key+"="+value)
		}
	}
	return strings.Join(pairs, "&")
}

func formatFenToYuan(fen int64) string {
	return fmt.Sprintf("%d.%02d", fen/100, fen%100)
}

func parseYuanToFen(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return 0, errors.New("invalid amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid amount")
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || yuan < 0 {
		return 0, errors.New("invalid amount")
	}
	cents := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if fraction == "" || len(fraction) > 2 {
			return 0, errors.New("invalid amount")
		}
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("invalid amount")
		}
	}
	if yuan > (1<<63-1-cents)/100 {
		return 0, errors.New("amount overflow")
	}
	return yuan*100 + cents, nil
}
