package repository

import (
	"errors"
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
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
		"close_reason": item.CloseReason, "updated_at": item.UpdatedAt,
	}).Error
}

// CreatePaymentNotification 写入一条回调记录；已存在同一通知时返回 found=false。
func (r *Repository) CreatePaymentNotification(item *model.PaymentNotification) (bool, error) {
	var existing model.PaymentNotification
	err := r.db.Where("provider = ? AND provider_notification_id = ?", item.Provider, item.ProviderNotificationID).
		First(&existing).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	return true, r.db.Create(item).Error
}

func (r *Repository) UpdatePaymentNotification(item *model.PaymentNotification) error {
	return r.db.Model(&model.PaymentNotification{}).Where("id = ?", item.ID).Updates(map[string]any{
		"processed": item.Processed, "error": item.Error,
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
			"notify_url": item.NotifyURL, "secrets_encrypted": item.SecretsEncrypted,
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
