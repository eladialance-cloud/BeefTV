package repository

import (
	"strings"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

func (r *Repository) AdminCreationCatalogItems(kind string, keyword string, status string, limit int, offset int) ([]model.CreationCatalogItem, int64, error) {
	query := r.db.Model(&model.CreationCatalogItem{}).Where("kind = ?", strings.TrimSpace(kind))
	if value := strings.TrimSpace(keyword); value != "" {
		like := "%" + value + "%"
		query = query.Where("name LIKE ? OR item_key LIKE ? OR description LIKE ?", like, like, like)
	}
	switch strings.TrimSpace(status) {
	case "enabled":
		query = query.Where("enabled = ?", true)
	case "disabled":
		query = query.Where("enabled = ?", false)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.CreationCatalogItem
	if err := query.Order("sort_order ASC, updated_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) CreationCatalogItem(id string) (*model.CreationCatalogItem, error) {
	var item model.CreationCatalogItem
	if err := r.db.First(&item, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateCreationCatalogItem(item *model.CreationCatalogItem) error {
	return r.db.Create(item).Error
}

func (r *Repository) SaveCreationCatalogItem(item *model.CreationCatalogItem) error {
	return r.db.Save(item).Error
}

func (r *Repository) DeleteCreationCatalogItem(id string) error {
	result := r.db.Delete(&model.CreationCatalogItem{}, "id = ?", strings.TrimSpace(id))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) PublicCreationCatalogItems(kind string, scene string) ([]model.CreationCatalogItem, error) {
	query := r.db.Where("kind = ? AND enabled = ?", strings.TrimSpace(kind), true)
	if value := strings.TrimSpace(scene); value != "" && value != "all" {
		query = query.Where("scene = ? OR scene = ''", value)
	}
	var items []model.CreationCatalogItem
	if err := query.Order("sort_order ASC, updated_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
