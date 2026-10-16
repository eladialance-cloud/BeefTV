package repository

import (
	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

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
