package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TopupProducts 返回全部充值商品，按管理员设置的顺序展示。
func (r *Repository) TopupProducts() ([]model.TopupProduct, error) {
	var items []model.TopupProduct
	err := r.db.Order("sort_order asc, price_fen asc, created_at asc").Find(&items).Error
	return items, err
}

func (r *Repository) TopupProduct(id string) (*model.TopupProduct, error) {
	var item model.TopupProduct
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateTopupProduct(item *model.TopupProduct) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdateTopupProduct(item *model.TopupProduct) error {
	return r.db.Model(&model.TopupProduct{}).Where("id = ?", item.ID).Updates(map[string]any{
		"name": item.Name, "price_fen": item.PriceFen, "credits": item.Credits, "bonus": item.Bonus,
		"provider": item.Provider, "enabled": item.Enabled, "sort_order": item.SortOrder,
		"updated_at": item.UpdatedAt,
	}).Error
}

func (r *Repository) DeleteTopupProduct(id string) error {
	result := r.db.Delete(&model.TopupProduct{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// PaymentOrders 返回全部订单（管理员用），最新在前。
func (r *Repository) PaymentOrders(limit int) ([]model.PaymentOrder, error) {
	var items []model.PaymentOrder
	err := r.db.Order("created_at desc, id desc").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) PaymentOrdersByUser(userID string, limit int) ([]model.PaymentOrder, error) {
	var items []model.PaymentOrder
	err := r.db.Where("user_id = ?", userID).Order("created_at desc, id desc").Limit(limit).Find(&items).Error
	return items, err
}

func (r *Repository) PaymentOrder(id string) (*model.PaymentOrder, error) {
	var item model.PaymentOrder
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreatePaymentOrder(item *model.PaymentOrder) error {
	return r.db.Create(item).Error
}

func (r *Repository) UpdatePaymentOrder(item *model.PaymentOrder) error {
	return r.db.Model(&model.PaymentOrder{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status": item.Status, "provider_trade_no": item.ProviderTradeNo,
		"paid_at": item.PaidAt, "settled_at": item.SettledAt, "settled_by": item.SettledBy,
		"provider_config_id": item.ProviderConfigID,
		"close_reason":       item.CloseReason, "updated_at": item.UpdatedAt,
		"expires_at":  item.ExpiresAt,
		"refunded_at": item.RefundedAt, "refunded_by": item.RefundedBy,
		"refund_reason": item.RefundReason, "refund_provider_no": item.RefundProviderNo,
		"refund_reversed": item.RefundReversed,
	}).Error
}

// ExpirePendingPaymentOrders closes unpaid orders whose deadline has passed.
// Legacy rows with a NULL expires_at fall back to created_at so an upgraded
// database never leaves old orders payable forever. Closing is guarded by the
// pending status, so an order settled between the read and the write survives.
func (r *Repository) ExpirePendingPaymentOrders(cutoff time.Time, reason string, limit int, now time.Time) (int64, error) {
	if limit <= 0 {
		limit = 200
	}
	var ids []string
	if err := r.db.Model(&model.PaymentOrder{}).
		Where("status = ? AND ((expires_at IS NOT NULL AND expires_at <= ?) OR (expires_at IS NULL AND created_at <= ?))",
			model.PaymentOrderPendingPayment, cutoff, cutoff).
		Order("created_at asc").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.Model(&model.PaymentOrder{}).
		Where("id IN ? AND status = ?", ids, model.PaymentOrderPendingPayment).
		Updates(map[string]any{"status": model.PaymentOrderClosed, "close_reason": reason, "updated_at": now.UTC()})
	return result.RowsAffected, result.Error
}

// PaymentOrderCountsByStatus groups orders by status for the reconciliation
// report. A status with zero rows is simply absent from the map.
func (r *Repository) PaymentOrderCountsByStatus() (map[string]int64, error) {
	var rows []struct {
		Status string
		Total  int64
	}
	if err := r.db.Model(&model.PaymentOrder{}).
		Select("status, COUNT(*) AS total").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Status] = row.Total
	}
	return counts, nil
}

// PaymentOrderSettledTotals sums money and credits for orders that actually
// settled, so reconciliation can compare the gateway total with our ledger.
func (r *Repository) PaymentOrderSettledTotals() (amountFen int64, credits int64, err error) {
	var row struct {
		AmountFen int64
		Credits   int64
	}
	if err = r.db.Model(&model.PaymentOrder{}).
		Select("COALESCE(SUM(amount_fen), 0) AS amount_fen, COALESCE(SUM(credits), 0) AS credits").
		Where("status = ?", model.PaymentOrderPaid).
		Scan(&row).Error; err != nil {
		return 0, 0, err
	}
	return row.AmountFen, row.Credits, nil
}

// PaymentOrderCountsNeedingAttention counts the two states that mean a human
// should look: a paid order without settlement, and an overdue unpaid order.
func (r *Repository) PaymentOrderCountsNeedingAttention(cutoff time.Time) (paidUnsettled int64, pendingOverdue int64, err error) {
	if err = r.db.Model(&model.PaymentOrder{}).
		Where("status = ? AND settled_at IS NULL", model.PaymentOrderPaid).
		Count(&paidUnsettled).Error; err != nil {
		return 0, 0, err
	}
	if err = r.db.Model(&model.PaymentOrder{}).
		Where("status = ? AND ((expires_at IS NOT NULL AND expires_at <= ?) OR (expires_at IS NULL AND created_at <= ?))",
			model.PaymentOrderPendingPayment, cutoff, cutoff).
		Count(&pendingOverdue).Error; err != nil {
		return 0, 0, err
	}
	return paidUnsettled, pendingOverdue, nil
}

// PaymentNotificationCounts reports callback rows that did not cleanly settle,
// so a stuck callback is visible instead of silently dropping the payment.
func (r *Repository) PaymentNotificationCounts() (unprocessed int64, failed int64, err error) {
	if err = r.db.Model(&model.PaymentNotification{}).Where("processed = ?", false).Count(&unprocessed).Error; err != nil {
		return 0, 0, err
	}
	if err = r.db.Model(&model.PaymentNotification{}).Where("error <> ''").Count(&failed).Error; err != nil {
		return 0, 0, err
	}
	return unprocessed, failed, nil
}

// CreatePaymentNotification 写入一条回调记录；已存在同一通知时返回 found=false。
func (r *Repository) CreatePaymentNotification(item *model.PaymentNotification) (bool, error) {
	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(item)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *Repository) UpdatePaymentNotification(item *model.PaymentNotification) error {
	return r.db.Model(&model.PaymentNotification{}).Where("id = ?", item.ID).Updates(map[string]any{
		"processed": item.Processed, "error": item.Error, "amount_fen": item.AmountFen,
	}).Error
}

// PaymentProviderConfigs 返回全部支付渠道配置，启用且排序靠前的排在最前。
func (r *Repository) PaymentProviderConfigs() ([]model.PaymentProviderConfig, error) {
	var items []model.PaymentProviderConfig
	err := r.db.Order("sort_order asc, created_at asc, id asc").Find(&items).Error
	return items, err
}

// PaymentProviderConfig 按 ID 读取一条配置；不存在时返回 gorm.ErrRecordNotFound。
func (r *Repository) PaymentProviderConfig(id string) (*model.PaymentProviderConfig, error) {
	var item model.PaymentProviderConfig
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreatePaymentProviderConfig(item *model.PaymentProviderConfig) error {
	return r.db.Create(item).Error
}

// UpdatePaymentProviderConfig 只写可变列，避免覆盖创建时间与创建人。
func (r *Repository) UpdatePaymentProviderConfig(item *model.PaymentProviderConfig) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model.PaymentProviderConfig{}, "id = ?", item.ID).Error; err != nil {
			return err
		}
		return tx.Model(&model.PaymentProviderConfig{}).Where("id = ?", item.ID).Updates(map[string]any{
			"provider": item.Provider, "name": item.Name, "app_id": item.AppID, "mch_id": item.MchID,
			"mch_serial_no": item.MchSerialNo,
			"notify_url":    item.NotifyURL, "secrets_encrypted": item.SecretsEncrypted,
			"enabled": item.Enabled, "sort_order": item.SortOrder,
			"last_tested_at": item.LastTestedAt, "last_test_error": item.LastTestError,
			"updated_at": item.UpdatedAt,
		}).Error
	})
}

func (r *Repository) DeletePaymentProviderConfig(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model.PaymentProviderConfig{}, "id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PaymentProviderConfig{}, "id = ?", id).Error
	})
}
