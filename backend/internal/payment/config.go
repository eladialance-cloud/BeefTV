// Package payment owns hosted payment-channel configuration and (later) the
// order/callback lifecycle. It only ships in the server profile: cmd/desktop
// must never link it, which TestDesktopDependencyBoundary enforces.
package payment

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/localcrypto"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/outbound"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository is the durable port for payment-channel configuration.
type Repository interface {
	PaymentProviderConfigs() ([]model.PaymentProviderConfig, error)
	PaymentProviderConfig(id string) (*model.PaymentProviderConfig, error)
	CreatePaymentProviderConfig(*model.PaymentProviderConfig) error
	UpdatePaymentProviderConfig(*model.PaymentProviderConfig) error
	DeletePaymentProviderConfig(id string) error
	TopupProducts() ([]model.TopupProduct, error)
	TopupProduct(id string) (*model.TopupProduct, error)
	CreateTopupProduct(*model.TopupProduct) error
	UpdateTopupProduct(*model.TopupProduct) error
	DeleteTopupProduct(id string) error
	PaymentOrders(limit int) ([]model.PaymentOrder, error)
	PaymentOrdersByUser(userID string, limit int) ([]model.PaymentOrder, error)
	PaymentOrder(id string) (*model.PaymentOrder, error)
	CreatePaymentOrder(*model.PaymentOrder) error
	UpdatePaymentOrder(*model.PaymentOrder) error
	ExpirePendingPaymentOrders(cutoff time.Time, reason string, limit int, now time.Time) (int64, error)
	PaymentOrderCountsByStatus() (map[string]int64, error)
	PaymentOrderSettledTotals() (amountFen int64, credits int64, err error)
	PaymentOrderCountsNeedingAttention(cutoff time.Time) (paidUnsettled int64, pendingOverdue int64, err error)
	PaymentNotificationCounts() (unprocessed int64, failed int64, err error)
	CreatePaymentNotification(*model.PaymentNotification) (bool, error)
	UpdatePaymentNotification(*model.PaymentNotification) error
	AppendAdminAudit(*model.AdminAuditEvent) error
}

// Service is the admin-facing owner of payment-channel configuration.
type Service struct {
	repo                 Repository
	credits              CreditRecharger
	dataDir              string
	now                  func() time.Time
	httpClient           *http.Client
	alipayEndpoint       string
	wechatNativeEndpoint string
	adjuster             CreditAdjuster
}

// CreditRecharger is the slice of the credit service the payment flow needs:
// an idempotent top-up. Keeping it narrow lets tests use a stub while
// production injects the real credits.Service.
type CreditRecharger interface {
	Recharge(userID string, amount int64, operatorID string, reason string, idempotencyKey string) error
}

// CreditAdjuster is the signed ledger write used by refunds. The production
// adapter implements both interfaces, while tests can provide only the slice
// they need.
type CreditAdjuster interface {
	Adjust(userID string, delta int64, operatorID string, reason string, idempotencyKey string) error
}

// New wires payment configuration and the recharge flow. credits may be nil
// when only provider configuration is used; marking an order paid then fails
// closed instead of silently dropping the points.
func New(repo Repository, credits CreditRecharger, dataDir string) *Service {
	service := &Service{repo: repo, credits: credits, dataDir: dataDir, now: func() time.Time { return time.Now().UTC() }}
	if adjuster, ok := credits.(CreditAdjuster); ok {
		service.adjuster = adjuster
	}
	service.httpClient = outbound.OutboundHTTPClient(15 * time.Second)
	service.alipayEndpoint = defaultAlipayEndpoint
	service.wechatNativeEndpoint = defaultWechatNativeEndpoint
	return service
}

// Input is the admin write payload. Secrets are write-only: leaving them empty
// on update keeps whatever is stored.
type Input struct {
	Provider          string `json:"provider"`
	Name              string `json:"name"`
	AppID             string `json:"appId"`
	MchID             string `json:"mchId"`
	MchSerialNo       string `json:"mchSerialNo"`
	NotifyURL         string `json:"notifyUrl"`
	ReturnURL         string `json:"returnUrl"`
	Endpoint          string `json:"endpoint"`
	PayType           string `json:"payType"`
	APIKey            string `json:"apiKey"`
	PrivateKey        string `json:"privateKey"`
	PlatformPublicKey string `json:"platformPublicKey"`
	Enabled           *bool  `json:"enabled"`
	SortOrder         *int   `json:"sortOrder"`
}

// View is the safe projection returned to the admin console: no plaintext
// secret, only whether one is configured.
type View struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Name          string     `json:"name"`
	AppID         string     `json:"appId"`
	MchID         string     `json:"mchId"`
	MchSerialNo   string     `json:"mchSerialNo"`
	NotifyURL     string     `json:"notifyUrl"`
	ReturnURL     string     `json:"returnUrl"`
	Endpoint      string     `json:"endpoint"`
	PayType       string     `json:"payType"`
	SecretMasked  string     `json:"secretMasked"`
	SecretSet     bool       `json:"secretSet"`
	Enabled       bool       `json:"enabled"`
	SortOrder     int        `json:"sortOrder"`
	LastTestedAt  *time.Time `json:"lastTestedAt"`
	LastTestError string     `json:"lastTestError"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// secrets is the encrypted credential bundle. Adding a gateway-specific field
// here is a schema-free change.
type secrets struct {
	APIKey            string `json:"apiKey"`
	PrivateKey        string `json:"privateKey"`
	PlatformPublicKey string `json:"platformPublicKey"`
}

const secretMask = "********"

var supportedProviders = map[string]struct{}{
	model.PaymentProviderAlipay: {},
	model.PaymentProviderWeChat: {},
	model.PaymentProviderEPay:   {},
}

// ErrUnsupportedProvider is returned when an admin names a gateway family the
// code has no adapter for.
var ErrUnsupportedProvider = errors.New("暂不支持该支付渠道类型")

// List 返回全部支付渠道的安全视图。
func (s *Service) List() ([]View, error) {
	items, err := s.repo.PaymentProviderConfigs()
	if err != nil {
		return nil, err
	}
	views := make([]View, 0, len(items))
	for index := range items {
		views = append(views, s.toView(&items[index]))
	}
	return views, nil
}

func (s *Service) Create(actor *model.User, input Input) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	item, err := s.build(actor, input, nil)
	if err != nil {
		return nil, err
	}
	item.ID = uuid.NewString()
	item.CreatedAt = s.now()
	item.UpdatedAt = item.CreatedAt
	item.CreatedBy = actor.ID
	if err := s.repo.CreatePaymentProviderConfig(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "payment_provider.create", item.ID, item.Name); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

func (s *Service) Update(actor *model.User, id string, input Input) (*View, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	existing, err := s.load(id)
	if err != nil {
		return nil, err
	}
	item, err := s.build(actor, input, existing)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	item.CreatedBy = existing.CreatedBy
	item.UpdatedAt = s.now()
	if err := s.repo.UpdatePaymentProviderConfig(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "payment_provider.update", item.ID, item.Name); err != nil {
		return nil, err
	}
	view := s.toView(item)
	return &view, nil
}

func (s *Service) Delete(actor *model.User, id string) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	existing, err := s.load(id)
	if err != nil {
		return err
	}
	if err := s.repo.DeletePaymentProviderConfig(id); err != nil {
		return err
	}
	return s.audit(actor, "payment_provider.delete", id, existing.Name)
}

func (s *Service) build(actor *model.User, input Input, existing *model.PaymentProviderConfig) (*model.PaymentProviderConfig, error) {
	provider := strings.TrimSpace(input.Provider)
	if existing != nil && provider == "" {
		provider = existing.Provider
	}
	if _, ok := supportedProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
	name := strings.TrimSpace(input.Name)
	if existing != nil && name == "" {
		name = existing.Name
	}
	if name == "" {
		return nil, kernel.BadAuthRequest("请填写渠道名称")
	}
	appID := pick(input.AppID, existing, func(item *model.PaymentProviderConfig) string { return item.AppID })
	mchID := pick(input.MchID, existing, func(item *model.PaymentProviderConfig) string { return item.MchID })
	mchSerialNo := pick(input.MchSerialNo, existing, func(item *model.PaymentProviderConfig) string { return item.MchSerialNo })
	notifyURL := pick(input.NotifyURL, existing, func(item *model.PaymentProviderConfig) string { return item.NotifyURL })
	returnURL := pick(input.ReturnURL, existing, func(item *model.PaymentProviderConfig) string { return item.ReturnURL })
	endpoint := pick(input.Endpoint, existing, func(item *model.PaymentProviderConfig) string { return item.Endpoint })
	payType := strings.ToLower(pick(input.PayType, existing, func(item *model.PaymentProviderConfig) string { return item.PayType }))
	if provider == model.PaymentProviderEPay {
		if endpoint == "" {
			return nil, kernel.BadAuthRequest("请填写易支付网关地址")
		}
		if appID == "" {
			return nil, kernel.BadAuthRequest("请填写易支付商户 PID")
		}
		if payType == "" {
			payType = "alipay"
		}
		switch payType {
		case "alipay", "wxpay", "qqpay":
		default:
			return nil, kernel.BadAuthRequest("易支付付款方式仅支持 alipay、wxpay、qqpay")
		}
	}

	stored := secrets{}
	if existing != nil && strings.TrimSpace(existing.SecretsEncrypted) != "" {
		plain, err := localcrypto.Decrypt(s.dataDir, existing.SecretsEncrypted)
		if err != nil {
			return nil, kernel.WrapAppError(kernel.CodeInternal, "支付渠道密钥解密失败，请重新填写", err)
		}
		if err := decodeSecrets(plain, &stored); err != nil {
			return nil, kernel.WrapAppError(kernel.CodeInternal, "支付渠道密钥格式损坏，请重新填写", err)
		}
	}
	if value := strings.TrimSpace(input.APIKey); value != "" {
		stored.APIKey = value
	}
	if value := strings.TrimSpace(input.PrivateKey); value != "" {
		stored.PrivateKey = value
	}
	if value := strings.TrimSpace(input.PlatformPublicKey); value != "" {
		stored.PlatformPublicKey = value
	}
	if existing == nil && stored.APIKey == "" && stored.PrivateKey == "" && stored.PlatformPublicKey == "" {
		return nil, kernel.BadAuthRequest("请至少填写一项渠道密钥")
	}
	encrypted, err := encryptSecrets(s.dataDir, stored)
	if err != nil {
		return nil, err
	}

	enabled := existing != nil && existing.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	sortOrder := 0
	if existing != nil {
		sortOrder = existing.SortOrder
	}
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	lastTestedAt, lastTestError := (*time.Time)(nil), ""
	if existing != nil {
		lastTestedAt, lastTestError = existing.LastTestedAt, existing.LastTestError
	}
	return &model.PaymentProviderConfig{
		Provider:         provider,
		Name:             name,
		AppID:            appID,
		MchID:            mchID,
		MchSerialNo:      mchSerialNo,
		NotifyURL:        notifyURL,
		ReturnURL:        returnURL,
		Endpoint:         endpoint,
		PayType:          payType,
		SecretsEncrypted: encrypted,
		Enabled:          enabled,
		SortOrder:        sortOrder,
		LastTestedAt:     lastTestedAt,
		LastTestError:    lastTestError,
	}, nil
}

func (s *Service) toView(item *model.PaymentProviderConfig) View {
	configured := strings.TrimSpace(item.SecretsEncrypted) != ""
	masked := ""
	if configured {
		masked = secretMask
	}
	return View{
		ID:            item.ID,
		Provider:      item.Provider,
		Name:          item.Name,
		AppID:         item.AppID,
		MchID:         item.MchID,
		MchSerialNo:   item.MchSerialNo,
		NotifyURL:     item.NotifyURL,
		ReturnURL:     item.ReturnURL,
		Endpoint:      item.Endpoint,
		PayType:       item.PayType,
		SecretMasked:  masked,
		SecretSet:     configured,
		Enabled:       item.Enabled,
		SortOrder:     item.SortOrder,
		LastTestedAt:  item.LastTestedAt,
		LastTestError: item.LastTestError,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func (s *Service) audit(actor *model.User, action string, targetID string, detail string) error {
	return s.repo.AppendAdminAudit(&model.AdminAuditEvent{
		ID:          uuid.NewString(),
		ActorUserID: actor.ID,
		Action:      action,
		TargetType:  "payment_provider_config",
		TargetID:    targetID,
		Summary:     detail,
		CreatedAt:   s.now(),
	})
}

// load 统一把仓储层的「查不到」翻译成 404，避免把 gorm 细节泄漏到 HTTP 层。
func (s *Service) load(id string) (*model.PaymentProviderConfig, error) {
	item, err := s.repo.PaymentProviderConfig(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, kernel.NotFound("支付渠道不存在")
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func pick(value string, existing *model.PaymentProviderConfig, current func(*model.PaymentProviderConfig) string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	if existing != nil {
		return current(existing)
	}
	return ""
}

func requireActor(actor *model.User) error {
	if actor == nil || strings.TrimSpace(actor.ID) == "" {
		return kernel.Unauthorized("请先登录")
	}
	if actor.Role != model.UserRoleAdmin {
		return kernel.Forbidden("需要管理员权限")
	}
	return nil
}
