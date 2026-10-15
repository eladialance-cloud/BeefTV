package repository

import (
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

// StorageLocations 返回全部平台级对象存储配置，默认存储排在最前。
func (r *Repository) StorageLocations() ([]model.StorageLocation, error) {
	var items []model.StorageLocation
	err := r.db.Order("is_default desc, created_at asc, id asc").Find(&items).Error
	return items, err
}

// StorageLocation 按 ID 读取一条配置；不存在时返回 gorm.ErrRecordNotFound。
func (r *Repository) StorageLocation(id string) (*model.StorageLocation, error) {
	var item model.StorageLocation
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateStorageLocation(item *model.StorageLocation) error {
	return r.db.Create(item).Error
}

// UpdateStorageLocation 只写可变列，避免 Save 把创建时间或 ID 一并覆盖。
func (r *Repository) UpdateStorageLocation(item *model.StorageLocation) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model.StorageLocation{}, "id = ?", item.ID).Error; err != nil {
			return err
		}
		return tx.Model(&model.StorageLocation{}).Where("id = ?", item.ID).Updates(map[string]any{
			"name": item.Name, "provider": item.Provider, "endpoint": item.Endpoint, "region": item.Region,
			"bucket": item.Bucket, "access_key_id": item.AccessKeyID, "secret_access_key": item.SecretAccessKey,
			"cdn_base_url": item.CDNBaseURL, "prefix": item.Prefix, "path_style": item.PathStyle,
			"enabled": item.Enabled, "updated_by": item.UpdatedBy, "updated_at": item.UpdatedAt,
			"last_tested_at": item.LastTestedAt, "last_test_error": item.LastTestError,
		}).Error
	})
}

func (r *Repository) DeleteStorageLocation(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model.StorageLocation{}, "id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&model.StorageLocation{}, "id = ?", id).Error
	})
}

// SetDefaultStorageLocation 在同一事务里清掉其它默认项，保证最多一条默认存储。
func (r *Repository) SetDefaultStorageLocation(id string, updatedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&model.StorageLocation{}, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.StorageLocation{}).Where("is_default = ? AND id <> ?", true, id).
			Updates(map[string]any{"is_default": false, "updated_at": updatedAt}).Error; err != nil {
			return err
		}
		return tx.Model(&model.StorageLocation{}).Where("id = ?", id).
			Updates(map[string]any{"is_default": true, "updated_at": updatedAt}).Error
	})
}
