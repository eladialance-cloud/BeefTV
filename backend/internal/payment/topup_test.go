package payment

import (
	"errors"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type topupRepo struct {
	products      []model.TopupProduct
	orders        []model.PaymentOrder
	notifications []model.PaymentNotification
	audits        []model.AdminAuditEvent
	configs       []model.PaymentProviderConfig
}

func (f *topupRepo) PaymentProviderConfigs() ([]model.PaymentProviderConfig, error) {
	return append([]model.PaymentProviderConfig(nil), f.configs...), nil
}

func (f *topupRepo) PaymentProviderConfig(id string) (*model.PaymentProviderConfig, error) {
	for index := range f.configs {
		if f.configs[index].ID == id {
			item := f.configs[index]
			return &item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *topupRepo) CreatePaymentProviderConfig(item *model.PaymentProviderConfig) error { return nil }

func (f *topupRepo) UpdatePaymentProviderConfig(item *model.PaymentProviderConfig) error {
	return gorm.ErrRecordNotFound
}

func (f *topupRepo) DeletePaymentProviderConfig(id string) error { return gorm.ErrRecordNotFound }

func (f *topupRepo) TopupProducts() ([]model.TopupProduct, error) {
	return append([]model.TopupProduct(nil), f.products...), nil
}

func (f *topupRepo) TopupProduct(id string) (*model.TopupProduct, error) {
	for index := range f.products {
		if f.products[index].ID == id {
			item := f.products[index]
			return &item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *topupRepo) CreateTopupProduct(item *model.TopupProduct) error {
	f.products = append(f.products, *item)
	return nil
}

func (f *topupRepo) UpdateTopupProduct(item *model.TopupProduct) error {
	for index := range f.products {
		if f.products[index].ID == item.ID {
			f.products[index] = *item
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *topupRepo) DeleteTopupProduct(id string) error {
	for index := range f.products {
		if f.products[index].ID == id {
			f.products = append(f.products[:index], f.products[index+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *topupRepo) PaymentOrders(limit int) ([]model.PaymentOrder, error) {
	if limit > 0 && len(f.orders) > limit {
		return append([]model.PaymentOrder(nil), f.orders[:limit]...), nil
	}
	return append([]model.PaymentOrder(nil), f.orders...), nil
}

func (f *topupRepo) PaymentOrdersByUser(userID string, limit int) ([]model.PaymentOrder, error) {
	var out []model.PaymentOrder
	for index := range f.orders {
		if f.orders[index].UserID == userID {
			out = append(out, f.orders[index])
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *topupRepo) PaymentOrder(id string) (*model.PaymentOrder, error) {
	for index := range f.orders {
		if f.orders[index].ID == id {
			item := f.orders[index]
			return &item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *topupRepo) CreatePaymentOrder(item *model.PaymentOrder) error {
	f.orders = append(f.orders, *item)
	return nil
}

func (f *topupRepo) UpdatePaymentOrder(item *model.PaymentOrder) error {
	for index := range f.orders {
		if f.orders[index].ID == item.ID {
			f.orders[index] = *item
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *topupRepo) CreatePaymentNotification(item *model.PaymentNotification) (bool, error) {
	for index := range f.notifications {
		if f.notifications[index].Provider == item.Provider &&
			f.notifications[index].ProviderNotificationID == item.ProviderNotificationID {
			return false, nil
		}
	}
	f.notifications = append(f.notifications, *item)
	return true, nil
}

func (f *topupRepo) UpdatePaymentNotification(item *model.PaymentNotification) error {
	for index := range f.notifications {
		if f.notifications[index].ID == item.ID {
			f.notifications[index] = *item
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *topupRepo) AppendAdminAudit(event *model.AdminAuditEvent) error {
	f.audits = append(f.audits, *event)
	return nil
}

type fakeCreditCreditor struct {
	calls []creditGrant
	err   error
}

type creditGrant struct {
	userID         string
	amount         int64
	operatorID     string
	reason         string
	idempotencyKey string
}

func (f *fakeCreditCreditor) Recharge(userID string, amount int64, operatorID string, reason string, idempotencyKey string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, creditGrant{userID, amount, operatorID, reason, idempotencyKey})
	return nil
}

func newTopupService(t *testing.T) (*Service, *topupRepo, *fakeCreditCreditor) {
	t.Helper()
	repo := &topupRepo{}
	credits := &fakeCreditCreditor{}
	service := New(repo, nil, t.TempDir())
	product := model.TopupProduct{
		ID: "p-1", Name: "1000 积分", PriceFen: 990, Credits: 1000, Bonus: 100,
		Provider: model.PaymentProviderAlipay, Enabled: true, SortOrder: 1,
	}
	repo.products = append(repo.products, product)
	service = New(repo, credits, t.TempDir())
	return service, repo, credits
}

func TestTopupListHidesDisabledForUsers(t *testing.T) {
	service, repo, _ := newTopupService(t)
	repo.products = append(repo.products, model.TopupProduct{ID: "p-2", Name: "停用", Enabled: false})

	userItems, err := service.ListTopupProducts(false)
	if err != nil {
		t.Fatalf("普通用户列表失败: %v", err)
	}
	if len(userItems) != 1 || userItems[0].ID != "p-1" {
		t.Fatalf("普通用户只能看到启用的商品，得到 %+v", userItems)
	}
	adminItems, err := service.ListTopupProducts(true)
	if err != nil {
		t.Fatalf("管理员列表失败: %v", err)
	}
	if len(adminItems) != 2 {
		t.Fatalf("管理员应看到全部商品，得到 %+v", adminItems)
	}
}

func TestCreatePaymentOrderSnapshotsProduct(t *testing.T) {
	service, repo, _ := newTopupService(t)
	order, err := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderWeChat)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if order.Status != model.PaymentOrderPendingPayment {
		t.Fatalf("新订单应待支付，得到 %q", order.Status)
	}
	if order.AmountFen != 990 || order.Credits != 1100 {
		t.Fatalf("订单必须快照价格与总积分，得到 %+v", order)
	}
	if order.Provider != model.PaymentProviderWeChat {
		t.Fatalf("订单渠道应为请求渠道，得到 %q", order.Provider)
	}
	if order.CreditIdempotency != "topup:"+order.ID+":recharge" {
		t.Fatalf("订单必须带稳定的入账幂等键，得到 %q", order.CreditIdempotency)
	}
	if len(repo.orders) != 1 {
		t.Fatalf("订单必须落库，得到 %d", len(repo.orders))
	}
	// 改价不应影响已下单的订单。
	repo.products[0].Credits = 999999
	stored, _ := repo.PaymentOrder(order.ID)
	if stored.Credits != 1100 {
		t.Fatalf("改价后历史订单积分必须不变，得到 %d", stored.Credits)
	}
}

func TestCreatePaymentOrderRejectsDisabledProduct(t *testing.T) {
	service, repo, _ := newTopupService(t)
	repo.products[0].Enabled = false
	if _, err := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay); err == nil {
		t.Fatal("停用商品不得下单")
	}
}

func TestCreatePaymentOrderRejectsUnknownProvider(t *testing.T) {
	service, _, _ := newTopupService(t)
	if _, err := service.CreatePaymentOrder("u-1", "p-1", "paypal"); err == nil {
		t.Fatal("未知支付渠道不得下单")
	}
}

func TestMarkPaidCreditsOnce(t *testing.T) {
	service, repo, credits := newTopupService(t)
	order, err := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	paid, err := service.MarkPaid(adminActor(), order.ID, "manual-1")
	if err != nil {
		t.Fatalf("确认支付失败: %v", err)
	}
	if paid.Status != model.PaymentOrderPaid || paid.SettledAt == nil {
		t.Fatalf("确认后订单应为已支付并带结算时间，得到 %+v", paid)
	}
	if len(credits.calls) != 1 {
		t.Fatalf("应入账一次，得到 %d", len(credits.calls))
	}
	grant := credits.calls[0]
	if grant.userID != "u-1" || grant.amount != 1100 || grant.idempotencyKey != "topup:"+order.ID+":recharge" {
		t.Fatalf("入账参数不对: %+v", grant)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "payment_order.mark_paid" {
		t.Fatalf("确认支付必须写审计，得到 %+v", repo.audits)
	}

	// 重复确认：订单已支付，不再重复入账，也不报错。
	again, err := service.MarkPaid(adminActor(), order.ID, "manual-2")
	if err != nil {
		t.Fatalf("重复确认应幂等成功: %v", err)
	}
	if again.Status != model.PaymentOrderPaid {
		t.Fatalf("重复确认后仍是已支付，得到 %q", again.Status)
	}
	if len(credits.calls) != 1 {
		t.Fatalf("重复确认不得再次入账，得到 %d", len(credits.calls))
	}
}

func TestMarkPaidRejectsNonAdmin(t *testing.T) {
	service, _, _ := newTopupService(t)
	order, _ := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	_, err := service.MarkPaid(&model.User{ID: "u-1", Role: model.UserRoleUser}, order.ID, "x")
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != 403 {
		t.Fatalf("普通用户不得确认支付，得到 %v", err)
	}
}

func TestMarkPaidRejectsClosedOrder(t *testing.T) {
	service, repo, _ := newTopupService(t)
	order, _ := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	repo.orders[0].Status = model.PaymentOrderClosed
	if _, err := service.MarkPaid(adminActor(), order.ID, "x"); err == nil {
		t.Fatal("已关闭订单不得再确认支付")
	}
}

func TestClosePaymentOrderReleasesPending(t *testing.T) {
	service, _, _ := newTopupService(t)
	order, _ := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	closed, err := service.ClosePaymentOrder(order.ID, "超时未支付")
	if err != nil {
		t.Fatalf("关闭订单失败: %v", err)
	}
	if closed.Status != model.PaymentOrderClosed || strings.TrimSpace(closed.CloseReason) == "" {
		t.Fatalf("关闭后状态与原因都应写入，得到 %+v", closed)
	}
	if _, err := service.ClosePaymentOrder(order.ID, "再关一次"); err != nil {
		t.Fatalf("重复关闭应幂等: %v", err)
	}
}

func TestApplyNotificationCreditsOnce(t *testing.T) {
	service, repo, credits := newTopupService(t)
	order, _ := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)

	notification := Notification{
		Provider:               model.PaymentProviderAlipay,
		ProviderNotificationID: "notify-1",
		ProviderTradeNo:        "trade-1",
		OrderID:                order.ID,
		Status:                 model.PaymentOrderPaid,
		AmountFen:              990,
		PayloadJSON:            `{"trade_status":"TRADE_SUCCESS"}`,
	}
	settled, err := service.ApplyNotification(notification)
	if err != nil {
		t.Fatalf("处理回调失败: %v", err)
	}
	if settled.Status != model.PaymentOrderPaid {
		t.Fatalf("回调后订单应为已支付，得到 %q", settled.Status)
	}
	if len(credits.calls) != 1 {
		t.Fatalf("回调应入账一次，得到 %d", len(credits.calls))
	}
	if len(repo.notifications) != 1 || !repo.notifications[0].Processed {
		t.Fatalf("回调必须留痕并标记已处理，得到 %+v", repo.notifications)
	}

	// 同一个回调再来一次：幂等，不重复入账。
	if _, err := service.ApplyNotification(notification); err != nil {
		t.Fatalf("重复回调不应报错: %v", err)
	}
	if len(credits.calls) != 1 {
		t.Fatalf("重复回调不得重复入账，得到 %d", len(credits.calls))
	}
	if len(repo.notifications) != 1 {
		t.Fatalf("重复回调不得新增记录，得到 %d", len(repo.notifications))
	}
}

func TestApplyNotificationRejectsUnknownOrder(t *testing.T) {
	service, _, _ := newTopupService(t)
	_, err := service.ApplyNotification(Notification{
		Provider:               model.PaymentProviderAlipay,
		ProviderNotificationID: "notify-x",
		OrderID:                "missing",
		Status:                 model.PaymentOrderPaid,
	})
	var appErr *kernel.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Status != 404 {
		t.Fatalf("未知订单回调应 404，得到 %v", err)
	}
}

func TestCreditFailuresAreRecorded(t *testing.T) {
	service, repo, credits := newTopupService(t)
	order, _ := service.CreatePaymentOrder("u-1", "p-1", model.PaymentProviderAlipay)
	credits.err = errors.New("ledger down")
	if _, err := service.MarkPaid(adminActor(), order.ID, "manual"); err == nil {
		t.Fatal("入账失败必须向上报错")
	}
	stored, _ := repo.PaymentOrder(order.ID)
	if stored.Status != model.PaymentOrderPendingPayment {
		t.Fatalf("入账失败时订单不得标记已支付，得到 %q", stored.Status)
	}
}
