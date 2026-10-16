package payment

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

type wechatNativeRequest struct {
	AppID       string `json:"appid"`
	MchID       string `json:"mchid"`
	Description string `json:"description"`
	OutTradeNo  string `json:"out_trade_no"`
	NotifyURL   string `json:"notify_url"`
	Amount      struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

type wechatCallbackEnvelope struct {
	ID       string `json:"id"`
	Resource struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		AssociatedData string `json:"associated_data"`
		Nonce          string `json:"nonce"`
		OriginalType   string `json:"original_type"`
	} `json:"resource"`
}

type wechatTransaction struct {
	AppID         string `json:"appid"`
	MchID         string `json:"mchid"`
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	Amount        struct {
		Total int64 `json:"total"`
	} `json:"amount"`
}

func buildWechatNative(ctx context.Context, client *http.Client, endpoint string, config *model.PaymentProviderConfig, credentials secrets, order *model.PaymentOrder, now time.Time) (string, error) {
	if config == nil || strings.TrimSpace(config.AppID) == "" {
		return "", invalidGatewayConfig("缺少微信 AppID")
	}
	if strings.TrimSpace(config.MchID) == "" {
		return "", invalidGatewayConfig("缺少微信商户号")
	}
	if strings.TrimSpace(config.MchSerialNo) == "" {
		return "", invalidGatewayConfig("缺少微信商户证书序列号")
	}
	if strings.TrimSpace(config.NotifyURL) == "" {
		return "", invalidGatewayConfig("缺少微信回调地址")
	}
	if strings.TrimSpace(credentials.APIKey) == "" {
		return "", invalidGatewayConfig("缺少微信 APIv3 密钥")
	}
	if strings.TrimSpace(credentials.PrivateKey) == "" {
		return "", invalidGatewayConfig("缺少微信商户私钥")
	}
	if order == nil || order.AmountFen <= 0 {
		return "", kernel.BadAuthRequest("支付订单金额无效")
	}
	privateKey, err := parseRSAPrivateKey(credentials.PrivateKey)
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "微信商户私钥格式无效", err)
	}
	payload := wechatNativeRequest{
		AppID:       config.AppID,
		MchID:       config.MchID,
		Description: order.ProductName,
		OutTradeNo:  order.ID,
		NotifyURL:   config.NotifyURL,
	}
	payload.Amount.Total = order.AmountFen
	payload.Amount.Currency = "CNY"
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultWechatNativeEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("微信支付接口地址无效")
	}
	nonce, err := randomHex(16)
	if err != nil {
		return "", err
	}
	timestamp := fmt.Sprintf("%d", now.Unix())
	message := "POST\n" + parsed.RequestURI() + "\n" + timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	signature, err := signRSASHA256(privateKey, message)
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeInternal, "微信支付请求签名失败", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		config.MchID, nonce, signature, timestamp, config.MchSerialNo))
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", kernel.WrapAppError(kernel.CodeBadGateway, "微信支付下单请求失败", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", kernel.WrapAppError(kernel.CodeBadGateway, "微信支付下单失败", errors.New(strings.TrimSpace(string(responseBody))))
	}
	var result struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return "", kernel.WrapAppError(kernel.CodeBadGateway, "微信支付下单响应无法解析", err)
	}
	if strings.TrimSpace(result.CodeURL) == "" {
		return "", kernel.NewAppError(kernel.CodeBadGateway, "微信支付未返回付款二维码")
	}
	return result.CodeURL, nil
}

func parseWechatNotification(config *model.PaymentProviderConfig, credentials secrets, headers http.Header, body []byte) (Notification, error) {
	if config == nil {
		return Notification{}, errors.New("wechat config is nil")
	}
	if strings.TrimSpace(credentials.PlatformPublicKey) == "" {
		return Notification{}, invalidGatewayConfig("缺少微信平台公钥或证书")
	}
	if len(credentials.APIKey) != 32 {
		return Notification{}, invalidGatewayConfig("微信 APIv3 密钥必须是 32 字节")
	}
	timestamp := strings.TrimSpace(headers.Get("Wechatpay-Timestamp"))
	nonce := strings.TrimSpace(headers.Get("Wechatpay-Nonce"))
	signature := strings.TrimSpace(headers.Get("Wechatpay-Signature"))
	if timestamp == "" || nonce == "" || signature == "" {
		return Notification{}, kernel.BadAuthRequest("微信回调缺少验签请求头")
	}
	publicKey, err := parseRSAPublicKey(credentials.PlatformPublicKey)
	if err != nil {
		return Notification{}, err
	}
	message := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	if err := verifyRSASHA256(publicKey, message, signature); err != nil {
		return Notification{}, kernel.BadAuthRequest("微信回调验签失败")
	}
	var envelope wechatCallbackEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return Notification{}, kernel.BadAuthRequest("微信回调格式无效")
	}
	decrypted, err := decryptWechatResource(credentials.APIKey, envelope.Resource.Ciphertext, envelope.Resource.Nonce, envelope.Resource.AssociatedData)
	if err != nil {
		return Notification{}, kernel.BadAuthRequest("微信回调密文解密失败")
	}
	var transaction wechatTransaction
	if err := json.Unmarshal(decrypted, &transaction); err != nil {
		return Notification{}, kernel.BadAuthRequest("微信交易数据格式无效")
	}
	if strings.TrimSpace(transaction.AppID) != "" && transaction.AppID != config.AppID {
		return Notification{}, errors.New("wechat appid mismatch")
	}
	if strings.TrimSpace(transaction.MchID) != "" && transaction.MchID != config.MchID {
		return Notification{}, errors.New("wechat mchid mismatch")
	}
	orderID := strings.TrimSpace(transaction.OutTradeNo)
	if orderID == "" {
		return Notification{}, kernel.BadAuthRequest("微信回调缺少订单号")
	}
	status := model.PaymentOrderPendingPayment
	if strings.ToUpper(strings.TrimSpace(transaction.TradeState)) == "SUCCESS" {
		status = model.PaymentOrderPaid
	}
	notificationID := strings.TrimSpace(envelope.ID)
	if notificationID == "" {
		notificationID = strings.TrimSpace(transaction.TransactionID)
	}
	if notificationID == "" {
		notificationID = orderID + ":" + transaction.TradeState
	}
	return Notification{
		Provider:               model.PaymentProviderWeChat,
		ProviderNotificationID: notificationID,
		ProviderTradeNo:        strings.TrimSpace(transaction.TransactionID),
		OrderID:                orderID,
		Status:                 status,
		AmountFen:              transaction.Amount.Total,
		PayloadJSON:            string(decrypted),
	}, nil
}

func decryptWechatResource(apiKey string, ciphertext string, nonce string, associatedData string) ([]byte, error) {
	key := []byte(apiKey)
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, []byte(nonce), data, []byte(associatedData))
}

func randomHex(size int) (string, error) {
	if size <= 0 {
		size = 16
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
