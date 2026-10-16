package payment

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TopupProductInput is the admin write payload for a recharge SKU. Price is in
// fen and Credits is the face value; Bonus is the promotional extra.
type TopupProductInput struct {
	Name      string `json:"name"`
	PriceFen  int64  `json:"priceFen"`
	Credits   int64  `json:"credits"`
	Bonus     int64  `json:"bonus"`
	Provider  string `json:"provider"`
	Enabled   *bool  `json:"enabled"`
	SortOrder *int   `json:"sortOrder"`
}

// CreateOrderInput starts one recharge attempt. Provider may be empty, in which
// case the product's default provider is used.
type CreateOrderInput struct {
	ProductID string `json:"productId"`
	Provider  string `json:"provider"`
}

// Notification is one parsed provider callback. The service owns the idempotent
// crediting; the provider adapter owns signature verification and parsing.
type Notification struct {
	Provider               string
	ProviderNotificationID string
	ProviderTradeNo        string
	OrderID                string
	Status                 string
	PayloadJSON            string
}

// RechargeKey is the stable idempotency key for one order's credit movement.
// A repeated callback or a retried admin confirmation converges on this key.
func RechargeKey(orderID string) string { return "topup:" + orderID + ":recharge" }

// ListTopupProducts returns SKUs for the storefront. includeDisabled is used by
// the admin console; ordinary users only ever see enabled SKUs.
func (s *Service) ListTopupProducts(includeDisabled bool) ([]model.TopupProduct, error) {
	items, err := s.repo.TopupProducts()
	if err != nil {
		return nil, err
	}
	SortTopupProducts(items)
	if includeDisabled {
		return items, nil
	}
	enabled := make([]model.TopupProduct, 0, len(items))
	for index := range items {
		if items[index].Enabled {
			enabled = append(enabled, items[index])
		}
	}
	return enabled, nil
}

// CreateTopupProduct adds a recharge SKU.
func (s *Service) CreateTopupProduct(actor *model.User, input TopupProductInput) (*model.TopupProduct, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	item, err := s.buildTopupProduct(input, nil)
	if err != nil {
		return nil, err
	}
	item.ID = uuid.NewString()
	item.CreatedAt = s.now()
	item.UpdatedAt = item.CreatedAt
	if err := s.repo.CreateTopupProduct(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "topup_product.create", item.ID, item.Name); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateTopupProduct edits a SKU. Existing orders keep the snapshot they were
// created with, so changing the price never rewrites history.
func (s *Service) UpdateTopupProduct(actor *model.User, id string, input TopupProductInput) (*model.TopupProduct, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	existing, err := s.loadTopupProduct(id)
	if err != nil {
		return nil, err
	}
	item, err := s.buildTopupProduct(input, existing)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = s.now()
	if err := s.repo.UpdateTopupProduct(item); err != nil {
		return nil, err
	}
	if err := s.audit(actor, "topup_product.update", item.ID, item.Name); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteTopupProduct removes a SKU from the storefront.
func (s *Service) DeleteTopupProduct(actor *model.User, id string) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	existing, err := s.loadTopupProduct(id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteTopupProduct(id); err != nil {
		return err
	}
	return s.audit(actor, "topup_product.delete", id, existing.Name)
}

// CreatePaymentOrder snapshots the product price and total credits so a later
// price change never rewrites an existing order.
func (s *Service) CreatePaymentOrder(userID string, productID string, provider string) (*model.PaymentOrder, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	product, err := s.loadTopupProduct(productID)
	if err != nil {
		return nil, err
	}
	if !product.Enabled {
		return nil, kernel.NewAppError(kernel.CodeConflict, "该充值商品已下架")
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = product.Provider
	}
	if _, ok := supportedProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
	now := s.now()
	order := &model.PaymentOrder{
		ID:                uuid.NewString(),
		UserID:            userID,
		ProductID:         product.ID,
		ProductName:       product.Name,
		Provider:          provider,
		AmountFen:         product.PriceFen,
		Credits:           product.TotalCredits(),
		Status:            model.PaymentOrderPendingPayment,
		CreditIdempotency: RechargeKey(""),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	order.CreditIdempotency = RechargeKey(order.ID)
	if err := s.repo.CreatePaymentOrder(order); err != nil {
		return nil, err
	}
	return order, nil
}

// OrdersByUser returns one user's recharge history, newest first.
func (s *Service) OrdersByUser(userID string, limit int) ([]model.PaymentOrder, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, kernel.Unauthorized("请先登录")
	}
	return s.repo.PaymentOrdersByUser(userID, normalizeOrderLimit(limit))
}

// Orders returns the admin order list, newest first.
func (s *Service) Orders(limit int) ([]model.PaymentOrder, error) {
	return s.repo.PaymentOrders(normalizeOrderLimit(limit))
}

// MarkPaid confirms a payment and credits the buyer exactly once. It is used by
// the admin console (offline payment, manual correction) and by the callback
// path once a notification has been parsed and verified.
func (s *Service) MarkPaid(actor *model.User, orderID string, providerTradeNo string) (*model.PaymentOrder, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	return s.settleOrder(orderID, actor.ID, providerTradeNo)
}

// MarkPaidBySystem is the callback-path variant: no human operator, but the
// same idempotent ledger key and order state machine.
func (s *Service) MarkPaidBySystem(orderID string, providerTradeNo string) (*model.PaymentOrder, error) {
	return s.settleOrder(orderID, "", providerTradeNo)
}

func (s *Service) settleOrder(orderID string, operatorID string, providerTradeNo string) (*model.PaymentOrder, error) {
	order, err := s.loadOrder(orderID)
	if err != nil {
		return nil, err
	}
	switch order.Status {
	case model.PaymentOrderPaid:
		return order, nil
	case model.PaymentOrderClosed:
		return nil, kernel.NewAppError(kernel.CodeConflict, "订单已关闭，不能再确认支付")
	}
	if s.credits == nil {
		return nil, kernel.NewAppError(kernel.CodeUnavailable, "积分服务尚未初始化，暂时无法入账")
	}
	if err := s.credits.Recharge(order.UserID, order.Credits, operatorID,
		fmt.Sprintf("充值：%s", order.ProductName), order.CreditIdempotency); err != nil {
		return nil, err
	}
	now := s.now()
	order.Status = model.PaymentOrderPaid
	order.PaidAt = &now
	order.SettledAt = &now
	order.SettledBy = operatorID
	if strings.TrimSpace(providerTradeNo) != "" {
		order.ProviderTradeNo = strings.TrimSpace(providerTradeNo)
	}
	order.UpdatedAt = now
	if err := s.repo.UpdatePaymentOrder(order); err != nil {
		return nil, err
	}
	if operatorID != "" {
		if err := s.audit(&model.User{ID: operatorID, Role: model.UserRoleAdmin},
			"payment_order.mark_paid", order.ID, order.ProductName); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// ClosePaymentOrder cancels an unpaid order. Crediting has not happened yet, so
// closing only flips the state; repeating it is a no-op.
func (s *Service) ClosePaymentOrder(orderID string, reason string) (*model.PaymentOrder, error) {
	order, err := s.loadOrder(orderID)
	if err != nil {
		return nil, err
	}
	if order.Status == model.PaymentOrderClosed {
		return order, nil
	}
	if order.Status == model.PaymentOrderPaid {
		return nil, kernel.NewAppError(kernel.CodeConflict, "已支付订单不能关闭")
	}
	now := s.now()
	order.Status = model.PaymentOrderClosed
	order.CloseReason = strings.TrimSpace(reason)
	order.UpdatedAt = now
	if err := s.repo.UpdatePaymentOrder(order); err != nil {
		return nil, err
	}
	return order, nil
}

// ApplyNotification records a provider callback and, when it reports success,
// credits the order. The (provider, notification id) pair is unique, so a
// retried callback is a cheap no-op that never moves points twice.
func (s *Service) ApplyNotification(notification Notification) (*model.PaymentOrder, error) {
	provider := strings.ToLower(strings.TrimSpace(notification.Provider))
	if _, ok := supportedProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
	notificationID := strings.TrimSpace(notification.ProviderNotificationID)
	if notificationID == "" {
		return nil, kernel.BadAuthRequest("支付回调缺少通知编号")
	}
	orderID := strings.TrimSpace(notification.OrderID)
	if orderID == "" {
		return nil, kernel.BadAuthRequest("支付回调缺少订单号")
	}

	record := &model.PaymentNotification{
		ID:                     uuid.NewString(),
		Provider:               provider,
		ProviderNotificationID: notificationID,
		ProviderTradeNo:        strings.TrimSpace(notification.ProviderTradeNo),
		OrderID:                orderID,
		Status:                 strings.TrimSpace(notification.Status),
		PayloadJSON:            notification.PayloadJSON,
		CreatedAt:              s.now(),
	}
	created, err := s.repo.CreatePaymentNotification(record)
	if err != nil {
		return nil, err
	}
	if !created {
		// 重复回调：直接返回订单，保证网关重推时不会二次入账。
		return s.loadOrder(orderID)
	}
	if notification.Status != model.PaymentOrderPaid {
		// 非成功回调（例如交易关闭）只留痕，不入账。
		return s.loadOrder(orderID)
	}
	order, settleErr := s.MarkPaidBySystem(orderID, notification.ProviderTradeNo)
	record.Processed = settleErr == nil
	if settleErr != nil {
		record.Error = settleErr.Error()
	}
	if err := s.repo.UpdatePaymentNotification(record); err != nil {
		return nil, err
	}
	return order, settleErr
}

func (s *Service) loadTopupProduct(id string) (*model.TopupProduct, error) {
	item, err := s.repo.TopupProduct(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, kernel.NotFound("充值商品不存在")
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) loadOrder(id string) (*model.PaymentOrder, error) {
	item, err := s.repo.PaymentOrder(strings.TrimSpace(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, kernel.NotFound("支付订单不存在")
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// buildTopupProduct validates the admin payload. existing is nil on create.
func (s *Service) buildTopupProduct(input TopupProductInput, existing *model.TopupProduct) (*model.TopupProduct, error) {
	name := strings.TrimSpace(input.Name)
	if existing != nil && name == "" {
		name = existing.Name
	}
	if name == "" {
		return nil, kernel.BadAuthRequest("请填写商品名称")
	}
	if input.PriceFen <= 0 {
		return nil, kernel.BadAuthRequest("商品价格必须大于零")
	}
	if input.Credits <= 0 {
		return nil, kernel.BadAuthRequest("商品积分必须大于零")
	}
	if input.Bonus < 0 {
		return nil, kernel.BadAuthRequest("赠送积分不能为负数")
	}
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if existing != nil && provider == "" {
		provider = existing.Provider
	}
	if _, ok := supportedProviders[provider]; !ok {
		return nil, kernel.BadAuthRequest(ErrUnsupportedProvider.Error())
	}
	enabled := true
	if existing != nil {
		enabled = existing.Enabled
	}
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
	return &model.TopupProduct{
		Name: name, PriceFen: input.PriceFen, Credits: input.Credits, Bonus: input.Bonus,
		Provider: provider, Enabled: enabled, SortOrder: sortOrder,
	}, nil
}

func normalizeOrderLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

// SortTopupProducts orders the storefront by sort order then price, so the admin
// console and the user page agree without relying on database ordering.
func SortTopupProducts(items []model.TopupProduct) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].PriceFen < items[j].PriceFen
	})
}
