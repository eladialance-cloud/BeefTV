package repository

import (
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newPaymentOrderRepo(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:payment-orders-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.PaymentOrder{}, &model.PaymentNotification{}); err != nil {
		t.Fatal(err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return New(db), db
}

func paymentTime(offset time.Duration) *time.Time {
	value := time.Unix(1_700_000_000, 0).UTC().Add(offset)
	return &value
}

func seedPaymentOrder(t *testing.T, db *gorm.DB, order model.PaymentOrder) {
	t.Helper()
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("写入订单 %s 失败: %v", order.ID, err)
	}
}

func TestExpirePendingPaymentOrdersClosesOnlyOverduePending(t *testing.T) {
	repo, db := newPaymentOrderRepo(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	for _, order := range []model.PaymentOrder{
		{ID: "o-overdue", UserID: "u-1", Status: model.PaymentOrderPendingPayment, CreatedAt: now.Add(-time.Hour), ExpiresAt: paymentTime(-time.Minute)},
		{ID: "o-live", UserID: "u-1", Status: model.PaymentOrderPendingPayment, CreatedAt: now, ExpiresAt: paymentTime(time.Minute)},
		{ID: "o-legacy-old", UserID: "u-1", Status: model.PaymentOrderPendingPayment, CreatedAt: now.Add(-time.Hour)},
		{ID: "o-legacy-new", UserID: "u-1", Status: model.PaymentOrderPendingPayment, CreatedAt: now.Add(time.Minute)},
		{ID: "o-paid-overdue", UserID: "u-1", Status: model.PaymentOrderPaid, CreatedAt: now.Add(-time.Hour), ExpiresAt: paymentTime(-time.Minute)},
	} {
		seedPaymentOrder(t, db, order)
	}

	closed, err := repo.ExpirePendingPaymentOrders(now, "超时未支付关闭", 0, now)
	if err != nil {
		t.Fatalf("清理超时订单失败: %v", err)
	}
	if closed != 2 {
		t.Fatalf("只应关闭两个超时待支付订单，得到 %d", closed)
	}

	for id, want := range map[string]string{
		"o-overdue":      model.PaymentOrderClosed,
		"o-live":         model.PaymentOrderPendingPayment,
		"o-legacy-old":   model.PaymentOrderClosed,
		"o-legacy-new":   model.PaymentOrderPendingPayment,
		"o-paid-overdue": model.PaymentOrderPaid,
	} {
		stored, err := repo.PaymentOrder(id)
		if err != nil {
			t.Fatalf("读取订单 %s 失败: %v", id, err)
		}
		if stored.Status != want {
			t.Fatalf("订单 %s 状态应为 %q，得到 %q", id, want, stored.Status)
		}
	}

	closedOrder, _ := repo.PaymentOrder("o-overdue")
	if closedOrder.CloseReason != "超时未支付关闭" {
		t.Fatalf("关闭原因必须写入，得到 %q", closedOrder.CloseReason)
	}
}

func TestExpirePendingPaymentOrdersHonorsLimit(t *testing.T) {
	repo, db := newPaymentOrderRepo(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	for _, id := range []string{"o-1", "o-2", "o-3"} {
		seedPaymentOrder(t, db, model.PaymentOrder{
			ID: id, UserID: "u-1", Status: model.PaymentOrderPendingPayment,
			CreatedAt: now.Add(-time.Hour), ExpiresAt: paymentTime(-time.Minute),
		})
	}

	closed, err := repo.ExpirePendingPaymentOrders(now, "超时", 2, now)
	if err != nil {
		t.Fatalf("清理超时订单失败: %v", err)
	}
	if closed != 2 {
		t.Fatalf("批量上限为 2 时应只关闭两条，得到 %d", closed)
	}
}

func TestPaymentOrderReconciliationTotals(t *testing.T) {
	repo, db := newPaymentOrderRepo(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	settledAt := paymentTime(-time.Hour)
	for _, order := range []model.PaymentOrder{
		{ID: "o-paid", UserID: "u-1", AmountFen: 990, Credits: 1100, Status: model.PaymentOrderPaid, CreatedAt: now, SettledAt: settledAt},
		{ID: "o-paid-unsettled", UserID: "u-1", AmountFen: 100, Credits: 100, Status: model.PaymentOrderPaid, CreatedAt: now},
		{ID: "o-refunded", UserID: "u-1", AmountFen: 200, Credits: 200, Status: model.PaymentOrderRefunded, CreatedAt: now},
		{ID: "o-overdue", UserID: "u-1", AmountFen: 1, Credits: 1, Status: model.PaymentOrderPendingPayment, CreatedAt: now, ExpiresAt: paymentTime(-time.Minute)},
		{ID: "o-pending", UserID: "u-1", AmountFen: 1, Credits: 1, Status: model.PaymentOrderPendingPayment, CreatedAt: now, ExpiresAt: paymentTime(time.Minute)},
	} {
		seedPaymentOrder(t, db, order)
	}
	for _, notification := range []model.PaymentNotification{
		{ID: "n-1", Provider: model.PaymentProviderAlipay, ProviderNotificationID: "n-1", Processed: false},
		{ID: "n-2", Provider: model.PaymentProviderAlipay, ProviderNotificationID: "n-2", Processed: false, Error: "bad signature"},
		{ID: "n-3", Provider: model.PaymentProviderWeChat, ProviderNotificationID: "n-3", Processed: true},
	} {
		if err := db.Create(&notification).Error; err != nil {
			t.Fatal(err)
		}
	}

	counts, err := repo.PaymentOrderCountsByStatus()
	if err != nil {
		t.Fatalf("状态计数失败: %v", err)
	}
	if counts[model.PaymentOrderPaid] != 2 || counts[model.PaymentOrderRefunded] != 1 || counts[model.PaymentOrderPendingPayment] != 2 {
		t.Fatalf("状态计数不对: %+v", counts)
	}

	amountFen, credits, err := repo.PaymentOrderSettledTotals()
	if err != nil {
		t.Fatalf("结算合计失败: %v", err)
	}
	if amountFen != 1090 || credits != 1200 {
		t.Fatalf("已结算金额 / 积分不对: %d / %d", amountFen, credits)
	}

	paidUnsettled, pendingOverdue, err := repo.PaymentOrderCountsNeedingAttention(now)
	if err != nil {
		t.Fatalf("待处理计数失败: %v", err)
	}
	if paidUnsettled != 1 || pendingOverdue != 1 {
		t.Fatalf("待处理计数不对: paid=%d overdue=%d", paidUnsettled, pendingOverdue)
	}

	unprocessed, failed, err := repo.PaymentNotificationCounts()
	if err != nil {
		t.Fatalf("回调计数失败: %v", err)
	}
	if unprocessed != 2 || failed != 1 {
		t.Fatalf("回调计数不对: unprocessed=%d failed=%d", unprocessed, failed)
	}
}

func TestUpdatePaymentOrderPersistsRefundFields(t *testing.T) {
	repo, db := newPaymentOrderRepo(t)
	paidAt := paymentTime(-time.Hour)
	refundedAt := paymentTime(-time.Minute)
	order := model.PaymentOrder{
		ID: "o-1", UserID: "u-1", ProductID: "p-1", ProductName: "1000 积分",
		AmountFen: 990, Credits: 1100, Status: model.PaymentOrderPaid,
		ProviderTradeNo: "trade-1", CreatedAt: *paymentTime(-time.Hour), PaidAt: paidAt,
		SettledAt: paidAt, SettledBy: "admin-1", CreditIdempotency: "topup:o-1:recharge",
	}
	seedPaymentOrder(t, db, order)

	order.Status = model.PaymentOrderRefunded
	order.RefundedAt = refundedAt
	order.RefundedBy = "admin-2"
	order.RefundReason = "用户申请"
	order.RefundProviderNo = "refund-1"
	order.RefundReversed = true
	if err := repo.UpdatePaymentOrder(&order); err != nil {
		t.Fatalf("退款订单更新失败: %v", err)
	}

	stored, err := repo.PaymentOrder("o-1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.PaymentOrderRefunded || !stored.RefundReversed {
		t.Fatalf("退款状态应落库，得到 %+v", stored)
	}
	if stored.RefundedBy != "admin-2" || stored.RefundReason != "用户申请" || stored.RefundProviderNo != "refund-1" {
		t.Fatalf("退款人 / 原因 / 退款单号应落库，得到 %+v", stored)
	}
	if stored.RefundedAt == nil || !stored.RefundedAt.Equal(refundedAt.UTC()) {
		t.Fatalf("退款时间应落库，得到 %v", stored.RefundedAt)
	}
	// 退款更新不应抹掉下单时快照的金额与幂等键。
	if stored.AmountFen != 990 || stored.Credits != 1100 || stored.CreditIdempotency != "topup:o-1:recharge" {
		t.Fatalf("退款更新不得改写快照与幂等键，得到 %+v", stored)
	}
}
