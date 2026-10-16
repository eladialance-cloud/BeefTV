package payment

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	defaultAlipayEndpoint       = "https://openapi.alipay.com/gateway.do"
	defaultWechatNativeEndpoint = "https://api.mch.weixin.qq.com/v3/pay/transactions/native"
)

// Launch is what the browser needs to start a payment. Alipay returns a URL to
// redirect to; WeChat Native returns a code_url that should be rendered as a
// QR code.
type Launch struct {
	Provider string `json:"provider"`
	Mode     string `json:"mode"`
	URL      string `json:"url,omitempty"`
	QRCode   string `json:"qrCode,omitempty"`
}

// BeginPayment creates a gateway launch payload for one pending order. The
// order stores the provider config id chosen at checkout, so later admin edits
// cannot silently move an existing order to another merchant account.
func (s *Service) BeginPayment(userID string, orderID string) (*Launch, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	order, err := s.loadOrder(orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, kernel.NotFound("支付订单不存在")
	}
	if order.Status != model.PaymentOrderPendingPayment {
		return nil, kernel.NewAppError(kernel.CodeConflict, "当前订单状态不能发起支付")
	}
	config, err := s.providerConfigForOrder(order)
	if err != nil {
		return nil, err
	}
	credentials, err := decryptProviderSecrets(s.dataDir, config)
	if err != nil {
		return nil, err
	}
	switch order.Provider {
	case model.PaymentProviderAlipay:
		target, err := buildAlipayPagePay(s.alipayEndpoint, config, credentials, order, s.now())
		if err != nil {
			return nil, err
		}
		return &Launch{Provider: order.Provider, Mode: "redirect", URL: target}, nil
	case model.PaymentProviderWeChat:
		codeURL, err := buildWechatNative(context.Background(), s.httpClient, s.wechatNativeEndpoint, config, credentials, order, s.now())
		if err != nil {
			return nil, err
		}
		return &Launch{Provider: order.Provider, Mode: "qr", QRCode: codeURL}, nil
	default:
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
}

// ProcessCallback verifies a provider callback against every enabled merchant
// account of that provider, then applies the idempotent order settlement.
func (s *Service) ProcessCallback(provider string, headers http.Header, body []byte) (*model.PaymentOrder, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if _, ok := supportedProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
	configs, err := s.repo.PaymentProviderConfigs()
	if err != nil {
		return nil, err
	}
	var lastErr error
	for index := range configs {
		config := &configs[index]
		if config.Provider != provider || !config.Enabled {
			continue
		}
		credentials, err := decryptProviderSecrets(s.dataDir, config)
		if err != nil {
			lastErr = err
			continue
		}
		var notification Notification
		switch provider {
		case model.PaymentProviderAlipay:
			notification, err = parseAlipayNotification(config, credentials, body)
		case model.PaymentProviderWeChat:
			notification, err = parseWechatNotification(config, credentials, headers, body)
		}
		if err != nil {
			lastErr = err
			continue
		}
		return s.ApplyNotification(notification)
	}
	if lastErr == nil {
		lastErr = errors.New("no enabled payment provider config")
	}
	return nil, kernel.WrapAppError(http.StatusBadRequest, "支付回调验签失败", lastErr)
}

func (s *Service) providerConfigForOrder(order *model.PaymentOrder) (*model.PaymentProviderConfig, error) {
	if order == nil {
		return nil, kernel.NotFound("支付订单不存在")
	}
	if strings.TrimSpace(order.ProviderConfigID) != "" {
		config, err := s.repo.PaymentProviderConfig(order.ProviderConfigID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, kernel.NewAppError(kernel.CodeUnavailable, "订单绑定的支付渠道不存在或已删除")
			}
			return nil, err
		}
		if config.Provider != order.Provider {
			return nil, kernel.NewAppError(kernel.CodeUnavailable, "订单绑定的支付渠道类型不匹配")
		}
		return config, nil
	}
	configs, err := s.repo.PaymentProviderConfigs()
	if err != nil {
		return nil, err
	}
	for index := range configs {
		if configs[index].Provider == order.Provider && configs[index].Enabled {
			return &configs[index], nil
		}
	}
	return nil, kernel.NewAppError(kernel.CodeUnavailable, "该支付渠道尚未启用，请联系管理员")
}

func (s *Service) firstEnabledProviderConfig(provider string) (*model.PaymentProviderConfig, error) {
	configs, err := s.repo.PaymentProviderConfigs()
	if err != nil {
		return nil, err
	}
	for index := range configs {
		if configs[index].Provider == provider && configs[index].Enabled {
			return &configs[index], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
